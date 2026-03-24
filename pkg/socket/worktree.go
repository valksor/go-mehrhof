package socket

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
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
	"github.com/valksor/kvelmo/pkg/provision"
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
}

// injectSeqAndBuffer assigns a sequence number to a JSON event, stores it in the
// ring buffer, and returns the enriched bytes (JSON with "seq" field + newline).
// The seq field is injected directly into the JSON bytes to avoid a full round-trip.
func (w *WorktreeSocket) injectSeqAndBuffer(data []byte) []byte {
	seq := w.eventSeq.Add(1)

	// Validate data is a non-empty JSON object
	if len(data) < 2 || data[0] != '{' {
		// Return safe fallback for invalid input
		enriched := []byte(fmt.Sprintf(`{"seq":%d,"error":"invalid_input"}`+"\n", seq))
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
		enriched = []byte(fmt.Sprintf(`{"seq":%d}`+"\n", seq))
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

// --- Task Lifecycle Handlers ---

type StartParams struct {
	Source               string                  `json:"source"` // e.g., "github:owner/repo#123"
	UseWorktreeIsolation bool                    `json:"use_worktree_isolation,omitempty"`
	AutoAdvance          bool                    `json:"auto_advance,omitempty"`  // Auto-progress through plan → implement → review
	SkipPhases           []string                `json:"skip_phases,omitempty"`   // Per-invocation phases to skip (e.g., ["simplify", "optimize"])
	ContextItems         []conductor.ContextItem `json:"context_items,omitempty"` // Attached context references (@-mentions)
}

func (w *WorktreeSocket) handleStart(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params StartParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, -32602, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
	}

	if params.Source == "" {
		return NewErrorResponse(req.ID, -32602, "source is required"), nil
	}

	if err := w.conductor.Start(ctx, params.Source); err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	// Enable auto-advance if requested
	if params.AutoAdvance {
		w.conductor.SetAutoAdvance(true)
	}

	// Set per-invocation skip phases
	if len(params.SkipPhases) > 0 {
		w.conductor.SetSkipPhases(params.SkipPhases)
	}

	// Attach context items (@-mentions) to the work unit
	if len(params.ContextItems) > 0 {
		w.conductor.SetContextItems(params.ContextItems)
	}

	// Worktree isolation is handled inside conductor.Start(). Log if active.
	if wu := w.conductor.WorkUnit(); wu != nil && wu.WorktreePath != "" {
		slog.Info("handleStart: worktree isolation active", "path", wu.WorktreePath, "branch", wu.Branch)
	}

	// Auto-advance: trigger next phase immediately after start.
	// If "plan" is in skip_phases, jump straight to implement.
	if params.AutoAdvance {
		skipPlan := false
		for _, p := range params.SkipPhases {
			if p == "plan" {
				skipPlan = true

				break
			}
		}
		go func() { //nolint:contextcheck // intentionally uses background context for async auto-advance
			bgCtx := context.Background()
			if skipPlan {
				if _, err := w.conductor.Implement(bgCtx); err != nil {
					slog.Warn("auto-advance: implement after start failed", "error", err)
				}
			} else {
				if _, err := w.conductor.Plan(bgCtx); err != nil {
					slog.Warn("auto-advance: plan after start failed", "error", err)
				}
			}
		}()
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "started",
		"state":  w.conductor.State(),
	})
}

type PlanParams struct {
	Prompt string `json:"prompt,omitempty"`  // Additional context for planning
	DryRun bool   `json:"dry_run,omitempty"` // Simulate without executing agent
}

func (w *WorktreeSocket) handlePlan(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params PlanParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil
		}
	}

	// Apply dry-run mode for this call
	prevDryRun := w.conductor.DryRunEnabled()
	if params.DryRun {
		w.conductor.SetDryRun(true)
		defer w.conductor.SetDryRun(prevDryRun)
	}

	// Submit planning job
	jobID, err := w.conductor.Plan(ctx)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "planning",
		"job_id": jobID,
		"state":  w.conductor.State(),
	})
}

type ImplementParams struct {
	Prompt string `json:"prompt,omitempty"`  // Additional context for implementation
	DryRun bool   `json:"dry_run,omitempty"` // Simulate without executing agent
}

