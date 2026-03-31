package socket

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valksor/kvelmo/pkg/agent/strategy"
	"github.com/valksor/kvelmo/pkg/codegraph"
	"github.com/valksor/kvelmo/pkg/conductor"
	"github.com/valksor/kvelmo/pkg/eventlog"
	"github.com/valksor/kvelmo/pkg/git"
	"github.com/valksor/kvelmo/pkg/memory"
	"github.com/valksor/kvelmo/pkg/provider"
	"github.com/valksor/kvelmo/pkg/screenshot"
	"github.com/valksor/kvelmo/pkg/settings"
	"github.com/valksor/kvelmo/pkg/storage"
	"github.com/valksor/kvelmo/pkg/worker"
)

// replayBufSize is the number of streaming events kept in the ring buffer for
// reconnecting clients to replay missed events.
const replayBufSize = 200

// WorktreeSocket provides JSON-RPC interface for a single worktree.
// Per flow_v2.md: "Worktree Socket manages per-project state machine, git ops".
type WorktreeSocket struct {
	server     *Server
	path       string
	globalPath string

	// Core components
	conductor   *conductor.Conductor
	repo        *git.Repository
	pool        *worker.Pool
	screenshots *screenshot.Store

	// Codegraph: lazy-initialized cached connection.
	codegraphOnce    sync.Once
	codegraphInst    *codegraph.Graph
	codegraphInitErr error

	// Streaming: active subscriber channels
	streams   map[string]chan []byte
	streamsMu sync.RWMutex

	// Replay buffer: ring buffer of the last replayBufSize events (with seq injected).
	// Clients can resume from a known seq on reconnect.
	eventSeq   atomic.Uint64
	replayBuf  [replayBufSize][]byte
	replayHead int
	replayMu   sync.Mutex
}

// WorktreeConfig configures a worktree socket.
type WorktreeConfig struct {
	WorktreePath string
	SocketPath   string
	GlobalPath   string
	Pool         *worker.Pool
}

// NewWorktreeSocket creates a new worktree socket with conductor integration.
// Git is optional - if the directory is not a git repository, git-dependent
// features (checkpoints, branches, PR workflow) will be unavailable but the
// socket will still work for basic operations.
func NewWorktreeSocket(cfg WorktreeConfig) (*WorktreeSocket, error) {
	repo, err := git.Open(cfg.WorktreePath)
	if err != nil {
		slog.Debug("git not available, some features disabled", "path", cfg.WorktreePath, "error", err)
		// Continue with repo = nil - conductor handles this gracefully
	}

	// Load settings first so provider tokens are available.
	// Settings come from local .env files (never global env vars).
	effective, _, _, err := settings.LoadEffective(cfg.WorktreePath)
	if err != nil {
		// Non-fatal: use defaults if settings fail to load.
		effective = settings.DefaultSettings()
	}

	providers := provider.NewRegistry(effective)

	cond := conductor.NewConductor(conductor.ConductorConfig{
		Repo:         repo,
		Pool:         cfg.Pool,
		Providers:    providers,
		WorktreePath: cfg.WorktreePath,
	})

	// Wire storage.Store so specs/reviews/sessions are persisted via pkg/storage.
	store := storage.NewStore(cfg.WorktreePath, settings.BoolValue(effective.Storage.SaveInProject, false))
	cond.SetStore(store)

	// Wire event log for lifecycle auditing.
	eventLogDir := filepath.Join(cfg.WorktreePath, ".kvelmo")
	evLog, evErr := eventlog.New(eventLogDir)
	if evErr != nil {
		slog.Debug("event log not available", "path", eventLogDir, "error", evErr)
	} else {
		cond.SetEventLog(evLog)
	}

	// Restore prior task state if a task.yaml exists from a previous session.
	_ = cond.LoadState(context.Background())

	// Wire memory indexer so completed tasks are indexed for memory.search.
	// We reuse the package-level adapter from memory.go (or create a per-worktree
	// indexer rooted at the worktree directory so that .kvelmo/specifications
	// and .kvelmo/sessions are found correctly).
	if adapter, adapterErr := getMemoryAdapter(context.Background()); adapterErr == nil {
		idxr := memory.NewIndexer(adapter.Store(), cfg.WorktreePath)
		cond.SetMemoryIndexer(idxr)
	}

	// Wire agent strategy from settings.
	if effective.Agent.Strategy != "" {
		if s, ok := strategy.Get(effective.Agent.Strategy); ok {
			cond.SetStrategy(s)
		} else {
			slog.Warn("unknown agent strategy in config, using default",
				"strategy", effective.Agent.Strategy,
				"available", strategy.List())
		}
	}
	for phase, name := range effective.Agent.PhaseStrategy {
		if s, ok := strategy.Get(name); ok {
			cond.SetPhaseStrategy(phase, s)
		} else {
			slog.Warn("unknown phase strategy in config, ignoring",
				"phase", phase, "strategy", name,
				"available", strategy.List())
		}
	}

	// Wire task group checker for cross-repo synchronized submit.
	if tgc := GetTaskGroupCoordinator(); tgc != nil {
		cond.SetTaskGroupChecker(tgc)
	}

	// Initialize screenshot store in .mehrhof directory
	mehrhofPath := filepath.Join(cfg.WorktreePath, ".mehrhof")

	// Canonicalize worktree path so path traversal guards in handlers
	// (e.g., handleWorktreeFilesList) compare against a normalized prefix.
	canonicalPath, err := filepath.Abs(cfg.WorktreePath)
	if err != nil {
		canonicalPath = filepath.Clean(cfg.WorktreePath)
	}

	w := &WorktreeSocket{
		server:      NewServer(cfg.SocketPath),
		path:        canonicalPath,
		globalPath:  cfg.GlobalPath,
		conductor:   cond,
		repo:        repo,
		pool:        cfg.Pool,
		screenshots: screenshot.NewStore(mehrhofPath),
		streams:     make(map[string]chan []byte),
	}

	w.registerHandlers()
	w.setupEventForwarding()

	return w, nil
}

