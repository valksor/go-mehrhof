// Package conductor orchestrates the task lifecycle workflow.
// Based on flow_v2.md design specification.
package conductor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/valksor/kvelmo/pkg/agent/strategy"
	"github.com/valksor/kvelmo/pkg/eventlog"
	"github.com/valksor/kvelmo/pkg/failclass"
	"github.com/valksor/kvelmo/pkg/findings"
	"github.com/valksor/kvelmo/pkg/git"
	"github.com/valksor/kvelmo/pkg/graph"
	"github.com/valksor/kvelmo/pkg/memory"
	"github.com/valksor/kvelmo/pkg/progress"
	"github.com/valksor/kvelmo/pkg/provider"
	"github.com/valksor/kvelmo/pkg/respcache"
	"github.com/valksor/kvelmo/pkg/security"
	"github.com/valksor/kvelmo/pkg/settings"
	"github.com/valksor/kvelmo/pkg/storage"
	"github.com/valksor/kvelmo/pkg/varpool"
	"github.com/valksor/kvelmo/pkg/worker"
)

// Conductor orchestrates the task automation workflow.
// Per flow_v2.md: "One conductor, one entrypoint. The socket IS the conductor.".
//
//nolint:containedctx // lifecycleCtx is intentionally stored to manage background goroutines that outlive request contexts
type Conductor struct {
	mu sync.RWMutex

	// ── Core ────────────────────────────────────────────────────────────
	machine    *Machine
	worktree   string // Worktree path (current directory or git worktree)
	pool       *worker.Pool
	git        *git.Repository
	providers  *provider.Registry
	globalPath string
	opts       Options
	stdout     io.Writer
	stderr     io.Writer

	// ── Lifecycle ───────────────────────────────────────────────────────
	lifecycleCtx    context.Context
	lifecycleCancel context.CancelFunc
	closeOnce       sync.Once
	closed          atomic.Bool

	// ── Task State ──────────────────────────────────────────────────────
	workUnit          *WorkUnit
	activeJobID       string           // ID of currently running job (for cancellation)
	activeScheduler   *graph.Scheduler // Currently running graph scheduler (for node approvals)
	phaseStartedAt    time.Time        // When the current phase started executing
	specWatcher       *specWatcher     // Watches spec files for mid-execution changes
	taskQueue         []*QueuedTask    // Pending tasks to auto-start after current finishes
	autoAdvance       bool             // When true, plan_done → implement, implement_done → review
	runtimeSkipPhases []string         // Per-invocation phase names to skip (merged with config)
	dryRun            bool             // Simulates phases without agent execution
	varPool           *varpool.Pool    // Sharing context between graph nodes

	// ── Event System ────────────────────────────────────────────────────
	events         chan ConductorEvent
	eventsMu       sync.Mutex // Protects events channel send during close
	listeners      []EventListener
	listenersMu    sync.RWMutex         // Protects listeners (separate from mu to avoid deadlock in emit)
	pendingPrompts map[string]chan bool // Blocking user prompts; key: UUID prompt ID; protected by c.mu

	// ── Strategy & Retry ────────────────────────────────────────────────
	strategy         strategy.Strategy            // Agent strategy (default: "direct")
	phaseStrategies  map[string]strategy.Strategy // Per-phase overrides take precedence
	iterationCount   map[string]int               // phase → current iteration count
	maxIterations    int                          // default 3
	phasePolicies    map[string]PhasePolicy       // Per-phase failure policies (retry, skip, or fail)
	retryCount       map[string]int               // phase → current retry count
	router           PhaseRouter                  // Evaluates phase output and decides next action
	lastFailureClass FailureClass                 // Classification of the most recent phase failure
	autoFixAttempt   int                          // Auto-fix loop: current attempt
	autoFixLastErr   string                       // Auto-fix loop: last error message

	// ── Quality & Progress ──────────────────────────────────────────────
	qualityGateRunning  bool                              // Guard: true while quality gate is running under mu unlock
	qualityGateDone     sync.WaitGroup                    // Signals when an in-flight quality gate finishes
	adversarialFindings []findings.Finding                // Most recent adversarial review results
	failclassHistory    *failclass.History                // Failure classification history across quality gate runs
	responseCache       *respcache.Cache                  // Avoids redundant agent calls on identical prompts
	progressEstimator   *progress.Estimator               // Progress estimation for active phases
	progressCalibrator  *progress.Calibrator              // Historical calibration for progress estimates
	cachedSettings      atomic.Pointer[settings.Settings] // Lock-free cached settings

	// ── Service Integrations (optional) ─────────────────────────────────
	memoryIndexer    *memory.Indexer         // Set via SetMemoryIndexer
	store            *storage.Store          // Set via SetStore
	notifier         Notifier                // Set via SetNotifier
	qualityRunner    QualityRunner           // Set via SetQualityRunner
	metricsRecorder  MetricsRecorder         // Set via SetMetricsRecorder
	canaryHarness    *security.CanaryHarness // Credential sandboxing (nil when disabled)
	eventLog         *eventlog.Log           // Orchestration state auditing
	taskGroupChecker TaskGroupChecker        // Cross-repo group readiness check during submit
}

