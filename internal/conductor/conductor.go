package conductor

import (
	"fmt"
	"io"
	"sync"

	"github.com/valksor/go-mehrhof/internal/agent"
	"github.com/valksor/go-mehrhof/internal/events"
	"github.com/valksor/go-mehrhof/internal/plugin"
	"github.com/valksor/go-mehrhof/internal/provider"
	"github.com/valksor/go-mehrhof/internal/storage"
	"github.com/valksor/go-mehrhof/internal/vcs"
	"github.com/valksor/go-mehrhof/internal/workflow"
)

// Conductor orchestrates the task automation workflow
type Conductor struct {
	mu sync.RWMutex

	// Core components
	machine   *workflow.Machine
	eventBus  *events.Bus
	workspace *storage.Workspace
	git       *vcs.Git

	// Registries
	providers *provider.Registry
	agents    *agent.Registry
	plugins   *plugin.Registry

	// Workflow plugin adapters (for lifecycle management)
	workflowAdapters []*plugin.WorkflowAdapter

	// Current state
	activeTask *storage.ActiveTask
	taskWork   *storage.TaskWork

	// Configuration
	opts Options

	// Active agent
	activeAgent     agent.Agent
	taskAgentConfig *provider.AgentConfig // Agent config from task source (if any)

	// Session tracking (for conversation history and token usage)
	currentSession     *storage.Session
	currentSessionFile string
}

// New creates a new Conductor with the given options
func New(opts ...Option) (*Conductor, error) {
	options := DefaultOptions()
	options.Apply(opts...)

	// Create event bus
	bus := events.NewBus()

	// Create state machine
	machine := workflow.NewMachine(bus)

	// Create registries
	providerRegistry := provider.NewRegistry()
	agentRegistry := agent.NewRegistry()

	c := &Conductor{
		machine:   machine,
		eventBus:  bus,
		providers: providerRegistry,
		agents:    agentRegistry,
		opts:      options,
	}

	// Subscribe to state changes
	bus.Subscribe(events.TypeStateChanged, c.onStateChanged)

	return c, nil
}

// GetProviderRegistry returns the provider registry
func (c *Conductor) GetProviderRegistry() *provider.Registry {
	return c.providers
}

// GetAgentRegistry returns the agent registry
func (c *Conductor) GetAgentRegistry() *agent.Registry {
	return c.agents
}

// GetEventBus returns the event bus
func (c *Conductor) GetEventBus() *events.Bus {
	return c.eventBus
}

// GetWorkspace returns the workspace
func (c *Conductor) GetWorkspace() *storage.Workspace {
	return c.workspace
}

// GetGit returns the git instance
func (c *Conductor) GetGit() *vcs.Git {
	return c.git
}

// GetActiveTask returns the current active task
func (c *Conductor) GetActiveTask() *storage.ActiveTask {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.activeTask
}

// GetTaskWork returns the current task work
func (c *Conductor) GetTaskWork() *storage.TaskWork {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.taskWork
}

// GetActiveAgent returns the active agent
func (c *Conductor) GetActiveAgent() agent.Agent {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.activeAgent
}

// GetMachine returns the state machine
func (c *Conductor) GetMachine() *workflow.Machine {
	return c.machine
}

// GetStdout returns the configured stdout writer
func (c *Conductor) GetStdout() io.Writer {
	return c.opts.Stdout
}

// GetStderr returns the configured stderr writer
func (c *Conductor) GetStderr() io.Writer {
	return c.opts.Stderr
}

// logVerbose logs a message if verbose mode is enabled
func (c *Conductor) logVerbose(format string, args ...any) {
	if c.opts.Verbose && c.opts.Stdout != nil {
		_, _ = fmt.Fprintf(c.opts.Stdout, format+"\n", args...)
	}
}

// logError logs an error using the callback if configured
func (c *Conductor) logError(err error) {
	if c.opts.OnError != nil {
		c.opts.OnError(err)
	}
}