func (w *WorktreeSocket) handleImplement(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params ImplementParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil
		}
	}

	// Apply dry-run mode for this call
	prevDryRun := w.conductor.DryRunEnabled()
	if params.DryRun {
		w.conductor.SetDryRun(true)
		defer w.conductor.SetDryRun(prevDryRun)
	}

	// Submit implementation job
	jobID, err := w.conductor.Implement(ctx)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "implementing",
		"job_id": jobID,
		"state":  w.conductor.State(),
	})
}

type OptimizeParams struct {
	Prompt string `json:"prompt,omitempty"`  // Additional context for optimization
	DryRun bool   `json:"dry_run,omitempty"` // Simulate without executing agent
}

func (w *WorktreeSocket) handleOptimize(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params OptimizeParams
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}

	// Apply dry-run mode for this call
	prevDryRun := w.conductor.DryRunEnabled()
	if params.DryRun {
		w.conductor.SetDryRun(true)
		defer w.conductor.SetDryRun(prevDryRun)
	}

	// Submit optimization job
	jobID, err := w.conductor.Optimize(ctx)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "optimizing",
		"job_id": jobID,
		"state":  w.conductor.State(),
	})
}

type SimplifyParams struct {
	DryRun bool `json:"dry_run,omitempty"` // Simulate without executing agent
}

func (w *WorktreeSocket) handleSimplify(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params SimplifyParams
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}

	// Apply dry-run mode for this call
	prevDryRun := w.conductor.DryRunEnabled()
	if params.DryRun {
		w.conductor.SetDryRun(true)
		defer w.conductor.SetDryRun(prevDryRun)
	}

	jobID, err := w.conductor.Simplify(ctx)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "simplifying",
		"job_id": jobID,
		"state":  w.conductor.State(),
	})
}

func (w *WorktreeSocket) handleShutdown(ctx context.Context, req *Request) (*Response, error) {
	// Send response before shutting down.
	go func() {
		time.Sleep(50 * time.Millisecond)
		if err := w.server.Stop(); err != nil {
			slog.Error("shutdown: failed to stop server", "error", err)
		}
	}()

	return NewResultResponse(req.ID, map[string]string{"status": "shutting_down"})
}

type ReviewParams struct {
	Approve bool   `json:"approve"`
	Reject  bool   `json:"reject"`
	Message string `json:"message,omitempty"`
	Fix     bool   `json:"fix,omitempty"`   // Auto-fix issues after entering review
	Force   bool   `json:"force,omitempty"` // Allow re-entering review from already-reviewed state
}

func (w *WorktreeSocket) handleReview(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params ReviewParams
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}

	if err := w.conductor.Review(ctx, params.Fix); err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	// Record review result if approve/reject specified
	if params.Approve || params.Reject {
		w.conductor.AddReview(params.Approve, params.Message)
	}

	return NewResultResponse(req.ID, map[string]any{
		"status":  "reviewing",
		"state":   w.conductor.State(),
		"message": params.Message,
	})
}

type SubmitParams struct {
	Title        string            `json:"title,omitempty"`
	Body         string            `json:"body,omitempty"`
	Draft        bool              `json:"draft,omitempty"`
	Reviewers    []string          `json:"reviewers,omitempty"`
	Labels       []string          `json:"labels,omitempty"`
	DeleteBranch bool              `json:"delete_branch,omitempty"` // Delete local branch after submit
	DryRun       bool              `json:"dry_run,omitempty"`       // Preview PR without creating it
	Sections     map[string]string `json:"sections,omitempty"`      // Per-PR custom sections (not yet wired into PR body generation)
}

func (w *WorktreeSocket) handleSubmit(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params SubmitParams
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}

	// Dry-run: preview PR without creating it
	if params.DryRun {
		preview, err := w.conductor.PreviewSubmit(ctx)
		if err != nil {
			return NewErrorResponse(req.ID, -32603, err.Error()), nil
		}

		return NewResultResponse(req.ID, map[string]any{
			"status":  "preview",
			"preview": preview,
		})
	}

	if err := w.conductor.Submit(ctx, params.DeleteBranch); err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "submitted",
		"state":  w.conductor.State(),
	})
}

// FinishParams holds params for the task.finish handler.
type FinishParams struct {
	DeleteRemote bool `json:"delete_remote,omitempty"` // Delete the remote feature branch
	Force        bool `json:"force,omitempty"`         // Finish even if PR is not merged
}