// NewWorktreeSocketSimple creates a worktree socket with git support but without conductor.
// Useful for basic operations that don't require the full task lifecycle.
func NewWorktreeSocketSimple(socketPath, worktreePath string) *WorktreeSocket {
	// Canonicalize path for consistent path traversal guards (matches NewWorktreeSocket)
	canonicalPath, err := filepath.Abs(worktreePath)
	if err != nil {
		canonicalPath = filepath.Clean(worktreePath)
	}
	w := &WorktreeSocket{
		server:     NewServer(socketPath),
		path:       canonicalPath,
		globalPath: GlobalSocketPath(),
		streams:    make(map[string]chan []byte),
	}

	// Try to open git repository
	repo, err := git.Open(worktreePath)
	if err == nil {
		w.repo = repo
	}

	w.registerBasicHandlers()

	return w
}

func (w *WorktreeSocket) registerBasicHandlers() {
	w.server.Handle("status", w.handleStatus)
	w.server.Handle("recap", w.handleRecap)
	w.server.Handle("ping", w.handlePing)
	w.server.Handle("strategy.list", w.handleStrategyList)
	w.server.Handle("checkpoints", w.handleCheckpoints)
	w.server.Handle("checkpoint.goto", w.handleCheckpointGoto)
	w.server.Handle("checkpoint.preview", w.handleCheckpointPreview)

	// Git handlers (work if repo is set)
	w.server.Handle("git.status", w.handleGitStatus)
	w.server.Handle("git.diff", w.handleGitDiff)
	w.server.Handle("git.diff_against", w.handleGitDiffAgainst)
	w.server.Handle("git.log", w.handleGitLog)

	// File browsing and listing
	w.server.Handle("browse", w.handleBrowse)
	w.server.Handle("files.list", w.handleWorktreeFilesList)

	// Streaming (required for frontend connection)
	w.server.HandleWithConn("stream.subscribe", w.handleStreamSubscribe)

	// Review history (gracefully handles missing conductor)
	w.server.Handle("review.list", w.handleReviewList)
}

