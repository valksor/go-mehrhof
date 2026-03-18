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
	"github.com/valksor/kvelmo/pkg/conductor"
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

	w := &WorktreeSocket{
		server:      NewServer(cfg.SocketPath),
		path:        cfg.WorktreePath,
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
	w := &WorktreeSocket{
		server:     NewServer(socketPath),
		path:       worktreePath,
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
	w.server.Handle("ping", w.handlePing)
	w.server.Handle("strategy.list", w.handleStrategyList)
	w.server.Handle("checkpoints", w.handleCheckpoints)
	w.server.Handle("checkpoint.goto", w.handleCheckpointGoto)

	// Git handlers (work if repo is set)
	w.server.Handle("git.status", w.handleGitStatus)
	w.server.Handle("git.diff", w.handleGitDiff)
	w.server.Handle("git.diff_against", w.handleGitDiffAgainst)
	w.server.Handle("git.log", w.handleGitLog)

	// File browsing
	w.server.Handle("browse", w.handleBrowse)

	// Streaming (required for frontend connection)
	w.server.HandleWithConn("stream.subscribe", w.handleStreamSubscribe)

	// Review history (gracefully handles missing conductor)
	w.server.Handle("review.list", w.handleReviewList)
}

func (w *WorktreeSocket) registerHandlers() {
	// Basic
	w.server.Handle("ping", w.handlePing)
	w.server.Handle("status", w.handleStatus)
	w.server.Handle("strategy.list", w.handleStrategyList)

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

	// Review history
	w.server.Handle("review.list", w.handleReviewList)
	w.server.Handle("review.view", w.handleReviewView)

	// Quality gate user prompts
	w.server.Handle("quality.respond", w.handleQualityRespond)

	// Checkpoint navigation
	w.server.Handle("undo", w.handleUndo)
	w.server.Handle("redo", w.handleRedo)
	w.server.Handle("checkpoints", w.handleCheckpoints)
	w.server.Handle("checkpoint.goto", w.handleCheckpointGoto)

	// Git operations
	w.server.Handle("git.status", w.handleGitStatus)
	w.server.Handle("git.diff", w.handleGitDiff)
	w.server.Handle("git.diff_against", w.handleGitDiffAgainst)
	w.server.Handle("git.log", w.handleGitLog)

	// Streaming
	w.server.HandleWithConn("stream.subscribe", w.handleStreamSubscribe)

	// Show spec/plan content
	w.server.Handle("show.spec", w.handleShowSpec)
	w.server.Handle("show.plan", w.handleShowSpec) // plan output is stored as specifications

	// File browsing
	w.server.Handle("browse", w.handleBrowse)

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
	w.server.Handle("review.checklist.get", w.handleReviewChecklistGet)
	w.server.Handle("review.checklist.check", w.handleReviewChecklistCheck)
	w.server.Handle("review.checklist.uncheck", w.handleReviewChecklistUncheck)

	// CI status
	w.server.Handle("ci.status", w.handleCIStatus)

	// Workflow hooks
	w.server.Handle("hooks.list", w.handleHooksList)
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
			result.Task = &TaskInfo{
				ID:           wu.ID,
				Title:        wu.Title,
				Source:       wu.Source.Reference,
				Branch:       wu.Branch,
				WorktreePath: wu.WorktreePath,
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
	}

	return NewResultResponse(req.ID, result)
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
	if w.conductor == nil {
		return NewErrorResponse(req.ID, -32600, "no conductor configured"), nil
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
	Source               string `json:"source"` // e.g., "github:owner/repo#123"
	UseWorktreeIsolation bool   `json:"use_worktree_isolation,omitempty"`
	AutoAdvance          bool   `json:"auto_advance,omitempty"` // Auto-progress through plan → implement → review
}

func (w *WorktreeSocket) handleStart(ctx context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, -32600, "no conductor configured"), nil
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

	// Determine worktree isolation: explicit param overrides, else check effective settings.
	useIsolation := params.UseWorktreeIsolation
	if !useIsolation {
		if effective, _, _, err := settings.LoadEffective(w.path); err == nil {
			useIsolation = settings.BoolValue(effective.Workflow.UseWorktreeIsolation, true)
		}
	}

	if useIsolation && w.repo != nil {
		wu := w.conductor.WorkUnit()
		if wu != nil {
			isolationBasePath := filepath.Join(w.path, ".kvelmo", "worktrees")
			if err := os.MkdirAll(isolationBasePath, 0o755); err == nil {
				wt, err := w.repo.CreateTaskWorktree(ctx, wu.ID, isolationBasePath)
				if err == nil {
					wu.WorktreePath = wt.Path
					wu.Branch = wt.Branch
					slog.Info("handleStart: created worktree isolation", "path", wt.Path, "branch", wt.Branch)
				} else {
					slog.Warn("handleStart: worktree creation failed", "error", err)
				}
			} else {
				slog.Warn("handleStart: mkdir failed for worktree base", "path", isolationBasePath, "error", err)
			}
		}
	}

	// Auto-advance: trigger planning immediately after start
	if params.AutoAdvance {
		go func() {
			if _, err := w.conductor.Plan(ctx, false); err != nil {
				slog.Warn("auto-advance: plan after start failed", "error", err)
			}
		}()
	}

	return NewResultResponse(req.ID, map[string]any{
		"status": "started",
		"state":  w.conductor.State(),
	})
}

type PlanParams struct {
	Prompt string `json:"prompt,omitempty"` // Additional context for planning
	Force  bool   `json:"force,omitempty"`  // Re-run even if already planned
}

func (w *WorktreeSocket) handlePlan(ctx context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, -32600, "no conductor configured"), nil
	}

	var params PlanParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	// Submit planning job
	jobID, err := w.conductor.Plan(ctx, params.Force)
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
	Prompt string `json:"prompt,omitempty"` // Additional context for implementation
	Force  bool   `json:"force,omitempty"`  // Re-run even if already implemented
}