func (w *WorktreeSocket) handleFinish(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params FinishParams
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}

	result, err := w.conductor.Finish(ctx, conductor.FinishOptions{
		DeleteRemoteBranch: params.DeleteRemote,
		Force:              params.Force,
	})
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"previous_branch":       result.PreviousBranch,
		"current_branch":        result.CurrentBranch,
		"branch_deleted":        result.BranchDeleted,
		"remote_branch_deleted": result.RemoteBranchDeleted,
	})
}

func (w *WorktreeSocket) handleRefresh(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	result, err := w.conductor.Refresh(ctx)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"task_id":             result.TaskID,
		"branch":              result.Branch,
		"pr_status":           result.PRStatus,
		"pr_merged":           result.PRMerged,
		"pr_url":              result.PRURL,
		"commits_behind_base": result.CommitsBehindBase,
		"action":              result.Action,
		"message":             result.Message,
	})
}

// RemoteApproveParams holds params for the remote.approve handler.
type RemoteApproveParams struct {
	Comment string `json:"comment,omitempty"`
}

func (w *WorktreeSocket) handleRemoteApprove(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params RemoteApproveParams
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}

	if err := w.conductor.ApprovePR(ctx, params.Comment); err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "approved",
		"state":  w.conductor.State(),
	})
}

// RemoteMergeParams holds params for the remote.merge handler.
type RemoteMergeParams struct {
	Method string `json:"method,omitempty"` // merge, squash, rebase (default: rebase)
}

func (w *WorktreeSocket) handleRemoteMerge(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params RemoteMergeParams
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}

	if err := w.conductor.MergePR(ctx, params.Method); err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "merged",
		"state":  w.conductor.State(),
	})
}

func (w *WorktreeSocket) handleAbort(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	if err := w.conductor.Abort(ctx); err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "aborted",
		"state":  w.conductor.State(),
	})
}

func (w *WorktreeSocket) handleStop(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	if err := w.conductor.Stop(ctx); err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "stopped",
		"state":  w.conductor.State(),
	})
}

func (w *WorktreeSocket) handleReset(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	if err := w.conductor.Reset(ctx); err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "reset",
		"state":  w.conductor.State(),
	})
}

// --- Abandon Handler ---

// AbandonParams holds params for the abandon handler.
type AbandonParams struct {
	KeepBranch bool `json:"keep_branch,omitempty"`
}

func (w *WorktreeSocket) handleAbandon(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params AbandonParams
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}

	if err := w.conductor.Abandon(ctx, params.KeepBranch); err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "abandoned",
	})
}

// --- Delete Handler ---

// DeleteParams holds params for the delete handler.
type DeleteParams struct {
	DeleteBranch bool `json:"delete_branch,omitempty"`
}

func (w *WorktreeSocket) handleDelete(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params DeleteParams
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}

	if err := w.conductor.Delete(ctx, params.DeleteBranch); err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "deleted",
	})
}

// --- Update Handler ---

// UpdateParams holds params for the update handler.
type UpdateParams struct{}

// UpdateResult is the response for the update handler.
type UpdateResult struct {
	Status           string `json:"status"`
	Changed          bool   `json:"changed"`
	NewSpecification string `json:"new_specification,omitempty"`
}

func (w *WorktreeSocket) handleUpdate(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	changed, specPath, err := w.conductor.UpdateTask(ctx)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	result := UpdateResult{
		Status:  "updated",
		Changed: changed,
	}
	if specPath != "" {
		result.NewSpecification = specPath
	}

	return NewResultResponse(req.ID, result)
}

// --- Review History Handlers ---

// ReviewListResult is the response for review.list.
type ReviewListResult struct {
	Reviews []storage.Review `json:"reviews"`
}

func (w *WorktreeSocket) handleReviewList(ctx context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		// No conductor: return empty list for basic sockets
		return NewResultResponse(req.ID, ReviewListResult{Reviews: []storage.Review{}})
	}

	reviews, err := w.conductor.ListReviews()
	if err != nil {
		// No task or no store: return empty list rather than error
		return NewResultResponse(req.ID, ReviewListResult{Reviews: []storage.Review{}})
	}

	return NewResultResponse(req.ID, ReviewListResult{Reviews: reviews})
}