// ConductorEvent represents an event emitted by the conductor.
type ConductorEvent struct {
	Type           string          `json:"type"`
	State          State           `json:"state,omitempty"`
	JobID          string          `json:"job_id,omitempty"`
	NodeID         string          `json:"node_id,omitempty"` // Graph node that produced this event
	CorrelationID  string          `json:"correlation_id,omitempty"`
	Message        string          `json:"message,omitempty"`
	Data           json.RawMessage `json:"data,omitempty"`
	Error          string          `json:"error,omitempty"`
	Phase          string          `json:"phase,omitempty"` // Phase that produced this event (plan, implement, etc.)
	FailureClass   FailureClass    `json:"failure_class,omitempty"`
	FailureMessage string          `json:"failure_message,omitempty"`
	SubTaskID      string          `json:"sub_task_id,omitempty"` // Set when event originates from a sub-task
	Timestamp      time.Time       `json:"timestamp"`
}

// EventListener is called when events occur.
type EventListener func(event ConductorEvent)

// FailurePolicy defines how a phase handles errors.
type FailurePolicy int

const (
	// FailurePolicyFail stops execution and waits for user intervention (default).
	FailurePolicyFail FailurePolicy = iota
	// FailurePolicyRetry re-runs the phase up to MaxRetries times.
	FailurePolicyRetry
	// FailurePolicySkip marks the phase as completed and advances the workflow.
	FailurePolicySkip
)

// PhasePolicy configures error handling for a specific phase.
type PhasePolicy struct {
	Policy     FailurePolicy
	MaxRetries int           // For FailurePolicyRetry
	RetryDelay time.Duration // For FailurePolicyRetry
}

// defaultPhasePolicies returns sensible defaults for each phase.
// Plan retries once on transient failure; review failures require user attention; implement retries twice;
// simplify and optimize are optional and can be skipped on failure.
func defaultPhasePolicies() map[string]PhasePolicy {
	return map[string]PhasePolicy{
		"plan":      {Policy: FailurePolicyRetry, MaxRetries: 1, RetryDelay: 5 * time.Second},
		"implement": {Policy: FailurePolicyRetry, MaxRetries: 2, RetryDelay: 5 * time.Second},
		"simplify":  {Policy: FailurePolicySkip},
		"optimize":  {Policy: FailurePolicySkip},
		"review":    {Policy: FailurePolicyFail},
		"submit":    {Policy: FailurePolicyFail},
	}
}

