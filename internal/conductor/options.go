package conductor

import (
	"io"
	"os"
	"time"
)

// Options configures the Conductor
type Options struct {
	// Agent configuration
	AgentName  string            // Which agent to use for all steps (default: auto-detect)
	StepAgents map[string]string // Per-step agent overrides (e.g., {"planning": "glm", "implementing": "claude"})
	Timeout    time.Duration     // Agent execution timeout

	// Behavior
	DryRun       bool // If true, don't apply file changes
	Verbose      bool // Enable verbose output
	CreateBranch bool // Create git branch for task
	UseWorktree  bool // Create git worktree for task
	AutoInit     bool // Auto-initialize workspace if needed

	// Yolo mode (full automation)
	YoloMode           bool // Enable full automation mode
	SkipAgentQuestions bool // Skip agent questions, proceed with best guess
	MaxQualityRetries  int  // Max retries for quality loop (default: 3)

	// Context preservation
	IncludeFullContext bool // Include full exploration context from pending question (default: summary only)

	// Output
	Stdout io.Writer // Where to write output (default: os.Stdout)
	Stderr io.Writer // Where to write errors (default: os.Stderr)

	// Paths
	WorkDir string // Working directory (default: current dir)

	// Provider configuration
	DefaultProvider string // Default provider for bare references (e.g., "file")

	// Naming overrides (CLI flags)
	ExternalKey           string // Override external key (e.g., "FEATURE-123")
	CommitPrefixTemplate  string // Override commit prefix template (e.g., "[{key}]")
	BranchPatternTemplate string // Override branch pattern template (e.g., "{type}/{key}--{slug}")

	// Callbacks
	OnStateChange func(from, to string)
	OnProgress    func(message string, percent int)
	OnError       func(err error)
}

// Option is a functional option for configuring Conductor
type Option func(*Options)

// DefaultOptions returns default options
func DefaultOptions() Options {
	return Options{
		AgentName:         "", // Auto-detect
		Timeout:           30 * time.Minute,
		DryRun:            false,
		Verbose:           false,
		Stdout:            os.Stdout,
		Stderr:            os.Stderr,
		WorkDir:           ".",
		MaxQualityRetries: 3,
	}
}

// WithAgent sets the agent name for all steps
func WithAgent(name string) Option {
	return func(o *Options) {
		o.AgentName = name
	}
}

// WithStepAgent sets a specific agent for a workflow step
func WithStepAgent(step, agentName string) Option {
	return func(o *Options) {
		if o.StepAgents == nil {
			o.StepAgents = make(map[string]string)
		}
		o.StepAgents[step] = agentName
	}
}

// WithTimeout sets the execution timeout
func WithTimeout(d time.Duration) Option {
	return func(o *Options) {
		o.Timeout = d
	}
}

// WithDryRun enables dry-run mode
func WithDryRun(enabled bool) Option {
	return func(o *Options) {
		o.DryRun = enabled
	}
}

// WithVerbose enables verbose output
func WithVerbose(enabled bool) Option {
	return func(o *Options) {
		o.Verbose = enabled
	}
}

// WithCreateBranch enables git branch creation
func WithCreateBranch(enabled bool) Option {
	return func(o *Options) {
		o.CreateBranch = enabled
	}
}

// WithUseWorktree enables git worktree creation
func WithUseWorktree(enabled bool) Option {
	return func(o *Options) {
		o.UseWorktree = enabled
		// Worktree implies branch creation
		if enabled {
			o.CreateBranch = true
		}
	}
}

// WithAutoInit enables auto-initialization of workspace
func WithAutoInit(enabled bool) Option {
	return func(o *Options) {
		o.AutoInit = enabled
	}
}

// WithYoloMode enables full automation mode
func WithYoloMode(enabled bool) Option {
	return func(o *Options) {
		o.YoloMode = enabled
		if enabled {
			o.SkipAgentQuestions = true
		}
	}
}