// ReviewViewParams holds params for review.view.
type ReviewViewParams struct {
	Number int `json:"number"`
}

func (w *WorktreeSocket) handleReviewView(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params ReviewViewParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	review, err := w.conductor.GetReview(params.Number)
	if err != nil {
		return NewErrorResponse(req.ID, -32604, fmt.Sprintf("review %d not found", params.Number)), nil //nolint:nilerr // JSON-RPC error response
	}

	return NewResultResponse(req.ID, review)
}

// --- Quality Gate Handlers ---

type qualityRespondParams struct {
	PromptID string `json:"prompt_id"`
	Answer   bool   `json:"answer"`
}

func (w *WorktreeSocket) handleQualityRespond(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params qualityRespondParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
	}

	if params.PromptID == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "prompt_id required"), nil
	}

	if err := w.conductor.RespondToPrompt(params.PromptID, params.Answer); err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{"status": "answered"})
}

// --- Checkpoint Handlers ---

type UndoParams struct {
	Steps int `json:"steps"`
}

func (w *WorktreeSocket) handleUndo(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params UndoParams
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}
	if params.Steps < 1 {
		params.Steps = 1
	}

	for range params.Steps {
		if err := w.conductor.Undo(ctx); err != nil {
			return NewErrorResponse(req.ID, -32603, err.Error()), nil
		}
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "undone",
		"steps":  params.Steps,
		"state":  w.conductor.State(),
	})
}

type RedoParams struct {
	Steps int `json:"steps"`
}

func (w *WorktreeSocket) handleRedo(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params RedoParams
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}
	if params.Steps < 1 {
		params.Steps = 1
	}

	for range params.Steps {
		if err := w.conductor.Redo(ctx); err != nil {
			return NewErrorResponse(req.ID, -32603, err.Error()), nil
		}
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "redone",
		"steps":  params.Steps,
		"state":  w.conductor.State(),
	})
}

// CheckpointGotoParams holds params for checkpoint.goto.
type CheckpointGotoParams struct {
	SHA string `json:"sha"`
}

func (w *WorktreeSocket) handleCheckpointGoto(ctx context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	var params CheckpointGotoParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	if params.SHA == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "sha is required"), nil
	}

	if err := w.conductor.GotoCheckpoint(ctx, params.SHA); err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "ok",
		"sha":    params.SHA,
		"state":  w.conductor.State(),
	})
}

// handleCheckpointPreview returns the diff between HEAD and a target checkpoint SHA.
func (w *WorktreeSocket) handleCheckpointPreview(ctx context.Context, req *Request) (*Response, error) {
	if w.repo == nil {
		return NewErrorResponse(req.ID, -32600, "no git repository"), nil
	}

	var params CheckpointGotoParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	if params.SHA == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "sha is required"), nil
	}

	diff, err := w.repo.DiffAgainst(ctx, params.SHA, false)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	stat, _ := w.repo.DiffAgainst(ctx, params.SHA, true)

	return NewResultResponse(req.ID, map[string]any{
		"sha":  params.SHA,
		"diff": diff,
		"stat": stat,
	})
}

// CheckpointInfo holds a checkpoint SHA enriched with git commit metadata.
type CheckpointInfo struct {
	SHA       string `json:"sha"`
	Message   string `json:"message"`
	Author    string `json:"author"`
	Timestamp string `json:"timestamp"`
	State     string `json:"state,omitempty"` // Conductor state at checkpoint time (from persisted metadata)
}

func (w *WorktreeSocket) handleCheckpoints(ctx context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewResultResponse(req.ID, map[string]any{
			"checkpoints": []CheckpointInfo{},
			"redo_stack":  []CheckpointInfo{},
		})
	}

	wu := w.conductor.WorkUnit()
	if wu == nil {
		return NewResultResponse(req.ID, map[string]any{
			"checkpoints": []CheckpointInfo{},
			"redo_stack":  []CheckpointInfo{},
		})
	}

	meta := wu.CheckpointMeta
	enrich := func(shas []string) []CheckpointInfo {
		result := make([]CheckpointInfo, 0, len(shas))
		for _, sha := range shas {
			info := CheckpointInfo{SHA: sha}
			if w.repo != nil {
				if entry, err := w.repo.CommitInfo(ctx, sha); err == nil {
					info.Message = entry.Message
					info.Author = entry.Author
					info.Timestamp = entry.Date
				}
			}
			if m, ok := meta[sha]; ok {
				info.State = m.State
			}
			result = append(result, info)
		}

		return result
	}

	return NewResultResponse(req.ID, map[string]any{
		"checkpoints": enrich(wu.Checkpoints),
		"redo_stack":  enrich(wu.RedoStack),
	})
}

