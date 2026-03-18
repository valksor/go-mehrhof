// Package strategy provides pluggable agent reasoning patterns.
// Strategies define how prompts are constructed and outputs evaluated,
// allowing different reasoning approaches per phase or task type.
package strategy

import (
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