// loadPhasePoliciesFromSettings applies user-configured phase policy overrides.
func (c *Conductor) loadPhasePoliciesFromSettings() {
	s := c.getEffectiveSettings()
	if s == nil {
		return
	}
	for phase, policyStr := range s.Workflow.PhasePolicies {
		switch policyStr {
		case "fail":
			c.phasePolicies[phase] = PhasePolicy{Policy: FailurePolicyFail}
		case "retry":
			retryDelay := time.Duration(s.Workflow.Retry.BackoffSeconds) * time.Second
			if retryDelay == 0 {
				retryDelay = 5 * time.Second
			}
			maxAttempts := s.Workflow.Retry.MaxAttempts
			if maxAttempts == 0 {
				maxAttempts = 2
			}
			c.phasePolicies[phase] = PhasePolicy{
				Policy:     FailurePolicyRetry,
				MaxRetries: maxAttempts,
				RetryDelay: retryDelay,
			}
		case "skip":
			c.phasePolicies[phase] = PhasePolicy{Policy: FailurePolicySkip}
		}
	}
}

// loadStrategiesFromSettings applies user-configured strategy overrides.
func (c *Conductor) loadStrategiesFromSettings() {
	s := c.getEffectiveSettings()
	if s == nil {
		return
	}
	// Default strategy.
	if s.Agent.Strategy != "" {
		if start, ok := strategy.Get(s.Agent.Strategy); ok {
			c.strategy = start
		}
	}
	// Per-phase strategy overrides.
	for phase, stratName := range s.Agent.PhaseStrategy {
		if start, ok := strategy.Get(stratName); ok {
			if c.phaseStrategies == nil {
				c.phaseStrategies = make(map[string]strategy.Strategy)
			}
			c.phaseStrategies[phase] = start
		}
	}
}

// Options configures the conductor.
type Options struct {
	WorkDir    string
	Verbose    bool
	GlobalPath string
	Pool       *worker.Pool
	Stdout     io.Writer
	Stderr     io.Writer
	// Settings overrides file-based settings loading when non-nil (useful in tests).
	Settings *settings.Settings
	// DryRun simulates phase execution without spawning agents.
	DryRun bool
}

// DefaultOptions returns default conductor options.
func DefaultOptions() Options {
	return Options{
		WorkDir: ".",
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
	}
}

// Option is a functional option for the conductor.
type Option func(*Options)

// WithWorkDir sets the working directory.
func WithWorkDir(dir string) Option {
	return func(o *Options) { o.WorkDir = dir }
}

// WithVerbose enables verbose output.
func WithVerbose(v bool) Option {
	return func(o *Options) { o.Verbose = v }
}

// WithPool sets the worker pool.
func WithPool(p *worker.Pool) Option {
	return func(o *Options) { o.Pool = p }
}

// WithStdout sets the stdout writer.
func WithStdout(w io.Writer) Option {
	return func(o *Options) { o.Stdout = w }
}

// WithStderr sets the stderr writer.
func WithStderr(w io.Writer) Option {
	return func(o *Options) { o.Stderr = w }
}

// WithSettings overrides file-based settings loading with the provided settings.
// Useful in tests to inject specific configuration without filesystem access.
func WithSettings(s *settings.Settings) Option {
	return func(o *Options) { o.Settings = s }
}

// WithDryRun enables dry-run mode (no agent execution).
func WithDryRun(v bool) Option {
	return func(o *Options) { o.DryRun = v }
}