// --- Git Handlers ---

func (w *WorktreeSocket) handleGitStatus(ctx context.Context, req *Request) (*Response, error) {
	if w.repo == nil {
		return NewErrorResponse(req.ID, -32600, "no git repository"), nil
	}

	branch, err := w.repo.CurrentBranch(ctx)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	hasChanges, err := w.repo.HasUncommittedChanges(ctx)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	files, _ := w.repo.DiffFilesWithStatus(ctx)
	if files == nil {
		files = []git.FileStatus{}
	}

	return NewResultResponse(req.ID, map[string]any{
		"branch":      branch,
		"has_changes": hasChanges,
		"files":       files,
	})
}

type GitDiffParams struct {
	Cached bool `json:"cached"`
}

func (w *WorktreeSocket) handleGitDiff(ctx context.Context, req *Request) (*Response, error) {
	if w.repo == nil {
		return NewErrorResponse(req.ID, -32600, "no git repository"), nil
	}

	var params GitDiffParams
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}

	diff, err := w.repo.Diff(ctx, params.Cached)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"diff": diff,
	})
}

type GitDiffAgainstParams struct {
	Ref  string `json:"ref"`
	Stat bool   `json:"stat"`
}

func (w *WorktreeSocket) handleGitDiffAgainst(ctx context.Context, req *Request) (*Response, error) {
	if w.repo == nil {
		return NewErrorResponse(req.ID, -32600, "no git repository"), nil
	}

	var params GitDiffAgainstParams
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}

	if params.Ref == "" {
		return NewErrorResponse(req.ID, -32602, "ref parameter is required"), nil
	}

	diff, err := w.repo.DiffAgainst(ctx, params.Ref, params.Stat)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"diff": diff,
	})
}

type GitLogParams struct {
	Count int `json:"count"`
}

func (w *WorktreeSocket) handleGitLog(ctx context.Context, req *Request) (*Response, error) {
	if w.repo == nil {
		return NewErrorResponse(req.ID, -32600, "no git repository"), nil
	}

	var params GitLogParams
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}
	if params.Count < 1 {
		params.Count = 10
	}

	entries, err := w.repo.Log(ctx, params.Count)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"entries": entries,
	})
}

// --- Streaming Handler ---

func (w *WorktreeSocket) handleStreamSubscribe(ctx context.Context, req *Request, conn net.Conn) (*Response, error) {
	var params struct {
		LastSeq uint64 `json:"last_seq,omitempty"`
	}
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}

	subID := fmt.Sprintf("sub-%d", time.Now().UnixNano())
	ch := make(chan []byte, 100)

	// Subscribe before snapshotting the buffer so no events are missed between
	// replay and live delivery.
	w.streamsMu.Lock()
	w.streams[subID] = ch
	w.streamsMu.Unlock()

	// Replay missed events if the client provides a last known sequence number.
	if params.LastSeq > 0 {
		w.replayMu.Lock()
		// Copy ring buffer in chronological order (oldest → newest).
		snapshot := make([][]byte, replayBufSize)
		for i := range replayBufSize {
			snapshot[i] = w.replayBuf[(w.replayHead+i)%replayBufSize]
		}
		w.replayMu.Unlock()

		var seqCheck struct {
			Seq uint64 `json:"seq"`
		}
		for _, entry := range snapshot {
			if entry == nil {
				continue
			}
			if err := json.Unmarshal(entry, &seqCheck); err != nil {
				continue
			}
			if seqCheck.Seq > params.LastSeq {
				if _, err := conn.Write(entry); err != nil {
					w.streamsMu.Lock()
					delete(w.streams, subID)
					w.streamsMu.Unlock()
					close(ch)

					return nil, fmt.Errorf("replay: %w", err)
				}
			}
		}
	}

	// Drain the subscription channel and write events to the connection.
	// A 30s heartbeat detects closed connections when events are infrequent.
	go func() {
		defer func() {
			w.streamsMu.Lock()
			delete(w.streams, subID)
			w.streamsMu.Unlock()
		}()
		// Heartbeats are keepalive signals, intentionally without seq numbers.
		// They are not part of the ordered event stream and not buffered for replay.
		heartbeat := []byte("{\"type\":\"heartbeat\"}\n")
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case event, ok := <-ch:
				if !ok {
					return
				}
				if _, err := conn.Write(event); err != nil {
					return
				}
			case <-ticker.C:
				if _, err := conn.Write(heartbeat); err != nil {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return NewResultResponse(req.ID, map[string]any{
		"subscription_id": subID,
		"status":          "subscribed",
	})
}

// --- Browse Handler ---

// WorktreeBrowseParams holds params for browse.
type WorktreeBrowseParams struct {
	Path  string `json:"path"`
	Files bool   `json:"files"` // include .md/.txt files
}

// WorktreeBrowseEntry represents a file or directory entry.
type WorktreeBrowseEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
}

