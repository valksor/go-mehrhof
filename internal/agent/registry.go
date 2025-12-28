package agent

import (
	"fmt"
	"sync"
)

// Registry manages available agents
type Registry struct {
	mu       sync.RWMutex
	agents   map[string]Agent
	fallback string
}

// NewRegistry creates an agent registry
func NewRegistry() *Registry {
	return &Registry{
		agents: make(map[string]Agent),
	}
}

// Register adds an agent to the registry
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

// Get returns an agent by name
func (r *Registry) Get(name string) (Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agent, ok := r.agents[name]
	if !ok {
		return nil, fmt.Errorf("agent not found: %s", name)
	}

	return agent, nil
}

// GetDefault returns the default/fallback agent
func (r *Registry) GetDefault() (Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.fallback == "" {
		return nil, fmt.Errorf("no agents registered")
	}

	return r.agents[r.fallback], nil
}

// SetDefault sets the default agent
func (r *Registry) SetDefault(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.agents[name]; !ok {
		return fmt.Errorf("agent not found: %s", name)
	}

	r.fallback = name
	return nil
}

// List returns all registered agent names
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.agents))
	for name := range r.agents {
		names = append(names, name)
	}
	return names
}

// Available returns agents that pass availability check
func (r *Registry) Available() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var available []string
	for name, agent := range r.agents {
		if err := agent.Available(); err == nil {
			available = append(available, name)
		}
	}
	return available
}

// Detect returns the first available agent
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

	return nil, fmt.Errorf("no available agents found")
}
