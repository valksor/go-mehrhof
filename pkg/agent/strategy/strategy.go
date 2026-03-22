// Package strategy provides pluggable agent reasoning patterns.
// Strategies define how prompts are constructed and outputs evaluated,
// allowing different reasoning approaches per phase or task type.
package strategy

import (
	"context"
	"slices"
	"sync"
)

// Input provides context for prompt construction.
type Input struct {
	Task        string
	Context     string            // Accumulated context (specs, prior output)
	Variables   map[string]string // From variable pool
	Phase       string            // Current phase name
	Constraints []string          // Quality requirements, style guides
}

// Output captures the evaluated result of agent execution.
type Output struct {
	Content  string
	Metadata map[string]string
	Status   string // "complete", "needs_iteration", "blocked"
}

// Strategy defines how an agent reasons about a task.
type Strategy interface {
	Name() string
	BuildPrompt(input Input) string
	EvaluateOutput(output string) Output
}

// Runner is an optional interface that strategies can implement to control
// the full agent execution loop. When a strategy implements Runner, the
// conductor delegates execution to RunLoop instead of the normal single-call path.
type Runner interface {
	// RunLoop executes the agent in a controlled loop.
	// exec sends a prompt and returns the agent's response.
	// emit forwards events to the conductor's event stream.
	// Returns the final combined output.
	RunLoop(ctx context.Context, exec ExecFunc, input Input, emit func(Event)) (Output, error)
}

// ExecFunc sends a prompt to an agent and returns its response.
// This abstracts the actual agent call so strategies don't depend on agent internals.
type ExecFunc func(ctx context.Context, prompt string) (string, error)

// Event is a lightweight event emitted during strategy execution.
type Event struct {
	Type    string // "pass_started", "pass_completed", "reflection"
	Message string
	Pass    int
}

// IsRunner returns true if the strategy implements the Runner interface.
func IsRunner(s Strategy) bool {
	_, ok := s.(Runner)

	return ok
}

var (
	mu         sync.RWMutex
	strategies = make(map[string]Strategy)
)

func init() {
	Register(&Direct{})
	Register(&Iterative{})
}

// Register adds a strategy to the registry.
func Register(s Strategy) {
	mu.Lock()
	defer mu.Unlock()

	strategies[s.Name()] = s
}

// Get returns a strategy by name.
func Get(name string) (Strategy, bool) {
	mu.RLock()
	defer mu.RUnlock()

	s, ok := strategies[name]

	return s, ok
}

// Default returns the "direct" strategy.
func Default() Strategy {
	s, _ := Get("direct")

	return s
}

// List returns sorted names of all registered strategies.
func List() []string {
	mu.RLock()
	defer mu.RUnlock()

	names := make([]string, 0, len(strategies))
	for name := range strategies {
		names = append(names, name)
	}

	slices.Sort(names)

	return names
}