// handleWorktreeFilesList lists files in the worktree for mentions/autocomplete.
// Mirrors the global handleFilesList but scoped to this worktree's path.
func (w *WorktreeSocket) handleWorktreeFilesList(_ context.Context, req *Request) (*Response, error) {
	var params struct {
		Path       string   `json:"path"`
		Extensions []string `json:"extensions,omitempty"`
		MaxDepth   int      `json:"max_depth,omitempty"`
	}
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}

	basePath := w.path
	if params.Path != "" {
		// Join to worktree root first, then verify the resolved path is still within it.
		joined := filepath.Join(w.path, filepath.Clean(params.Path))
		resolved, absErr := filepath.Abs(joined)
		if absErr != nil {
			return nil, fmt.Errorf("resolve path: %w", absErr)
		}
		if !strings.HasPrefix(resolved+string(filepath.Separator), w.path+string(filepath.Separator)) {
			return NewErrorResponse(req.ID, -32602, "path outside worktree"), nil
		}
		basePath = resolved
	}

	maxDepth := params.MaxDepth
	if maxDepth <= 0 || maxDepth > 10 {
		maxDepth = 3
	}

	type fileEntry struct {
		Name  string `json:"name"`
		Path  string `json:"path"`
		IsDir bool   `json:"is_dir"`
		Size  int64  `json:"size,omitempty"`
	}

	var entries []fileEntry
	skipDirs := map[string]bool{
		"node_modules": true, "vendor": true, "dist": true,
		"build": true, "__pycache__": true, ".git": true,
	}

	_ = filepath.WalkDir(basePath, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // Continue on individual errors
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}

			return nil
		}
		relPath, _ := filepath.Rel(basePath, p)
		depth := strings.Count(relPath, string(filepath.Separator))
		if depth > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}

			return nil
		}
		if d.IsDir() && skipDirs[d.Name()] {
			return filepath.SkipDir
		}
		if len(params.Extensions) > 0 && !d.IsDir() {
			ext := strings.ToLower(filepath.Ext(d.Name()))
			found := false
			for _, e := range params.Extensions {
				if ext == e || ext == "."+e {
					found = true

					break
				}
			}
			if !found {
				return nil
			}
		}
		info, _ := d.Info()
		var size int64
		if info != nil && !d.IsDir() {
			size = info.Size()
		}
		entries = append(entries, fileEntry{
			Name:  d.Name(),
			Path:  relPath,
			IsDir: d.IsDir(),
			Size:  size,
		})

		return nil
	})

	return NewResultResponse(req.ID, map[string]any{"files": entries})
}