func (w *WorktreeSocket) registerHandlers() {
	// Register basic handlers first (ping, status, git.*, stream.subscribe, etc.)
	w.registerBasicHandlers()

	// Task lifecycle
	w.server.Handle("start", w.handleStart)
	w.server.Handle("plan", w.handlePlan)
	w.server.Handle("implement", w.handleImplement)
	w.server.Handle("optimize", w.handleOptimize)
	w.server.Handle("simplify", w.handleSimplify)
	w.server.Handle("review", w.handleReview)
	w.server.Handle("submit", w.handleSubmit)
	w.server.Handle("task.finish", w.handleFinish)
	w.server.Handle("task.refresh", w.handleRefresh)
	w.server.Handle("remote.approve", w.handleRemoteApprove)
	w.server.Handle("remote.merge", w.handleRemoteMerge)
	w.server.Handle("abort", w.handleAbort)
	w.server.Handle("stop", w.handleStop)
	w.server.Handle("reset", w.handleReset)
	w.server.Handle("shutdown", w.handleShutdown)

	// Task management
	w.server.Handle("abandon", w.handleAbandon)
	w.server.Handle("delete", w.handleDelete)
	w.server.Handle("update", w.handleUpdate)

	// Task queue
	w.server.Handle("queue.add", w.handleQueueAdd)
	w.server.Handle("queue.remove", w.handleQueueRemove)
	w.server.Handle("queue.list", w.handleQueueList)
	w.server.Handle("queue.reorder", w.handleQueueReorder)

	// Task history
	w.server.Handle("task.history", w.handleTaskHistory)
	w.server.Handle("task.search", w.handleTaskSearch)

	// Suggestions
	w.server.Handle("suggestions.list", w.handleSuggestionsList)

	// Changelog
	w.server.Handle("changelog.generate", w.handleChangelogGenerate)

	// Review
	w.server.Handle("review.view", w.handleReviewView)

	// Context resolution (@-mentions)
	w.server.Handle("context.resolve", w.handleContextResolve)

	// Quality gate user prompts
	w.server.Handle("quality.respond", w.handleQualityRespond)

	// Checkpoint navigation
	w.server.Handle("undo", w.handleUndo)
	w.server.Handle("redo", w.handleRedo)

	// Show spec/plan content
	w.server.Handle("show.spec", w.handleShowSpec)
	w.server.Handle("show.plan", w.handleShowSpec) // plan output is stored as specifications

	// Screenshots
	w.server.Handle("screenshots.list", w.handleScreenshotsList)
	w.server.Handle("screenshots.get", w.handleScreenshotsGet)
	w.server.Handle("screenshots.capture", w.handleScreenshotsCapture)
	w.server.Handle("screenshots.delete", w.handleScreenshotsDelete)

	// Task export
	w.server.Handle("task.export", w.handleTaskExport)

	// Task tagging
	w.server.Handle("task.tag", w.handleTaskTag)

	// Policy checking
	w.server.Handle("policy.check", w.handlePolicyCheck)

	// Approval & review gates
	w.server.Handle("approve", w.handleApprove)
	w.server.Handle("approve.node", w.handleApproveNode)
	w.server.Handle("review.checklist.get", w.handleReviewChecklistGet)
	w.server.Handle("review.checklist.check", w.handleReviewChecklistCheck)
	w.server.Handle("review.checklist.uncheck", w.handleReviewChecklistUncheck)

	// CI status
	w.server.Handle("ci.status", w.handleCIStatus)

	// Auto-fix status
	w.server.Handle("autofix.status", w.handleAutoFixStatus)

	// Workflow hooks
	w.server.Handle("hooks.list", w.handleHooksList)

	// Code graph
	w.server.Handle("codegraph.stats", w.handleCodegraphStats)
	w.server.Handle("codegraph.index", w.handleCodegraphIndex)
	w.server.Handle("codegraph.search", w.handleCodegraphSearch)
	w.server.Handle("codegraph.callers", w.handleCodegraphCallers)
	w.server.Handle("codegraph.deps", w.handleCodegraphDeps)

	// Adversarial review
	w.server.Handle("adversarial.run", w.handleAdversarialRun)
	w.server.Handle("adversarial.results", w.handleAdversarialResults)

	// Discovery
	w.server.Handle("discovery.scan", w.handleDiscoveryScan)

	// Risk evaluation
	w.server.Handle("risk.evaluate", w.handleRiskEvaluate)
	w.server.Handle("risk.history", w.handleRiskHistory)

	// Event log
	w.server.Handle("eventlog.query", w.handleEventlogQuery)

	// Failure classification
	w.server.Handle("failclass.stats", w.handleFailclassStats)

	// Progress estimation
	w.server.Handle("progress.get", w.handleProgressGet)

	// Conversation forking
	w.server.Handle("fork.create", w.handleForkCreate)
	w.server.Handle("fork.list", w.handleForkList)
	w.server.Handle("fork.compare", w.handleForkCompare)
	w.server.Handle("fork.select", w.handleForkSelect)

	// Provisioning
	w.server.Handle("provision.preview", w.handleProvisionPreview)

	// Response cache
	w.server.Handle("cache.stats", w.handleCacheStats)
	w.server.Handle("cache.clear", w.handleCacheClear)
}