// New creates a new Conductor with the given options.
func New(opts ...Option) (*Conductor, error) {
	options := DefaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	// Resolve working directory
	workDir, err := filepath.Abs(options.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("resolve work dir: %w", err)
	}

	// Load settings so provider tokens are available.
	// Settings come from local .env files (never global env vars).
	// WithSettings() can override this for testing.
	var effectiveSettings *settings.Settings
	if options.Settings != nil {
		effectiveSettings = options.Settings
	} else {
		var err error
		effectiveSettings, _, _, err = settings.LoadEffective(workDir)
		if err != nil {
			// Non-fatal: use defaults if settings fail to load.
			effectiveSettings = settings.DefaultSettings()
		}
	}

	// Create state machine
	machine := NewMachine()

	// Create provider registry with tokens from settings
	providers := provider.NewRegistry(effectiveSettings)

	lifecycleCtx, lifecycleCancel := context.WithCancel(context.Background())

	c := &Conductor{
		machine:            machine,
		worktree:           workDir,
		pool:               options.Pool,
		providers:          providers,
		globalPath:         options.GlobalPath,
		lifecycleCtx:       lifecycleCtx,
		lifecycleCancel:    lifecycleCancel,
		events:             make(chan ConductorEvent, 100),
		pendingPrompts:     make(map[string]chan bool),
		opts:               options,
		stdout:             options.Stdout,
		stderr:             options.Stderr,
		varPool:            varpool.New(),
		iterationCount:     make(map[string]int),
		maxIterations:      3,
		phasePolicies:      defaultPhasePolicies(),
		retryCount:         make(map[string]int),
		dryRun:             options.DryRun,
		router:             NewDefaultRouter(),
		progressCalibrator: progress.NewCalibrator(),
	}
	c.cachedSettings.Store(effectiveSettings) // Cache pre-loaded settings (atomic)
	c.loadPhasePoliciesFromSettings()
	c.loadStrategiesFromSettings()
	c.initResponseCache(effectiveSettings)

	// Subscribe to state machine changes
	machine.AddListener(c.onStateChanged)

	// Register status sync listener for bidirectional provider status updates
	c.setupStatusSync()

	// Register post-transition hook listener
	c.setupPostHooks()

	return c, nil
}

// Initialize initializes the conductor with git repository.
func (c *Conductor) Initialize(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Open git repository
	repo, err := git.Open(c.worktree)
	if err != nil {
		c.logVerbosef("Warning: not a git repository: %v", err)
		// Continue without git - some operations will be limited
	} else {
		c.git = repo

		// Apply commit signing setting
		s := c.getEffectiveSettings()
		if s.Git.SignCommits != nil && *s.Git.SignCommits {
			repo.SetSignCommits(true)
		}

		// Enforce signed commits policy
		if s.Workflow.Policy.RequireSignedCommits && !repo.IsSigningConfigured(ctx) {
			return errors.New("policy requires signed commits but GPG signing is not configured. Run: git config commit.gpgsign true")
		}
	}

	return nil
}

// setupPostHooks registers a state machine listener that runs post-transition hooks
// after each successful state change. This allows workflows like running security scans
// after implementation or audit logging after submission.
func (c *Conductor) setupPostHooks() {
	c.machine.AddListener(func(_, _ State, event Event, _ *WorkUnit) {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			if err := c.RunPostTransitionHooks(ctx, event); err != nil {
				slog.Warn("post-transition hook failed", "event", event, "error", err)
				c.emit(ConductorEvent{
					Type:    "hook_error",
					Message: fmt.Sprintf("Post-%s hook failed: %v", event, err),
				})
			}
		}()
	})
}

// State returns the current workflow state.
func (c *Conductor) State() State {
	return c.machine.State()
}

// WorkUnit returns the current work unit (alias for GetWorkUnit).
func (c *Conductor) WorkUnit() *WorkUnit {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.workUnit
}

// Repo returns the git repository for the conductor's worktree.
func (c *Conductor) Repo() *git.Repository {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.git
}

// GetWorkUnit returns the current work unit.
func (c *Conductor) GetWorkUnit() *WorkUnit {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.workUnit == nil {
		return nil
	}
	// Return a copy
	wu := *c.workUnit

	return &wu
}

// MarkDirty persists the current work unit state to disk.
// Use after modifying work unit fields like Tags or Priority directly.
func (c *Conductor) MarkDirty() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.workUnit != nil {
		c.workUnit.UpdatedAt = time.Now()
	}
	c.persistState()
}

// Machine returns the state machine.
func (c *Conductor) Machine() *Machine {
	return c.machine
}

// TaskTraceID returns the current task's trace ID. Returns empty string if
// no task is loaded.
func (c *Conductor) TaskTraceID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.workUnit == nil {
		return ""
	}

	return c.workUnit.TaskTraceID
}

// SetAutoAdvance enables or disables automatic phase progression.
// When enabled, the conductor automatically advances through phases:
// plan_done → implement, implement_done → review.
func (c *Conductor) SetAutoAdvance(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.autoAdvance = enabled
}

// AutoAdvance returns whether automatic phase progression is enabled.
func (c *Conductor) AutoAdvance() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.autoAdvance
}