func (w *WorktreeSocket) handleBrowse(ctx context.Context, req *Request) (*Response, error) {
	var params WorktreeBrowseParams
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}

	path := params.Path
	if path == "" {
		path = w.path // default to worktree path
	}
	path = filepath.Clean(path)

	// Validate path is within worktree to prevent path traversal
	path, err := ValidatePathWithRoots([]string{w.path}, path)
	if err != nil {
		return NewErrorResponse(req.ID, -32602, "access denied: path outside worktree"), nil //nolint:nilerr // JSON-RPC error response
	}

	info, err := os.Stat(path)
	if err != nil {
		return NewErrorResponse(req.ID, -32602, "path not found"), nil //nolint:nilerr // JSON-RPC error response
	}
	if !info.IsDir() {
		return NewErrorResponse(req.ID, -32602, "not a directory"), nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, "cannot read directory"), nil //nolint:nilerr // JSON-RPC error response
	}

	result := []WorktreeBrowseEntry{}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue // skip hidden
		}

		if e.IsDir() {
			result = append(result, WorktreeBrowseEntry{
				Name:  name,
				Path:  filepath.Join(path, name),
				IsDir: true,
			})
		} else if params.Files {
			// Include .md and .txt files
			ext := strings.ToLower(filepath.Ext(name))
			if ext == ".md" || ext == ".txt" {
				result = append(result, WorktreeBrowseEntry{
					Name:  name,
					Path:  filepath.Join(path, name),
					IsDir: false,
				})
			}
		}
	}

	return NewResultResponse(req.ID, map[string]any{
		"path":    path,
		"parent":  filepath.Dir(path),
		"entries": result,
	})
}

// --- Screenshot Handlers ---

type ScreenshotListParams struct {
	TaskID string `json:"task_id"`
}

func (w *WorktreeSocket) handleScreenshotsList(ctx context.Context, req *Request) (*Response, error) {
	var params ScreenshotListParams
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}

	// Get task ID from params or current work unit
	taskID := params.TaskID
	if taskID == "" && w.conductor != nil {
		if wu := w.conductor.WorkUnit(); wu != nil {
			taskID = wu.ID
		}
	}

	if taskID == "" {
		return NewResultResponse(req.ID, map[string]any{
			"screenshots": []screenshot.Screenshot{},
		})
	}

	screenshots, err := w.screenshots.List(taskID)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"screenshots": screenshots,
	})
}

type ScreenshotGetParams struct {
	TaskID       string `json:"task_id"`
	ScreenshotID string `json:"screenshot_id"`
}

func (w *WorktreeSocket) handleScreenshotsGet(ctx context.Context, req *Request) (*Response, error) {
	var params ScreenshotGetParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
	}

	// Get task ID from params or current work unit
	taskID := params.TaskID
	if taskID == "" && w.conductor != nil {
		if wu := w.conductor.WorkUnit(); wu != nil {
			taskID = wu.ID
		}
	}

	if taskID == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "task_id required"), nil
	}

	if params.ScreenshotID == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "screenshot_id required"), nil
	}

	ss, err := w.screenshots.Get(taskID, params.ScreenshotID)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, ss)
}

type ScreenshotCaptureParams struct {
	TaskID string `json:"task_id,omitempty"`
	Source string `json:"source"` // "agent" or "user"
	Step   string `json:"step,omitempty"`
	Agent  string `json:"agent,omitempty"`
	Format string `json:"format,omitempty"` // "png" or "jpeg"
	Data   string `json:"data"`             // base64 encoded image
}

func (w *WorktreeSocket) handleScreenshotsCapture(ctx context.Context, req *Request) (*Response, error) {
	var params ScreenshotCaptureParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
	}

	// Get task ID from params or current work unit
	taskID := params.TaskID
	if taskID == "" && w.conductor != nil {
		if wu := w.conductor.WorkUnit(); wu != nil {
			taskID = wu.ID
		}
	}

	if taskID == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "task_id required (no active task)"), nil
	}

	if params.Data == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "data required"), nil
	}

	// Decode base64 image data
	imageData, err := base64.StdEncoding.DecodeString(params.Data)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid base64 data"), nil //nolint:nilerr // JSON-RPC error response
	}

	source := params.Source
	if source == "" {
		source = screenshot.SourceUser
	}

	opts := screenshot.SaveOptions{
		Source: source,
		Step:   params.Step,
		Agent:  params.Agent,
		Format: params.Format,
	}

	ss, err := w.screenshots.Save(taskID, imageData, opts)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	// Emit screenshot_captured event to all subscribers
	w.emitEvent("screenshot_captured", ss)

	return NewResultResponse(req.ID, ss)
}

type ScreenshotDeleteParams struct {
	TaskID       string `json:"task_id"`
	ScreenshotID string `json:"screenshot_id"`
}

