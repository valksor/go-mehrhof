// Package testutil provides shared testing utilities for go-mehrhof tests.
package testutil

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/valksor/go-mehrhof/internal/agent"
)

// ──────────────────────────────────────────────────────────────────────────────
// MockAgent implements agent.Agent for testing
// ──────────────────────────────────────────────────────────────────────────────

// MockAgent is a configurable mock implementation of agent.Agent.
type MockAgent struct {
	NameVal      string
	AvailableErr error
	Response     *agent.Response
	RunErr       error
	EnvVars      map[string]string
	Args         []string

	// Callbacks for verifying calls
	RunCalled     bool
	RunPrompt     string
	RunCtx        context.Context
	StreamEvents  []agent.Event
	StreamErr     error
	CallbackCalls []agent.Event
}

// NewMockAgent creates a new MockAgent with default values.
func NewMockAgent(name string) *MockAgent {
	return &MockAgent{
		NameVal: name,
		EnvVars: make(map[string]string),
	}
}

// Name returns the agent's identifier.
func (m *MockAgent) Name() string {
	return m.NameVal
}

// Run executes a prompt and returns the configured response.
func (m *MockAgent) Run(ctx context.Context, prompt string) (*agent.Response, error) {
	m.RunCalled = true
	m.RunPrompt = prompt
	m.RunCtx = ctx
	if m.RunErr != nil {
		return nil, m.RunErr
	}
	return m.Response, nil
}

// RunStream executes a prompt and streams configured events.
func (m *MockAgent) RunStream(ctx context.Context, prompt string) (<-chan agent.Event, <-chan error) {
	m.RunCalled = true
	m.RunPrompt = prompt
	m.RunCtx = ctx

	eventCh := make(chan agent.Event, len(m.StreamEvents))
	errCh := make(chan error, 1)

	go func() {
		defer close(eventCh)
		defer close(errCh)

		for _, event := range m.StreamEvents {
			select {
			case eventCh <- event:
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			}
		}

		if m.StreamErr != nil {
			errCh <- m.StreamErr
		}
	}()

	return eventCh, errCh
}

// RunWithCallback executes with a callback for each event.
func (m *MockAgent) RunWithCallback(ctx context.Context, prompt string, cb agent.StreamCallback) (*agent.Response, error) {
	m.RunCalled = true
	m.RunPrompt = prompt
	m.RunCtx = ctx

	for _, event := range m.StreamEvents {
		m.CallbackCalls = append(m.CallbackCalls, event)
		if err := cb(event); err != nil {
			return nil, err
		}
	}

	if m.RunErr != nil {
		return nil, m.RunErr
	}
	return m.Response, nil
}

// Available checks if the agent is available.
func (m *MockAgent) Available() error {
	return m.AvailableErr
}

// WithEnv adds an environment variable. Returns a new MockAgent.
func (m *MockAgent) WithEnv(key, value string) agent.Agent {
	newMock := &MockAgent{
		NameVal:      m.NameVal,
		AvailableErr: m.AvailableErr,
		Response:     m.Response,
		RunErr:       m.RunErr,
		EnvVars:      make(map[string]string),
		Args:         m.Args,
		StreamEvents: m.StreamEvents,
		StreamErr:    m.StreamErr,
	}
	for k, v := range m.EnvVars {
		newMock.EnvVars[k] = v
	}
	newMock.EnvVars[key] = value
	return newMock
}

// WithArgs adds CLI arguments. Returns a new MockAgent.
func (m *MockAgent) WithArgs(args ...string) agent.Agent {
	newArgs := make([]string, len(m.Args), len(m.Args)+len(args))
	copy(newArgs, m.Args)
	newArgs = append(newArgs, args...)

	return &MockAgent{
		NameVal:      m.NameVal,
		AvailableErr: m.AvailableErr,
		Response:     m.Response,
		RunErr:       m.RunErr,
		EnvVars:      m.EnvVars,
		Args:         newArgs,
		StreamEvents: m.StreamEvents,
		StreamErr:    m.StreamErr,
	}
}

// WithResponse configures the response to return.
func (m *MockAgent) WithResponse(resp *agent.Response) *MockAgent {
	m.Response = resp
	return m
}

// WithError configures the error to return.
func (m *MockAgent) WithError(err error) *MockAgent {
	m.RunErr = err
	return m
}

// WithAvailableError configures the availability error.
func (m *MockAgent) WithAvailableError(err error) *MockAgent {
	m.AvailableErr = err
	return m
}

// WithStreamEvents configures events to stream.
func (m *MockAgent) WithStreamEvents(events []agent.Event) *MockAgent {
	m.StreamEvents = events
	return m
}

// ──────────────────────────────────────────────────────────────────────────────
// MockProcess implements plugin.Process-like interface for testing
// ──────────────────────────────────────────────────────────────────────────────

