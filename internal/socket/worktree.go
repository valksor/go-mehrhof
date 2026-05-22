package socket

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valksor/kvelmo/agent/strategy"
	"github.com/valksor/kvelmo/internal/codegraph"
	"github.com/valksor/kvelmo/internal/conductor"
	"github.com/valksor/kvelmo/internal/eventlog"
	"github.com/valksor/kvelmo/internal/git"
	"github.com/valksor/kvelmo/internal/memory"
	"github.com/valksor/kvelmo/internal/provider"
	"github.com/valksor/kvelmo/internal/screenshot"
	"github.com/valksor/kvelmo/internal/storage"
	"github.com/valksor/kvelmo/internal/worker"
	"github.com/valksor/kvelmo/settings"
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

	// Wire storage.Store so specs/reviews/sessions are persisted via internal/storage.
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
	// Done in the background: getMemoryAdapter loads the embedding model (an
	// 86 MB download on first run), and the socket must bind without waiting on
	// it — otherwise startup blocks past the caller's readiness timeout. The
	// indexer is optional; memory.search degrades gracefully until it is set.
	// SetMemoryIndexer locks the conductor mutex, so this is safe concurrently.
	go func() {
		if adapter, adapterErr := getMemoryAdapter(context.Background()); adapterErr == nil {
			idxr := memory.NewIndexer(adapter.Store(), cfg.WorktreePath)
			cond.SetMemoryIndexer(idxr)
		}
	}()

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

	// MCP-facing surface — consumed by the kvelmo MCP server (cmd/kvelmo/commands/mcp.go)
	// which exposes these as MCP tools to interactive Claude Code sessions
	// launched by the claudemcp adapter.
	w.server.Handle("mcp.task.get", w.handleMCPTaskGet)
	w.server.Handle("mcp.task.specifications", w.handleMCPSpecifications)
	w.server.Handle("mcp.files.read", w.handleMCPFileRead)
	w.server.Handle("mcp.artifacts.save", w.handleMCPArtifactsSave)
	w.server.Handle("mcp.checkpoints.create", w.handleMCPCheckpoint)
	w.server.Handle("mcp.signal.complete", w.handleMCPSignalComplete)
	w.server.Handle("mcp.signal.failure", w.handleMCPSignalFailure)

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
	return NewResultResponse(req.ID, map[string]string{keyStatus: "ok"})
}

func (w *WorktreeSocket) handleStrategyList(_ context.Context, req *Request) (*Response, error) {
	return NewResultResponse(req.ID, strategy.List())
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