func (w *WorktreeSocket) handleScreenshotsDelete(ctx context.Context, req *Request) (*Response, error) {
	var params ScreenshotDeleteParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
	}

	// Get task ID from params or current work unit
	taskID := params.TaskID
	if taskID == "" && w.conductor != nil {
		if wu := w.conductor.WorkUnit(); wu != nil {
			taskID = wu.ID
		}
	}

	if taskID == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "task_id required"), nil
	}

	if params.ScreenshotID == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "screenshot_id required"), nil
	}

	if err := w.screenshots.Delete(taskID, params.ScreenshotID); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	// Emit screenshot_deleted event to all subscribers
	w.emitEvent("screenshot_deleted", map[string]string{
		"id":      params.ScreenshotID,
		"task_id": taskID,
	})

	return NewResultResponse(req.ID, map[string]any{
		"success": true,
	})
}

// emitEvent broadcasts an event to all stream subscribers.
func (w *WorktreeSocket) emitEvent(eventType string, data any) {
	event := map[string]any{
		"type":      eventType,
		"data":      data,
		"timestamp": time.Now(),
	}

	eventData, err := json.Marshal(event)
	if err != nil {
		return
	}

	enriched := w.injectSeqAndBuffer(eventData)

	w.streamsMu.RLock()
	for _, ch := range w.streams {
		select {
		case ch <- enriched:
		default:
			slog.Warn("worktree event channel full, dropping event", "type", eventType)
		}
	}
	w.streamsMu.RUnlock()
}

// --- Progress Estimation ---

func (w *WorktreeSocket) handleProgressGet(_ context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	estimate := w.conductor.GetProgressEstimate()
	if estimate == nil {
		return NewResultResponse(req.ID, map[string]any{
			"active": false,
		})
	}

	return NewResultResponse(req.ID, map[string]any{
		"active":      true,
		"percent":     estimate.Percent,
		"eta_seconds": estimate.ETASeconds,
		"signals":     estimate.Signals,
		"calibrated":  estimate.Calibrated,
	})
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

// handleContextResolve resolves a context reference to its content.
// Used by the web UI for @-mention preview and validation.
func (w *WorktreeSocket) handleContextResolve(ctx context.Context, req *Request) (*Response, error) {
	var params struct {
		Type string `json:"type"` // "file", "symbol", "commit", "branch", "terminal", "url"
		Ref  string `json:"ref"`  // Reference to resolve
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, -32602, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
	}
	if params.Type == "" || params.Ref == "" {
		return NewErrorResponse(req.ID, -32602, "type and ref are required"), nil
	}

	resolver := &conductor.ContextResolver{
		WorktreeRoot: w.path,
		Repo:         w.repo,
		Graph:        w.codegraphInst, // may be nil — symbol resolution falls back to grep
	}

	item := conductor.ContextItem{
		Type: conductor.ContextType(params.Type),
		Ref:  params.Ref,
	}

	resolved, err := resolver.Resolve(ctx, item)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	resp, err := NewResultResponse(req.ID, map[string]string{
		"label":   resolved.Label,
		"content": resolved.Content,
	})
	if err != nil {
		return NewErrorResponse(req.ID, -32603, "encode result: "+err.Error()), nil //nolint:nilerr // JSON-RPC error
	}

	return resp, nil
}

// handleProvisionPreview returns what would be provisioned without executing.
func (w *WorktreeSocket) handleProvisionPreview(_ context.Context, req *Request) (*Response, error) {
	cfg, _, _, err := settings.LoadEffective(w.path)
	if err != nil {
		cfg = settings.DefaultSettings()
	}

	if !settings.BoolValue(cfg.Git.Provision.Enabled, true) {
		return NewResultResponse(req.ID, map[string]string{"status": "disabled"})
	}

	defaults := provision.DefaultOptions(w.path)
	userOpts := provision.Options{
		CopyPatterns:    cfg.Git.Provision.CopyPatterns,
		SymlinkPatterns: cfg.Git.Provision.SymlinkPatterns,
		SetupCommands:   cfg.Git.Provision.SetupCommands,
	}
	merged := provision.MergeOptions(defaults, userOpts)

	result, previewErr := provision.Preview(w.path, merged)
	if previewErr != nil {
		return NewErrorResponse(req.ID, -32603, previewErr.Error()), nil
	}

	resp, respErr := NewResultResponse(req.ID, result)
	if respErr != nil {
		return NewErrorResponse(req.ID, -32603, "encode result: "+respErr.Error()), nil //nolint:nilerr // JSON-RPC error
	}

	return resp, nil
}