// injectSeqAndBuffer assigns a sequence number to a JSON event, stores it in the
// ring buffer, and returns the enriched bytes (JSON with "seq" field + newline).
// The seq field is injected directly into the JSON bytes to avoid a full round-trip.
func (w *WorktreeSocket) injectSeqAndBuffer(data []byte) []byte {
	seq := w.eventSeq.Add(1)

	// Validate data is a non-empty JSON object
	if len(data) < 2 || data[0] != '{' {
		// Return safe fallback for invalid input
		enriched := fmt.Appendf(nil, `{"seq":%d,"error":"invalid_input"}`+"\n", seq)
		w.replayMu.Lock()
		// Store a defensive copy to prevent shared backing memory issues
		bufCopy := make([]byte, len(enriched))
		copy(bufCopy, enriched)
		w.replayBuf[w.replayHead] = bufCopy
		w.replayHead = (w.replayHead + 1) % replayBufSize
		w.replayMu.Unlock()

		return enriched
	}

	// Handle empty object {} specially to avoid invalid JSON {"seq":N,}
	var enriched []byte
	if len(data) == 2 && data[1] == '}' {
		enriched = fmt.Appendf(nil, `{"seq":%d}`+"\n", seq)
	} else {
		// data is a JSON object starting with `{`. Inject "seq":N right after the brace.
		prefix := fmt.Appendf(nil, `{"seq":%d,`, seq)
		enriched = append(prefix, data[1:]...)
		enriched = append(enriched, '\n')
	}

	w.replayMu.Lock()
	// Store a defensive copy to prevent shared backing memory issues
	bufCopy := make([]byte, len(enriched))
	copy(bufCopy, enriched)
	w.replayBuf[w.replayHead] = bufCopy
	w.replayHead = (w.replayHead + 1) % replayBufSize
	w.replayMu.Unlock()

	return enriched
}

func (w *WorktreeSocket) setupEventForwarding() {
	if w.conductor == nil {
		return
	}

	// Forward conductor events to subscribers
	w.conductor.OnEvent(func(event conductor.ConductorEvent) {
		data, err := json.Marshal(event)
		if err != nil {
			return
		}

		enriched := w.injectSeqAndBuffer(data)

		w.streamsMu.RLock()
		for _, ch := range w.streams {
			select {
			case ch <- enriched:
			default:
				slog.Warn("worktree event channel full, dropping event", "type", event.Type)
			}
		}
		w.streamsMu.RUnlock()
	})
}

// noConductor returns an error response when a handler requires a conductor
// but none is configured. Returns nil if conductor is available.
func (w *WorktreeSocket) noConductor(id string) *Response {
	if w.conductor != nil {
		return nil
	}

	return NewErrorResponse(id, -32600, "no conductor configured")
}

