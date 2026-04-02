package worker

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/valksor/kvelmo/pkg/agent"
)

// AddWorker adds a simulated worker to the pool (for testing).
func (p *Pool) AddWorker() *Worker {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.workers) >= p.maxWorkers {
		return nil
	}

	w := &Worker{
		ID:        "w-" + uuid.New().String()[:6],
		Status:    StatusAvailable,
		StartedAt: time.Now(),
	}
	p.workers[w.ID] = w

	return w
}

// AddWorkerWithAgent adds a worker with specified agent name (without connecting).
func (p *Pool) AddWorkerWithAgent(agentName string) *Worker {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.workers) >= p.maxWorkers {
		return nil
	}

	w := &Worker{
		ID:        "w-" + uuid.New().String()[:6],
		AgentName: agentName,
		Status:    StatusAvailable,
		StartedAt: time.Now(),
	}
	p.workers[w.ID] = w

	return w
}

// AddDefaultWorker adds the default worker (cannot be removed).
func (p *Pool) AddDefaultWorker(agentName string) *Worker {
	p.mu.Lock()
	defer p.mu.Unlock()

	w := &Worker{
		ID:        "default",
		AgentName: agentName,
		Status:    StatusAvailable,
		StartedAt: time.Now(),
		IsDefault: true,
	}
	p.workers[w.ID] = w

	return w
}

// AddAgentWorker creates a worker backed by an agent from the registry.
// If agentName is empty, auto-detects the first available agent.
// If isDefault is true, the worker cannot be removed.
func (p *Pool) AddAgentWorker(ctx context.Context, agentName string, isDefault bool) (*Worker, error) {
	p.mu.Lock()
	if len(p.workers) >= p.maxWorkers {
		p.mu.Unlock()

		return nil, fmt.Errorf("max workers (%d) reached", p.maxWorkers)
	}
	p.mu.Unlock()

	// Validate against allowed agents whitelist
	if agentName != "" && len(p.allowedAgents) > 0 {
		if !slices.Contains(p.allowedAgents, agentName) {
			return nil, fmt.Errorf("agent %q is not in the allowed list (allowed: %v)", agentName, p.allowedAgents)
		}
	}

	// Get agent from registry
	var ag agent.Agent
	var err error
	if agentName != "" {
		ag, err = p.agents.Get(agentName)
		if err != nil {
			return nil, fmt.Errorf("get agent %q: %w", agentName, err)
		}
	} else {
		ag, err = p.agents.Detect()
		if err != nil {
			return nil, fmt.Errorf("detect agent: %w", err)
		}
	}

	// Connect agent
	if err := ag.Connect(ctx); err != nil {
		return nil, fmt.Errorf("connect agent %q: %w", ag.Name(), err)
	}

	id := "ag-" + uuid.New().String()[:6]
	if isDefault {
		id = "default"
	}
	w := &Worker{
		ID:        id,
		Status:    StatusAvailable,
		StartedAt: time.Now(),
		AgentName: ag.Name(),
		Agent:     ag,
		IsDefault: isDefault,
	}

	p.mu.Lock()
	p.workers[w.ID] = w
	p.mu.Unlock()

	return w, nil
}

// Agents returns the agent registry.
func (p *Pool) Agents() *agent.Registry {
	return p.agents
}

// RemoveWorker removes a worker from the pool.
// Returns error if worker is default or not found.
func (p *Pool) RemoveWorker(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	w, ok := p.workers[id]
	if !ok {
		return fmt.Errorf("worker %s not found", id)
	}
	if w.IsDefault {
		return errors.New("cannot remove default worker")
	}
	if w.Agent != nil {
		_ = w.Agent.Close()
	}
	delete(p.workers, id)

	return nil
}

// ListWorkers returns all workers.
// Uses full Lock because it may update worker status from agent connection state.
func (p *Pool) ListWorkers() []*Worker {
	p.mu.Lock()
	defer p.mu.Unlock()

	workers := make([]*Worker, 0, len(p.workers))

	for _, w := range p.workers {
		// Update status from agent if connected
		if w.Agent != nil {
			if w.Agent.Connected() && w.Status == StatusDisconnected {
				w.Status = StatusAvailable
			} else if !w.Agent.Connected() && w.Status != StatusDisconnected {
				w.Status = StatusDisconnected
			}
		}
		workers = append(workers, w)
	}

	// Sort by ID for consistent ordering
	slices.SortFunc(workers, func(a, b *Worker) int {
		return cmp.Compare(a.ID, b.ID)
	})

	return workers
}