// SetSkipPhases sets per-invocation phases to skip during auto-advance.
// These are merged with config-level SkipPhases.
func (c *Conductor) SetSkipPhases(phases []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.runtimeSkipPhases = phases
}

// SetContextItems attaches context references to the current work unit.
// Items are lightweight references resolved at dispatch time, not persisted content.
func (c *Conductor) SetContextItems(items []ContextItem) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.workUnit != nil {
		c.workUnit.ContextItems = items
		c.persistState()

		// Emit event so TUI and web UI see the attached context
		if len(items) > 0 {
			refs := make([]string, 0, len(items))
			for _, item := range items {
				refs = append(refs, fmt.Sprintf("@%s %s", item.Type, item.Ref))
			}
			go c.emit(ConductorEvent{
				Type:    "context_attached",
				Message: "Context attached: " + strings.Join(refs, ", "),
			})
		}
	}
}

// SkipPhases returns the effective skip phases (runtime + config merged).
func (c *Conductor) SkipPhases() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var merged []string
	if s := c.getEffectiveSettings(); s != nil {
		merged = append(merged, s.Workflow.SkipPhases...)
	}
	for _, p := range c.runtimeSkipPhases {
		if !slices.Contains(merged, p) {
			merged = append(merged, p)
		}
	}

	return merged
}

// Suggestions returns workflow suggestions based on historical task patterns.
func (c *Conductor) Suggestions() ([]memory.SkipSuggestion, []memory.AgentSuggestion) {
	if c.store == nil {
		return nil, nil
	}

	tasks, err := c.store.ListArchivedTasks()
	if err != nil {
		slog.Debug("failed to list archived tasks for suggestions", "error", err)

		return nil, nil
	}

	return memory.DetectSkipPatterns(tasks), memory.DetectAgentPatterns(tasks)
}

// getWorkDir returns the effective working directory for operations.
// When worktree isolation is active, returns the isolated worktree path.
// Otherwise returns the main worktree (project root).
func (c *Conductor) getWorkDir() string {
	if c.workUnit != nil && c.workUnit.WorktreePath != "" {
		return c.workUnit.WorktreePath
	}

	return c.worktree
}

// getBaseBranch returns the base branch from settings or git detection.
// Returns error if neither is available (no silent fallback).
// This method is lock-free to allow calling from methods that already hold c.mu.
func (c *Conductor) getBaseBranch(ctx context.Context) (string, error) {
	// 1. Check settings override
	if settings := c.getEffectiveSettings(); settings != nil && settings.Git.BaseBranch != "" {
		return settings.Git.BaseBranch, nil
	}

	// 2. Auto-detect from git
	if c.git != nil {
		return c.git.DefaultBranch(ctx)
	}

	return "", errors.New("cannot determine base branch: git not available and git.base_branch not configured")
}

// GetBaseBranch returns the base branch from settings or git detection.
func (c *Conductor) GetBaseBranch(ctx context.Context) (string, error) {
	return c.getBaseBranch(ctx)
}

// GetEffectiveSettings returns the effective (merged) settings.
func (c *Conductor) GetEffectiveSettings() *settings.Settings {
	return c.getEffectiveSettings()
}

// getEffectiveSettings returns cached settings, loading them on first access.
// Settings are cached to avoid repeated file I/O across phases.
// This method is lock-free to allow calling from methods that already hold c.mu.
func (c *Conductor) getEffectiveSettings() *settings.Settings {
	// Fast path: return cached settings (lock-free)
	if cached := c.cachedSettings.Load(); cached != nil {
		return cached
	}

	// Slow path: load settings (only happens if ReloadSettings() was called)
	effectiveSettings, _, _, err := settings.LoadEffective(c.worktree)
	if err != nil {
		// Non-fatal: fall back to defaults when settings cannot be loaded.
		effectiveSettings = settings.DefaultSettings()
		c.logVerbosef("Warning: could not load settings: %v — using defaults", err)
	}

	// Compare-and-swap to avoid race with concurrent reload
	c.cachedSettings.CompareAndSwap(nil, effectiveSettings)

	return c.cachedSettings.Load()
}

