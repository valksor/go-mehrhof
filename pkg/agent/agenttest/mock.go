// Package agenttest provides test utilities for agent consumers.
package agenttest

import (
	"context"
	"maps"
	"sync"
	"time"

	"github.com/valksor/kvelmo/pkg/agent"
)

// PermissionCall records a HandlePermission invocation.
type PermissionCall struct {
	RequestID string
	Approved  bool
}

// MockAgent is a configurable mock that implements agent.Agent for testing.
// It emits a preconfigured sequence of events on SendPrompt and tracks all calls.
type MockAgent struct {
	mu sync.Mutex

	name      string
	events    []agent.Event
	available error
	connected bool

	// Tracking (exported for assertions)
	Prompts      []string
	Permissions  []PermissionCall
	ConnectCalls int
	CloseCalls   int

	// Builder state
	env     map[string]string
	args    []string
	workDir string
	timeout time.Duration
}

// NewMockAgent creates a mock agent that emits the given events on each SendPrompt call.
// If no events are provided, it emits a single EventComplete.
func NewMockAgent(name string, events ...agent.Event) *MockAgent {
	return &MockAgent{
		name:   name,
		events: events,
		env:    make(map[string]string),
	}
}

// WithAvailableError makes Available() return the given error.
func (m *MockAgent) WithAvailableError(err error) *MockAgent {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.available = err

	return m
}

// Name returns the agent identifier.
func (m *MockAgent) Name() string {
	return m.name
}

// Available returns the configured error (nil by default).
func (m *MockAgent) Available() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.available
}

// Connect marks the agent as connected and records the call.
func (m *MockAgent) Connect(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = true
	m.ConnectCalls++

	return nil
}

// Connected returns the connection state.
func (m *MockAgent) Connected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.connected
}

// SendPrompt returns a channel that emits the preconfigured events.
// Appends an EventComplete if the event sequence doesn't end with one.
func (m *MockAgent) SendPrompt(_ context.Context, prompt string) (<-chan agent.Event, error) {
	m.mu.Lock()
	m.Prompts = append(m.Prompts, prompt)
	events := make([]agent.Event, len(m.events))
	copy(events, m.events)
	m.mu.Unlock()

	ch := make(chan agent.Event, len(events)+1)
	go func() {
		defer close(ch)
		hasTerminal := false
		for _, e := range events {
			if e.Timestamp.IsZero() {
				e.Timestamp = time.Now()
			}
			ch <- e
			if e.Type == agent.EventComplete || e.Type == agent.EventError {
				hasTerminal = true

				break
			}
		}
		if !hasTerminal {
			ch <- agent.Event{
				Type:      agent.EventComplete,
				Timestamp: time.Now(),
			}
		}
	}()

	return ch, nil
}

// HandlePermission records the permission call.
func (m *MockAgent) HandlePermission(requestID string, approved bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Permissions = append(m.Permissions, PermissionCall{
		RequestID: requestID,
		Approved:  approved,
	})

	return nil
}

// Close marks the agent as disconnected and records the call.
func (m *MockAgent) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = false
	m.CloseCalls++

	return nil
}

// Interrupt interrupts the current agent turn (no-op for mock).
func (m *MockAgent) Interrupt() error {
	return nil
}

// WithEnv returns a new MockAgent with the environment variable added.
func (m *MockAgent) WithEnv(key, value string) agent.Agent {
	m.mu.Lock()
	defer m.mu.Unlock()

	newMock := m.clone()
	newMock.env[key] = value

	return newMock
}

// WithArgs returns a new MockAgent with the arguments appended.
func (m *MockAgent) WithArgs(args ...string) agent.Agent {
	m.mu.Lock()
	defer m.mu.Unlock()

	newMock := m.clone()
	newMock.args = append(newMock.args, args...)

	return newMock
}

// WithWorkDir returns a new MockAgent with the working directory set.
func (m *MockAgent) WithWorkDir(dir string) agent.Agent {
	m.mu.Lock()
	defer m.mu.Unlock()

	newMock := m.clone()
	newMock.workDir = dir

	return newMock
}

// WithTimeout returns a new MockAgent with the timeout set.
func (m *MockAgent) WithTimeout(d time.Duration) agent.Agent {
	m.mu.Lock()
	defer m.mu.Unlock()

	newMock := m.clone()
	newMock.timeout = d

	return newMock
}

// clone creates a deep copy of the mock (must be called under lock).
func (m *MockAgent) clone() *MockAgent {
	events := make([]agent.Event, len(m.events))
	copy(events, m.events)

	env := make(map[string]string, len(m.env))
	maps.Copy(env, m.env)

	args := make([]string, len(m.args))
	copy(args, m.args)

	return &MockAgent{
		name:      m.name,
		events:    events,
		available: m.available,
		env:       env,
		args:      args,
		workDir:   m.workDir,
		timeout:   m.timeout,
	}
}

// Ensure MockAgent implements agent.Agent at compile time.
var _ agent.Agent = (*MockAgent)(nil)