// MockProcess simulates a plugin Process for testing adapters.
type MockProcess struct {
	mu sync.Mutex

	// Call responses keyed by method name
	CallResponses map[string]json.RawMessage
	CallErrors    map[string]error

	// For streaming
	StreamEvents []json.RawMessage
	StreamErr    error

	// Call tracking
	Calls []MockProcessCall

	// State
	Running  bool
	StopErr  error
	Manifest MockManifest
}

// MockProcessCall records a call to the mock process.
type MockProcessCall struct {
	Method string
	Params any
}

// MockManifest represents a minimal manifest for testing.
type MockManifest struct {
	Name string
	Type string
}

// NewMockProcess creates a new MockProcess.
func NewMockProcess() *MockProcess {
	return &MockProcess{
		CallResponses: make(map[string]json.RawMessage),
		CallErrors:    make(map[string]error),
		Running:       true,
	}
}

// Call simulates an RPC call to the plugin.
func (m *MockProcess) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Calls = append(m.Calls, MockProcessCall{Method: method, Params: params})

	if err, ok := m.CallErrors[method]; ok && err != nil {
		return nil, err
	}

	if resp, ok := m.CallResponses[method]; ok {
		return resp, nil
	}

	// Return empty JSON object by default
	return json.RawMessage(`{}`), nil
}

// Stream simulates streaming from the plugin.
func (m *MockProcess) Stream(ctx context.Context, method string, params any) (<-chan json.RawMessage, error) {
	m.mu.Lock()
	m.Calls = append(m.Calls, MockProcessCall{Method: method, Params: params})
	events := m.StreamEvents
	streamErr := m.StreamErr
	m.mu.Unlock()

	ch := make(chan json.RawMessage, len(events))

	go func() {
		defer close(ch)

		for _, event := range events {
			select {
			case ch <- event:
			case <-ctx.Done():
				return
			}
		}

		// streamErr is captured for future error handling if needed
		_ = streamErr
	}()

	return ch, nil
}

// Stop simulates stopping the plugin process.
func (m *MockProcess) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Running = false
	return m.StopErr
}

// IsRunning returns whether the mock process is running.
func (m *MockProcess) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Running
}

// SetCallResponse configures a response for a method.
func (m *MockProcess) SetCallResponse(method string, response any) error {
	data, err := json.Marshal(response)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CallResponses[method] = data
	return nil
}

// SetCallError configures an error for a method.
func (m *MockProcess) SetCallError(method string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CallErrors[method] = err
}

// SetStreamEvents configures events to stream.
func (m *MockProcess) SetStreamEvents(events []any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.StreamEvents = make([]json.RawMessage, 0, len(events))
	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			return err
		}
		m.StreamEvents = append(m.StreamEvents, data)
	}
	return nil
}

// GetCalls returns all recorded calls.
func (m *MockProcess) GetCalls() []MockProcessCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]MockProcessCall, len(m.Calls))
	copy(result, m.Calls)
	return result
}

// ClearCalls clears recorded calls.
func (m *MockProcess) ClearCalls() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = nil
}

// ──────────────────────────────────────────────────────────────────────────────
// MockProvider simulates a provider for conductor testing
// ──────────────────────────────────────────────────────────────────────────────

// MockProvider simulates a task provider for testing.
type MockProvider struct {
	ParseErr       error
	FetchErr       error
	SnapshotErr    error
	FetchResult    *MockWorkUnit
	SnapshotResult *MockSnapshot
	ParseResult    string
	MatchCalls     []string
	ParseCalls     []string
	FetchCalls     []string
	MatchResult    bool
}

// MockWorkUnit represents a work unit for testing.
type MockWorkUnit struct {
	ID          string
	Title       string
	Description string
	ExternalKey string
	Type        string
	Status      string
}

// MockSnapshot represents a provider snapshot for testing.
type MockSnapshot struct {
	Files map[string]string
}

// NewMockProvider creates a new MockProvider.
func NewMockProvider() *MockProvider {
	return &MockProvider{
		MatchResult: true,
	}
}

// Match checks if a reference matches this provider.
func (m *MockProvider) Match(ref string) bool {
	m.MatchCalls = append(m.MatchCalls, ref)
	return m.MatchResult
}

// Parse parses a reference string.
func (m *MockProvider) Parse(ref string) (string, error) {
	m.ParseCalls = append(m.ParseCalls, ref)
	if m.ParseErr != nil {
		return "", m.ParseErr
	}
	return m.ParseResult, nil
}

// Fetch retrieves a work unit.
func (m *MockProvider) Fetch(ctx context.Context, ref string) (*MockWorkUnit, error) {
	m.FetchCalls = append(m.FetchCalls, ref)
	if m.FetchErr != nil {
		return nil, m.FetchErr
	}
	return m.FetchResult, nil
}

// WithFetchResult configures the fetch result.
func (m *MockProvider) WithFetchResult(wu *MockWorkUnit) *MockProvider {
	m.FetchResult = wu
	return m
}

// WithFetchError configures a fetch error.
func (m *MockProvider) WithFetchError(err error) *MockProvider {
	m.FetchErr = err
	return m
}