// --- Basic Handlers ---

func (w *WorktreeSocket) handlePing(ctx context.Context, req *Request) (*Response, error) {
	return NewResultResponse(req.ID, map[string]string{"status": "ok"})
}

func (w *WorktreeSocket) handleStrategyList(_ context.Context, req *Request) (*Response, error) {
	return NewResultResponse(req.ID, strategy.List())
}

func (w *WorktreeSocket) handleStatus(ctx context.Context, req *Request) (*Response, error) {
	result := StatusResult{
		Path:  w.path,
		State: StateNone,
	}

	if w.conductor != nil {
		state := w.conductor.State()
		result.State = TaskState(state)

		if wu := w.conductor.WorkUnit(); wu != nil {
			var sourceRef string
			if wu.Source != nil {
				sourceRef = wu.Source.Reference
			}
			result.Task = &TaskInfo{
				ID:           wu.ID,
				Title:        wu.Title,
				Source:       sourceRef,
				Branch:       wu.Branch,
				WorktreePath: wu.WorktreePath,
				ContextItems: wu.ContextItems,
			}
		}

		if ids := w.conductor.PendingPromptIDs(); len(ids) > 0 {
			result.PendingPromptID = ids[0]
		}

		if wu := w.conductor.WorkUnit(); wu != nil {
			switch state {
			case conductor.StatePlanning, conductor.StateImplementing, conductor.StateSimplifying,
				conductor.StateOptimizing, conductor.StateReviewing:
				if len(wu.Jobs) > 0 {
					result.ActiveJobID = wu.Jobs[len(wu.Jobs)-1]
				}
			case conductor.StateNone, conductor.StateLoaded, conductor.StatePlanned,
				conductor.StateImplemented, conductor.StateSubmitted, conductor.StateFailed,
				conductor.StateWaiting, conductor.StatePaused:
				// Not in a working state — no active job
			}
		}

		result.QueueDepth = w.conductor.QueueLength()

		if fc := w.conductor.LastFailureClass(); fc != "" {
			result.LastFailureClass = string(fc)
		}

		if wu := w.conductor.WorkUnit(); wu != nil && len(wu.PhaseMetrics) > 0 {
			result.PhaseMetrics = wu.PhaseMetrics
		}

		if rs := w.conductor.RecoveryState(); rs != "" {
			result.NeedsRecovery = rs
		}

		if sp := w.conductor.SkipPhases(); len(sp) > 0 {
			result.SkipPhases = sp
		}
	}

	return NewResultResponse(req.ID, result)
}

func (w *WorktreeSocket) handleRecap(ctx context.Context, req *Request) (*Response, error) {
	result := RecapResult{
		Path:       w.path,
		State:      StateNone,
		NextAction: "Run 'kvelmo start' to load a task",
	}

	if w.conductor == nil {
		return NewResultResponse(req.ID, result)
	}

	state := w.conductor.State()
	result.State = TaskState(state)

	wu := w.conductor.WorkUnit()
	if wu != nil {
		var sourceRef string
		if wu.Source != nil {
			sourceRef = wu.Source.Reference
		}
		result.Task = &TaskInfo{
			ID:           wu.ID,
			Title:        wu.Title,
			Source:       sourceRef,
			Branch:       wu.Branch,
			WorktreePath: wu.WorktreePath,
			ContextItems: wu.ContextItems,
		}

		result.CheckpointCount = len(wu.Checkpoints)
		result.Tags = wu.Tags

		if len(wu.PhaseMetrics) > 0 {
			result.PhaseMetrics = wu.PhaseMetrics
		}

		// Last checkpoint with enrichment
		if len(wu.Checkpoints) > 0 {
			lastSHA := wu.Checkpoints[len(wu.Checkpoints)-1]
			info := CheckpointInfo{SHA: lastSHA}
			if w.repo != nil {
				if entry, err := w.repo.CommitInfo(ctx, lastSHA); err == nil {
					info.Message = entry.Message
					info.Author = entry.Author
					info.Timestamp = entry.Date
					// Compute human-readable time since last activity
					if t, parseErr := time.Parse(time.RFC3339, entry.Date); parseErr == nil {
						result.LastActivity = formatTimeSince(t)
					}
				}
			}
			result.LastCheckpoint = &info
		}
	}

	// Files changed in working tree
	if w.repo != nil {
		if files, err := w.repo.DiffFilesWithStatus(ctx); err == nil {
			result.FilesChanged = files
		}
	}

	if fc := w.conductor.LastFailureClass(); fc != "" {
		switch fc {
		case conductor.FailureClassHardStop:
			result.LastError = "Hard failure — requires manual intervention (run 'kvelmo logs' for details)"
		case conductor.FailureClassRecoverable:
			result.LastError = "Transient error — will auto-retry"
		case conductor.FailureClassDegraded:
			result.LastError = "Non-critical failure — workflow continued with warning"
		case conductor.FailureClassSkippable:
			result.LastError = "Phase skipped — nothing to do"
		default:
			result.LastError = string(fc)
		}
	}

	result.NextAction = suggestNextAction(state, wu)

	return NewResultResponse(req.ID, result)
}

