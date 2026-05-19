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

	"github.com/valksor/kvelmo/agent/strategy"
	"github.com/valksor/kvelmo/internal/eventlog"
	"github.com/valksor/kvelmo/internal/findings"
	"github.com/valksor/kvelmo/internal/git"
	"github.com/valksor/kvelmo/internal/graph"
	"github.com/valksor/kvelmo/internal/memory"
	"github.com/valksor/kvelmo/internal/provider"
	"github.com/valksor/kvelmo/internal/quality"
	"github.com/valksor/kvelmo/internal/respcache"
	"github.com/valksor/kvelmo/internal/security"
	"github.com/valksor/kvelmo/internal/storage"
	"github.com/valksor/kvelmo/internal/varpool"
	"github.com/valksor/kvelmo/internal/worker"
	"github.com/valksor/kvelmo/settings"
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
	qualityGateCh       chan struct{}                     // Closed when an in-flight quality gate finishes; nil when idle
	adversarialFindings []findings.Finding                // Most recent adversarial review results
	failclassHistory    *quality.FailHistory              // Failure classification history across quality gate runs
	responseCache       *respcache.Cache                  // Avoids redundant agent calls on identical prompts
	progressEstimator   *ProgressEstimator                // Progress estimation for active phases
	progressCalibrator  *ProgressCalibrator               // Historical calibration for progress estimates
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
		PhasePlan:      {Policy: FailurePolicyRetry, MaxRetries: 1, RetryDelay: 5 * time.Second},
		PhaseImplement: {Policy: FailurePolicyRetry, MaxRetries: 2, RetryDelay: 5 * time.Second},
		PhaseSimplify:  {Policy: FailurePolicySkip},
		PhaseOptimize:  {Policy: FailurePolicySkip},
		PhaseReview:    {Policy: FailurePolicyFail},
		PhaseSubmit:    {Policy: FailurePolicyFail},
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
		progressCalibrator: NewProgressCalibrator(),
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
		progressCalibrator: NewProgressCalibrator(),
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

	// Apply user-configured phase policies, strategies, and auto_advance.
	c.loadPhasePoliciesFromSettings()
	c.loadStrategiesFromSettings()
	if s := c.getEffectiveSettings(); s != nil {
		if settings.BoolValue(s.Workflow.AutoAdvance, false) {
			c.autoAdvance = true
		}
	}

	return c
}