// ReloadSettings clears the cached settings, forcing a reload on next access.
// Use this if settings have been changed and need to be refreshed.
func (c *Conductor) ReloadSettings() {
	c.cachedSettings.Store(nil)
}

// EventTypeUserPrompt is emitted when the conductor needs a yes/no answer from the user.
const EventTypeUserPrompt = "user_prompt"

// promptUser emits a user_prompt event and blocks until the socket delivers
// an answer via RespondToPrompt, or ctx is cancelled.
// Must NOT be called while holding c.mu.
func (c *Conductor) promptUser(ctx context.Context, question string) (bool, error) {
	promptID := "prompt-" + uuid.New().String()
	ch := make(chan bool, 1)

	c.mu.Lock()
	c.pendingPrompts[promptID] = ch
	c.mu.Unlock()

	c.emit(ConductorEvent{
		Type:    EventTypeUserPrompt,
		Message: question,
		Data: mustMarshalJSON(map[string]string{
			"prompt_id": promptID,
			"question":  question,
		}),
	})

	select {
	case answer := <-ch:
		return answer, nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pendingPrompts, promptID)
		c.mu.Unlock()

		return false, ctx.Err()
	}
}

// PendingPromptIDs returns the IDs of all currently pending user prompts.
// Used by status to surface actionable items to CLI users.
func (c *Conductor) PendingPromptIDs() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	ids := make([]string, 0, len(c.pendingPrompts))
	for id := range c.pendingPrompts {
		ids = append(ids, id)
	}

	return ids
}

// RespondToPrompt delivers an answer to a pending promptUser call.
// Called by the quality.respond socket handler.
func (c *Conductor) RespondToPrompt(promptID string, answer bool) error {
	c.mu.Lock()
	ch, ok := c.pendingPrompts[promptID]
	if ok {
		delete(c.pendingPrompts, promptID)
	}
	c.mu.Unlock()

	if !ok {
		return fmt.Errorf("prompt %q not found or already answered", promptID)
	}

	ch <- answer

	return nil
}

// mustMarshalJSON marshals v to JSON, panicking on error.
// Only for use with known-good data types where marshaling cannot fail.
func mustMarshalJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("mustMarshalJSON: %v", err))
	}

	return b
}

// phaseToScope maps a phase name to its varpool scope constant.
func phaseToScope(phase string) string {
	switch phase {
	case "plan":
		return varpool.ScopePlan
	case "implement":
		return varpool.ScopeImplement
	case "simplify":
		return varpool.ScopeSimplify
	case "optimize":
		return varpool.ScopeOptimize
	case "review":
		return varpool.ScopeReview
	default:
		return phase
	}
}

// resetPhaseState clears all per-phase transient state before (re-)entering a phase.
// This prevents iteration counts, retry counts, stale quality gate results, and
// scoped varpool data from leaking across phase re-entries.
// Quality gate is always cleared (not just on re-entry) to avoid stale results
// from a previous lifecycle influencing the current one.
// Must be called while holding c.mu.
func (c *Conductor) resetPhaseState(phase string) {
	c.iterationCount[phase] = 0
	c.retryCount[phase] = 0
	if c.workUnit != nil {
		c.workUnit.QualityGatePassed = nil
		c.workUnit.QualityGateError = ""
	}
	c.varPool.ClearScope(phaseToScope(phase))
}

// DryRunEnabled returns whether the conductor is in dry-run mode.
func (c *Conductor) DryRunEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.dryRun
}

// SetDryRun enables or disables dry-run mode.
func (c *Conductor) SetDryRun(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.dryRun = v
}

// AutoFixStatus returns the current state of the quality gate auto-fix loop.
type AutoFixStatus struct {
	Active      bool   `json:"active"`
	Attempt     int    `json:"attempt"`
	MaxAttempts int    `json:"max_attempts"`
	LastError   string `json:"last_error,omitempty"`
}