// suggestNextAction returns a human-readable suggestion for the next workflow step.
func suggestNextAction(state conductor.State, wu *conductor.WorkUnit) string {
	switch state {
	case conductor.StateNone:
		return "Run 'kvelmo start' to load a task"
	case conductor.StateLoaded:
		return "Run 'kvelmo plan' to generate a specification"
	case conductor.StatePlanning:
		return "Planning in progress — wait or run 'kvelmo watch'"
	case conductor.StatePlanned:
		return "Run 'kvelmo implement' to start coding"
	case conductor.StateImplementing:
		return "Implementation in progress — wait or run 'kvelmo watch'"
	case conductor.StateImplemented:
		return "Run 'kvelmo review' to check the work, or 'kvelmo simplify' for cleanup"
	case conductor.StateSimplifying:
		return "Simplification in progress — wait or run 'kvelmo watch'"
	case conductor.StateOptimizing:
		return "Optimization in progress — wait or run 'kvelmo watch'"
	case conductor.StateReviewing:
		return "Review in progress — wait or check 'kvelmo checklist list'"
	case conductor.StateSubmitted:
		if wu != nil && wu.PRID != "" {
			return "PR submitted (" + wu.PRID + ") — merge it, then run 'kvelmo finish'"
		}

		return "PR submitted — merge it, then run 'kvelmo finish'"
	case conductor.StateFailed:
		return "Run 'kvelmo retry' to re-run the failed phase, or 'kvelmo reset' to start over"
	case conductor.StateWaiting:
		return "Agent is waiting for input — check 'kvelmo quality respond' or answer via chat"
	case conductor.StatePaused:
		return "Task is paused — resume when ready"
	default:
		return ""
	}
}

// formatTimeSince returns a human-readable duration string.
func formatTimeSince(t time.Time) string {
	d := time.Since(t)
	if d < 0 {
		return "just now"
	}
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 minute ago"
		}

		return fmt.Sprintf("%d minutes ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}

		return fmt.Sprintf("%d hours ago", h)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}

		return fmt.Sprintf("%d days ago", days)
	}
}

// --- Show Spec/Plan Handlers ---

// SpecEntry holds a single specification file's path and content.
type SpecEntry struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// ShowSpecResult is returned by the show.spec and show.plan RPC methods.
type ShowSpecResult struct {
	Specifications []SpecEntry `json:"specifications"`
}

func (w *WorktreeSocket) handleShowSpec(_ context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	wu := w.conductor.WorkUnit()
	if wu == nil {
		return NewResultResponse(req.ID, ShowSpecResult{Specifications: []SpecEntry{}})
	}

	specs := make([]SpecEntry, 0, len(wu.Specifications))
	for _, path := range wu.Specifications {
		content, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("show.spec: failed to read specification", "path", path, "error", err)

			continue
		}
		specs = append(specs, SpecEntry{Path: path, Content: string(content)})
	}

	return NewResultResponse(req.ID, ShowSpecResult{Specifications: specs})
}

