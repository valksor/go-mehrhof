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
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/valksor/kvelmo/pkg/agent/strategy"
	"github.com/valksor/kvelmo/pkg/git"
	"github.com/valksor/kvelmo/pkg/memory"
	"github.com/valksor/kvelmo/pkg/provider"
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

	// Core components
	machine    *Machine
	worktree   string // Worktree path (current directory or git worktree)
	pool       *worker.Pool
	git        *git.Repository
	providers  *provider.Registry
	globalPath string

	// Lifecycle context for background goroutines (watchJob, indexer)
	// Cancelled when Close() is called
	lifecycleCtx    context.Context
	lifecycleCancel context.CancelFunc

	// Close protection
	closeOnce sync.Once
	closed    atomic.Bool

	// Current task state
	workUnit    *WorkUnit
	activeJobID string // ID of currently running job (for cancellation)

	// Task queue (pending tasks to auto-start after current finishes)
	taskQueue []*QueuedTask

	// Event streaming
	events      chan ConductorEvent
	eventsMu    sync.Mutex // Protects events channel send during close
	listeners   []EventListener
	listenersMu sync.RWMutex // Protects listeners (separate from mu to avoid deadlock in emit)

	// pendingPrompts holds channels for blocking user prompts.
	// Key: UUID prompt ID. Protected by c.mu.
	pendingPrompts map[string]chan bool

	// autoAdvance triggers automatic progression through phases when jobs complete.
	// When true, plan_done → implement, implement_done → review.
	autoAdvance bool

	// Configuration
	opts Options

	// Output writers
	stdout io.Writer
	stderr io.Writer

	// Memory indexer (optional, set via SetMemoryIndexer)
	memoryIndexer *memory.Indexer

	// Storage (optional, set via SetStore)
	store *storage.Store

	// Optional service integrations (set via SetNotifier, SetQualityRunner, SetMetricsRecorder)
	notifier        Notifier
	qualityRunner   QualityRunner
	metricsRecorder MetricsRecorder

	// Variable pool for sharing context between graph nodes
	varPool *varpool.Pool

	// Canary harness for credential sandboxing (nil when disabled)
	canaryHarness *security.CanaryHarness

	// Agent strategy (default: "direct"). Per-phase overrides take precedence.
	strategy        strategy.Strategy
	phaseStrategies map[string]strategy.Strategy

	// Iteration tracking for strategy evaluation (bounded re-submission).
	// When a strategy's EvaluateOutput returns "needs_iteration", the phase
	// is re-submitted up to maxIterations times before accepting the result.
	iterationCount map[string]int // phase → current iteration count
	maxIterations  int            // default 3

	// Per-phase failure policies (retry, skip, or fail).
	// Inspired by Dify/RAGFlow per-node error strategies.
	phasePolicies map[string]PhasePolicy
	retryCount    map[string]int // phase → current retry count

	// Cached settings (loaded once, reused across phases).
	// Uses atomic.Pointer for lock-free access to avoid deadlock when called
	// from methods that already hold c.mu.
	cachedSettings atomic.Pointer[settings.Settings]
}

// ConductorEvent represents an event emitted by the conductor.
type ConductorEvent struct {
	Type          string          `json:"type"`
	State         State           `json:"state,omitempty"`
	JobID         string          `json:"job_id,omitempty"`
	NodeID        string          `json:"node_id,omitempty"` // Graph node that produced this event
	CorrelationID string          `json:"correlation_id,omitempty"`
	Message       string          `json:"message,omitempty"`
	Data          json.RawMessage `json:"data,omitempty"`
	Error         string          `json:"error,omitempty"`
	Timestamp     time.Time       `json:"timestamp"`
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
// Plan and review failures require user attention; implement retries once;
// simplify and optimize are optional and can be skipped on failure.
func defaultPhasePolicies() map[string]PhasePolicy {
	return map[string]PhasePolicy{
		"plan":      {Policy: FailurePolicyFail},
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
		machine:         machine,
		worktree:        workDir,
		pool:            options.Pool,
		providers:       providers,
		globalPath:      options.GlobalPath,
		lifecycleCtx:    lifecycleCtx,
		lifecycleCancel: lifecycleCancel,
		events:          make(chan ConductorEvent, 100),
		pendingPrompts:  make(map[string]chan bool),
		opts:            options,
		stdout:          options.Stdout,
		stderr:          options.Stderr,
		varPool:         varpool.New(),
		iterationCount:  make(map[string]int),
		maxIterations:   3,
		phasePolicies:   defaultPhasePolicies(),
		retryCount:      make(map[string]int),
	}
	c.cachedSettings.Store(effectiveSettings) // Cache pre-loaded settings (atomic)
	c.loadPhasePoliciesFromSettings()
	c.loadStrategiesFromSettings()

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

func (c *Conductor) logVerbosef(format string, args ...any) {
	if c.opts.Verbose && c.stdout != nil {
		_, _ = fmt.Fprintf(c.stdout, format+"\n", args...)
	}
}

// Status returns the current status for display.
func (c *Conductor) Status() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	status := map[string]interface{}{
		"state":    c.machine.State(),
		"worktree": c.worktree,
	}

	if c.workUnit != nil {
		status["task"] = map[string]interface{}{
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
		machine:         machine,
		git:             cfg.Repo,
		pool:            cfg.Pool,
		providers:       providers,
		lifecycleCtx:    lifecycleCtx,
		lifecycleCancel: lifecycleCancel,
		events:          make(chan ConductorEvent, 100),
		pendingPrompts:  make(map[string]chan bool),
		stdout:          os.Stdout,
		stderr:          os.Stderr,
		varPool:         varpool.New(),
		iterationCount:  make(map[string]int),
		maxIterations:   3,
		phasePolicies:   defaultPhasePolicies(),
		retryCount:      make(map[string]int),
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