// GetAutoFixStatus returns the current auto-fix loop state.
func (c *Conductor) GetAutoFixStatus() AutoFixStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	s := c.getEffectiveSettings()
	maxAttempts := 3
	if s != nil && s.Workflow.AutoFix.MaxAttempts > 0 {
		maxAttempts = s.Workflow.AutoFix.MaxAttempts
	}

	return AutoFixStatus{
		Active:      c.autoFixAttempt > 0,
		Attempt:     c.autoFixAttempt,
		MaxAttempts: maxAttempts,
		LastError:   c.autoFixLastErr,
	}
}

// LastFailureClass returns the classification of the most recent phase failure.
func (c *Conductor) LastFailureClass() FailureClass {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.lastFailureClass
}

func (c *Conductor) logVerbosef(format string, args ...any) {
	if c.opts.Verbose && c.stdout != nil {
		_, _ = fmt.Fprintf(c.stdout, format+"\n", args...)
	}
}

// Status returns the current status for display.
func (c *Conductor) Status() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()

	status := map[string]any{
		"state":    c.machine.State(),
		"worktree": c.worktree,
	}

	if c.workUnit != nil {
		status["task"] = map[string]any{
			"id":          c.workUnit.ID,
			"title":       c.workUnit.Title,
			"branch":      c.workUnit.Branch,
			"checkpoints": len(c.workUnit.Checkpoints),
			"jobs":        len(c.workUnit.Jobs),
		}
	}

	return status
}

// OnEvent registers an event listener (alias for AddListener).
func (c *Conductor) OnEvent(listener EventListener) {
	c.AddListener(listener)
}

// ForceWorkUnit directly sets the work unit on the conductor.
// Intended for use in tests and internal tooling that need to
// set up a known state without going through the full Start flow.
func (c *Conductor) ForceWorkUnit(wu *WorkUnit) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.workUnit = wu
	c.machine.SetWorkUnit(wu)
}

// ConductorConfig configures a conductor instance for use by socket layer.
type ConductorConfig struct {
	Repo         *git.Repository
	Pool         *worker.Pool
	Providers    *provider.Registry
	WorktreePath string // Optional: explicit project directory path; falls back to Repo.Path() if empty
}

// NewConductor creates a new conductor with explicit configuration.
// This is used by the socket package to avoid circular imports.
func NewConductor(cfg ConductorConfig) *Conductor {
	machine := NewMachine()

	providers := cfg.Providers
	if providers == nil {
		// Fallback to default settings if no providers passed.
		// In practice, callers should always pass providers with proper tokens.
		providers = provider.NewRegistry(settings.DefaultSettings())
	}

	lifecycleCtx, lifecycleCancel := context.WithCancel(context.Background())

	c := &Conductor{
		machine:            machine,
		git:                cfg.Repo,
		pool:               cfg.Pool,
		providers:          providers,
		lifecycleCtx:       lifecycleCtx,
		lifecycleCancel:    lifecycleCancel,
		events:             make(chan ConductorEvent, 100),
		pendingPrompts:     make(map[string]chan bool),
		stdout:             os.Stdout,
		stderr:             os.Stderr,
		varPool:            varpool.New(),
		iterationCount:     make(map[string]int),
		maxIterations:      3,
		phasePolicies:      defaultPhasePolicies(),
		retryCount:         make(map[string]int),
		router:             NewDefaultRouter(),
		progressCalibrator: progress.NewCalibrator(),
	}

	// Set worktree path - prefer explicit config, fallback to repo path
	if cfg.WorktreePath != "" {
		c.worktree = cfg.WorktreePath
	} else if cfg.Repo != nil {
		c.worktree = cfg.Repo.Path()
	}

	machine.AddListener(c.onStateChanged)

	// Register status sync listener for bidirectional provider status updates
	c.setupStatusSync()

	// Register post-transition hook listener
	c.setupPostHooks()

	// Load auto_advance from effective settings (can still be overridden via SetAutoAdvance)
	if s := c.getEffectiveSettings(); s != nil {
		if settings.BoolValue(s.Workflow.AutoAdvance, false) {
			c.autoAdvance = true
		}
	}

	return c
}