// --- Lifecycle ---

func (w *WorktreeSocket) Start(ctx context.Context) error {
	go w.registerWithGlobal(ctx)

	return w.server.Start(ctx)
}

func (w *WorktreeSocket) Stop() error {
	w.streamsMu.Lock()
	for _, ch := range w.streams {
		close(ch)
	}
	w.streams = make(map[string]chan []byte)
	w.streamsMu.Unlock()

	if w.codegraphInst != nil {
		_ = w.codegraphInst.Close()
	}

	return w.server.Stop()
}

func (w *WorktreeSocket) registerWithGlobal(ctx context.Context) {
	client, err := NewClient(w.globalPath, WithTimeout(2*time.Second))
	if err != nil {
		return
	}
	defer func() { _ = client.Close() }()

	params := RegisterParams{
		Path:       w.path,
		SocketPath: w.server.Path(),
	}

	_, _ = client.Call(ctx, "projects.register", params)
}

func (w *WorktreeSocket) Path() string {
	return w.path
}

func (w *WorktreeSocket) Server() *Server {
	return w.server
}

func (w *WorktreeSocket) Conductor() *conductor.Conductor {
	return w.conductor
}

// --- Types ---

type TaskState string

const (
	StateNone         TaskState = "none"
	StateLoaded       TaskState = "loaded"
	StatePlanning     TaskState = "planning"
	StatePlanned      TaskState = "planned"
	StateImplementing TaskState = "implementing"
	StateImplemented  TaskState = "implemented"
	StateOptimizing   TaskState = "optimizing"
	StateReviewing    TaskState = "reviewing"
	StateSubmitted    TaskState = "submitted"
	StateFailed       TaskState = "failed"
	StateWaiting      TaskState = "waiting"
	StatePaused       TaskState = "paused"
)

type StatusResult struct {
	State            TaskState                          `json:"state"`
	Path             string                             `json:"path"`
	Task             *TaskInfo                          `json:"task,omitempty"`
	PendingPromptID  string                             `json:"pending_prompt_id,omitempty"`
	ActiveJobID      string                             `json:"active_job_id,omitempty"`
	QueueDepth       int                                `json:"queue_depth,omitempty"`
	LastError        string                             `json:"last_error,omitempty"`
	LastFailureClass string                             `json:"last_failure_class,omitempty"`
	PhaseMetrics     map[string]*conductor.PhaseMetrics `json:"phase_metrics,omitempty"`
	NeedsRecovery    string                             `json:"needs_recovery,omitempty"` // Interrupted phase name if recovery needed
	SkipPhases       []string                           `json:"skip_phases,omitempty"`    // Phases that will be skipped
}

type TaskInfo struct {
	ID           string                  `json:"id"`
	Title        string                  `json:"title"`
	Source       string                  `json:"source"`
	Branch       string                  `json:"branch,omitempty"`
	WorktreePath string                  `json:"worktree_path,omitempty"`
	ContextItems []conductor.ContextItem `json:"context_items,omitempty"`
}

// RecapResult is returned by the recap RPC method.
// It provides a concise summary of the current task state for resuming work.
type RecapResult struct {
	State           TaskState                          `json:"state"`
	Path            string                             `json:"path"`
	Task            *TaskInfo                          `json:"task,omitempty"`
	LastCheckpoint  *CheckpointInfo                    `json:"last_checkpoint,omitempty"`
	CheckpointCount int                                `json:"checkpoint_count"`
	FilesChanged    []git.FileStatus                   `json:"files_changed,omitempty"`
	PhaseMetrics    map[string]*conductor.PhaseMetrics `json:"phase_metrics,omitempty"`
	Tags            []string                           `json:"tags,omitempty"`
	LastActivity    string                             `json:"last_activity,omitempty"` // Human-readable time since last checkpoint
	NextAction      string                             `json:"next_action"`             // Suggested next step
	LastError       string                             `json:"last_error,omitempty"`
}
