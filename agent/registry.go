package agent

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
)

// Registry manages available agents.
type Registry struct {
	agents   map[string]Agent
	fallback string
	mu       sync.RWMutex
}

// NewRegistry creates an empty agent registry.
func NewRegistry() *Registry {
	return &Registry{
		agents: make(map[string]Agent),
	}
}

// Register adds an agent to the registry.
// Returns error if an agent with the same name is already registered.
func (r *Registry) Register(agent Agent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := agent.Name()
	if _, exists := r.agents[name]; exists {
		return fmt.Errorf("agent already registered: %s", name)
	}

	r.agents[name] = agent

	// First registered agent becomes fallback
	if r.fallback == "" {
		r.fallback = name
	}

	return nil
}

// MustRegister registers an agent and panics on error.
func (r *Registry) MustRegister(agent Agent) {
	if err := r.Register(agent); err != nil {
		panic(err)
	}
}

// Get returns an agent by name.
func (r *Registry) Get(name string) (Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agent, ok := r.agents[name]
	if !ok {
		return nil, fmt.Errorf("agent not found: %s", name)
	}

	return agent, nil
}

// GetDefault returns the default/fallback agent.
func (r *Registry) GetDefault() (Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.fallback == "" {
		return nil, errors.New("no agents registered")
	}

	return r.agents[r.fallback], nil
}

// SetDefault sets the default agent by name.
func (r *Registry) SetDefault(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.agents[name]; !ok {
		return fmt.Errorf("agent not found: %s", name)
	}

	r.fallback = name

	return nil
}

// List returns all registered agent names (sorted).
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return slices.Sorted(maps.Keys(r.agents))
}

// Available returns names of agents that pass the Available() check.
func (r *Registry) Available() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var available []string
	for name, agent := range r.agents {
		if agent.Available() == nil {
			available = append(available, name)
		}
	}
	slices.Sort(available)

	return available
}

// Detect returns the first available agent.
// Tries the default agent first, then others.
func (r *Registry) Detect() (Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Try default first
	if r.fallback != "" {
		if agent := r.agents[r.fallback]; agent != nil {
			if err := agent.Available(); err == nil {
				return agent, nil
			}
		}
	}

	// Try others
	for _, agent := range r.agents {
		if err := agent.Available(); err == nil {
			return agent, nil
		}
	}

	return nil, errors.New("no available agents found")
}

// GetOrDetect tries to get a specific agent, falls back to auto-detection.
func (r *Registry) GetOrDetect(name string) (Agent, error) {
	if name != "" {
		return r.Get(name)
	}

	return r.Detect()
}

// Count returns the number of registered agents.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.agents)
}

// Unregister removes an agent from the registry.
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.agents[name]; !ok {
		return fmt.Errorf("agent not found: %s", name)
	}

	delete(r.agents, name)

	// Update fallback if we removed it
	if r.fallback == name {
		r.fallback = ""
		for n := range r.agents {
			r.fallback = n

			break
		}
	}

	return nil
}

// Clear removes all agents from the registry.
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.agents = make(map[string]Agent)
	r.fallback = ""
}