// WithSkipAgentQuestions skips pending questions from agents
func WithSkipAgentQuestions(enabled bool) Option {
	return func(o *Options) {
		o.SkipAgentQuestions = enabled
	}
}

// WithMaxQualityRetries sets max retries for quality loop
func WithMaxQualityRetries(n int) Option {
	return func(o *Options) {
		o.MaxQualityRetries = n
	}
}

// WithIncludeFullContext enables including full exploration context from pending question
func WithIncludeFullContext(enabled bool) Option {
	return func(o *Options) {
		o.IncludeFullContext = enabled
	}
}

// WithStdout sets the stdout writer
func WithStdout(w io.Writer) Option {
	return func(o *Options) {
		o.Stdout = w
	}
}

// WithStderr sets the stderr writer
func WithStderr(w io.Writer) Option {
	return func(o *Options) {
		o.Stderr = w
	}
}

// WithWorkDir sets the working directory
func WithWorkDir(dir string) Option {
	return func(o *Options) {
		o.WorkDir = dir
	}
}

// WithDefaultProvider sets the default provider for bare references
func WithDefaultProvider(provider string) Option {
	return func(o *Options) {
		o.DefaultProvider = provider
	}
}

// WithExternalKey sets the external key override for branch/commit naming
func WithExternalKey(key string) Option {
	return func(o *Options) {
		o.ExternalKey = key
	}
}

// WithCommitPrefixTemplate sets the commit prefix template override
func WithCommitPrefixTemplate(template string) Option {
	return func(o *Options) {
		o.CommitPrefixTemplate = template
	}
}

// WithBranchPatternTemplate sets the branch pattern template override
func WithBranchPatternTemplate(template string) Option {
	return func(o *Options) {
		o.BranchPatternTemplate = template
	}
}

// WithStateChangeCallback sets the state change callback
func WithStateChangeCallback(fn func(from, to string)) Option {
	return func(o *Options) {
		o.OnStateChange = fn
	}
}

// WithProgressCallback sets the progress callback
func WithProgressCallback(fn func(message string, percent int)) Option {
	return func(o *Options) {
		o.OnProgress = fn
	}
}

// WithErrorCallback sets the error callback
func WithErrorCallback(fn func(err error)) Option {
	return func(o *Options) {
		o.OnError = fn
	}
}

// Apply applies options to the Options struct
func (o *Options) Apply(opts ...Option) {
	for _, opt := range opts {
		opt(o)
	}
}

// TalkOptions configures talk mode
type TalkOptions struct {
	Continue    bool   // Continue existing session
	SessionFile string // Specific session file to continue
}

// FinishOptions configures the finish operation
type FinishOptions struct {
	SquashMerge  bool   // Use squash merge
	DeleteBranch bool   // Delete branch after merge
	TargetBranch string // Branch to merge into
	PushAfter    bool   // Push after merge

	// PR-related options (for GitHub provider)
	CreatePR bool   // Create PR instead of local merge
	DraftPR  bool   // Create PR as draft
	PRTitle  string // Custom PR title (defaults to task title)
	PRBody   string // Custom PR body
}

// DefaultFinishOptions returns default finish options
func DefaultFinishOptions() FinishOptions {
	return FinishOptions{
		SquashMerge:  true,
		DeleteBranch: true,
		TargetBranch: "", // Auto-detect base branch
		PushAfter:    false,
		CreatePR:     false,
		DraftPR:      false,
	}
}

// DeleteOptions configures the delete operation
type DeleteOptions struct {
	Force       bool // Skip confirmation prompt
	KeepBranch  bool // Keep the git branch (only delete workspace)
	KeepWorkDir bool // Keep the work directory (only delete branch)
}

// DefaultDeleteOptions returns default delete options
func DefaultDeleteOptions() DeleteOptions {
	return DeleteOptions{
		Force:       false,
		KeepBranch:  false,
		KeepWorkDir: false,
	}
}