func (w *WorktreeSocket) handleImplement(ctx context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, -32600, "no conductor configured"), nil
	}

	var params ImplementParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	// Submit implementation job
	jobID, err := w.conductor.Implement(ctx, params.Force)
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
	Prompt string `json:"prompt,omitempty"` // Additional context for optimization
}

func (w *WorktreeSocket) handleOptimize(ctx context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, -32600, "no conductor configured"), nil
	}

	var params OptimizeParams
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
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

func (w *WorktreeSocket) handleSimplify(ctx context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, -32600, "no conductor configured"), nil
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
	Fix     bool   `json:"fix,omitempty"` // Auto-fix issues after entering review
}

func (w *WorktreeSocket) handleReview(ctx context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, -32600, "no conductor configured"), nil
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
	Title        string   `json:"title,omitempty"`
	Body         string   `json:"body,omitempty"`
	Draft        bool     `json:"draft,omitempty"`
	Reviewers    []string `json:"reviewers,omitempty"`
	Labels       []string `json:"labels,omitempty"`
	DeleteBranch bool     `json:"delete_branch,omitempty"` // Delete local branch after submit
}

func (w *WorktreeSocket) handleSubmit(ctx context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, -32600, "no conductor configured"), nil
	}

	var params SubmitParams
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
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
	if w.conductor == nil {
		return NewErrorResponse(req.ID, -32600, "no conductor configured"), nil
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
	if w.conductor == nil {
		return NewErrorResponse(req.ID, -32600, "no conductor configured"), nil
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
	if w.conductor == nil {
		return NewErrorResponse(req.ID, -32600, "no conductor configured"), nil
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
	if w.conductor == nil {
		return NewErrorResponse(req.ID, -32600, "no conductor configured"), nil
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
	if w.conductor == nil {
		return NewErrorResponse(req.ID, -32600, "no conductor configured"), nil
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
	if w.conductor == nil {
		return NewErrorResponse(req.ID, -32600, "no conductor configured"), nil
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
	if w.conductor == nil {
		return NewErrorResponse(req.ID, -32600, "no conductor configured"), nil
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
	if w.conductor == nil {
		return NewErrorResponse(req.ID, -32600, "no conductor configured"), nil
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
	if w.conductor == nil {
		return NewErrorResponse(req.ID, -32600, "no conductor configured"), nil
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
	if w.conductor == nil {
		return NewErrorResponse(req.ID, -32600, "no conductor configured"), nil
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
	if w.conductor == nil {
		return NewErrorResponse(req.ID, -32600, "no conductor configured"), nil
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
	if w.conductor == nil {
		return NewErrorResponse(req.ID, -32600, "no conductor configured"), nil
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
	if w.conductor == nil {
		return NewErrorResponse(req.ID, -32600, "no conductor configured"), nil
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
	if w.conductor == nil {
		return NewErrorResponse(req.ID, -32600, "no conductor configured"), nil
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
	if w.conductor == nil {
		return NewErrorResponse(req.ID, -32600, "no conductor configured"), nil
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

// CheckpointInfo holds a checkpoint SHA enriched with git commit metadata.
type CheckpointInfo struct {
	SHA       string `json:"sha"`
	Message   string `json:"message"`
	Author    string `json:"author"`
	Timestamp string `json:"timestamp"`
}

func (w *WorktreeSocket) handleCheckpoints(ctx context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, -32600, "no conductor configured"), nil
	}

	wu := w.conductor.WorkUnit()
	if wu == nil {
		return NewResultResponse(req.ID, map[string]any{
			"checkpoints": []CheckpointInfo{},
			"redo_stack":  []CheckpointInfo{},
		})
	}

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
	State           TaskState `json:"state"`
	Path            string    `json:"path"`
	Task            *TaskInfo `json:"task,omitempty"`
	PendingPromptID string    `json:"pending_prompt_id,omitempty"`
	ActiveJobID     string    `json:"active_job_id,omitempty"`
	QueueDepth      int       `json:"queue_depth,omitempty"`
	LastError       string    `json:"last_error,omitempty"`
}

type TaskInfo struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Source       string `json:"source"`
	Branch       string `json:"branch,omitempty"`
	WorktreePath string `json:"worktree_path,omitempty"`
}
