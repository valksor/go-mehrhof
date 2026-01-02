package agent

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockAgent is a test implementation of Agent interface.
type mockAgent struct {
	available error
	runErr    error
	response  *Response
	name      string
}

func (m *mockAgent) Name() string {
	return m.name
}

func (m *mockAgent) Run(ctx context.Context, prompt string) (*Response, error) {
	if m.runErr != nil {
		return nil, m.runErr
	}
	return m.response, nil
}

func (m *mockAgent) RunStream(ctx context.Context, prompt string) (<-chan Event, <-chan error) {
	events := make(chan Event)
	errs := make(chan error)
	close(events)
	close(errs)
	return events, errs
}

func (m *mockAgent) RunWithCallback(ctx context.Context, prompt string, cb StreamCallback) (*Response, error) {
	return m.response, m.runErr
}

func (m *mockAgent) Available() error {
	return m.available
}

func (m *mockAgent) WithEnv(key, value string) Agent {
	return m
}

func (m *mockAgent) WithArgs(args ...string) Agent {
	return m
}

func TestNewConfig(t *testing.T) {
	cfg := NewConfig()

	if cfg.Timeout != 30*time.Minute {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, 30*time.Minute)
	}
	if cfg.RetryCount != 3 {
		t.Errorf("RetryCount = %d, want 3", cfg.RetryCount)
	}
	if cfg.RetryDelay != time.Second {
		t.Errorf("RetryDelay = %v, want %v", cfg.RetryDelay, time.Second)
	}
}

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()

	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if r.agents == nil {
		t.Error("agents map not initialized")
	}
}

func TestRegistryRegister(t *testing.T) {
	r := NewRegistry()
	agent := &mockAgent{name: "test-agent"}

	err := r.Register(agent)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Verify agent is registered
	got, err := r.Get("test-agent")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Name() != "test-agent" {
		t.Errorf("got agent name = %q, want %q", got.Name(), "test-agent")
	}
}

func TestRegistryRegisterDuplicate(t *testing.T) {
	r := NewRegistry()
	agent1 := &mockAgent{name: "test-agent"}
	agent2 := &mockAgent{name: "test-agent"}

	if err := r.Register(agent1); err != nil {
		t.Fatalf("Register(agent1): %v", err)
	}
	err := r.Register(agent2)

	if err == nil {
		t.Error("Register should fail for duplicate agent")
	}
}

func TestRegistryGet(t *testing.T) {
	r := NewRegistry()
	agent := &mockAgent{name: "test-agent"}
	if err := r.Register(agent); err != nil {
		t.Fatalf("Register(test-agent): %v", err)
	}

	got, err := r.Get("test-agent")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Name() != "test-agent" {
		t.Errorf("got agent name = %q, want %q", got.Name(), "test-agent")
	}
}

func TestRegistryGetNotFound(t *testing.T) {
	r := NewRegistry()

	_, err := r.Get("nonexistent")
	if err == nil {
		t.Error("Get should fail for nonexistent agent")
	}
}

func TestRegistryGetDefault(t *testing.T) {
	r := NewRegistry()
	agent := &mockAgent{name: "first-agent"}
	if err := r.Register(agent); err != nil {
		t.Fatalf("Register(first-agent): %v", err)
	}

	// First registered agent becomes default
	got, err := r.GetDefault()
	if err != nil {
		t.Fatalf("GetDefault failed: %v", err)
	}
	if got.Name() != "first-agent" {
		t.Errorf("default agent = %q, want %q", got.Name(), "first-agent")
	}
}

func TestRegistryGetDefaultEmpty(t *testing.T) {
	r := NewRegistry()

	_, err := r.GetDefault()
	if err == nil {
		t.Error("GetDefault should fail when no agents registered")
	}
}

func TestRegistrySetDefault(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(&mockAgent{name: "agent1"}); err != nil {
		t.Fatalf("Register(agent1): %v", err)
	}
	if err := r.Register(&mockAgent{name: "agent2"}); err != nil {
		t.Fatalf("Register(agent2): %v", err)
	}

	err := r.SetDefault("agent2")
	if err != nil {
		t.Fatalf("SetDefault failed: %v", err)
	}

	got, _ := r.GetDefault()
	if got.Name() != "agent2" {
		t.Errorf("default agent = %q, want %q", got.Name(), "agent2")
	}
}

func TestRegistrySetDefaultNotFound(t *testing.T) {
	r := NewRegistry()

	err := r.SetDefault("nonexistent")
	if err == nil {
		t.Error("SetDefault should fail for nonexistent agent")
	}
}

func TestRegistryList(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(&mockAgent{name: "agent1"}); err != nil {
		t.Fatalf("Register(agent1): %v", err)
	}
	if err := r.Register(&mockAgent{name: "agent2"}); err != nil {
		t.Fatalf("Register(agent2): %v", err)
	}
	if err := r.Register(&mockAgent{name: "agent3"}); err != nil {
		t.Fatalf("Register(agent3): %v", err)
	}

	list := r.List()
	if len(list) != 3 {
		t.Errorf("List returned %d agents, want 3", len(list))
	}

	// Check all agents are in list
	names := make(map[string]bool)
	for _, name := range list {
		names[name] = true
	}
	for _, expected := range []string{"agent1", "agent2", "agent3"} {
		if !names[expected] {
			t.Errorf("List missing agent %q", expected)
		}
	}
}

func TestRegistryAvailable(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(&mockAgent{name: "available1", available: nil}); err != nil {
		t.Fatalf("Register(available1): %v", err)
	}
	if err := r.Register(&mockAgent{name: "unavailable", available: errors.New("not available")}); err != nil {
		t.Fatalf("Register(unavailable): %v", err)
	}
	if err := r.Register(&mockAgent{name: "available2", available: nil}); err != nil {
		t.Fatalf("Register(available2): %v", err)
	}

	available := r.Available()
	if len(available) != 2 {
		t.Errorf("Available returned %d agents, want 2", len(available))
	}

	// Check only available agents are returned
	names := make(map[string]bool)
	for _, name := range available {
		names[name] = true
	}
	if !names["available1"] || !names["available2"] {
		t.Error("Available should return available1 and available2")
	}
	if names["unavailable"] {
		t.Error("Available should not return unavailable agent")
	}
}

func TestRegistryDetect(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(&mockAgent{name: "available", available: nil}); err != nil {
		t.Fatalf("Register(available): %v", err)
	}

	agent, err := r.Detect()
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if agent.Name() != "available" {
		t.Errorf("Detect returned %q, want %q", agent.Name(), "available")
	}
}

func TestRegistryDetectPrefersDefault(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(&mockAgent{name: "agent1", available: nil}); err != nil {
		t.Fatalf("Register(agent1): %v", err)
	}
	if err := r.Register(&mockAgent{name: "agent2", available: nil}); err != nil {
		t.Fatalf("Register(agent2): %v", err)
	}
	if err := r.SetDefault("agent2"); err != nil {
		t.Fatalf("SetDefault(agent2): %v", err)
	}

	agent, err := r.Detect()
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if agent.Name() != "agent2" {
		t.Errorf("Detect returned %q, want %q (default)", agent.Name(), "agent2")
	}
}

func TestRegistryDetectFallsBack(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(&mockAgent{name: "unavailable", available: errors.New("not available")}); err != nil {
		t.Fatalf("Register(unavailable): %v", err)
	}
	if err := r.Register(&mockAgent{name: "available", available: nil}); err != nil {
		t.Fatalf("Register(available): %v", err)
	}

	agent, err := r.Detect()
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if agent.Name() != "available" {
		t.Errorf("Detect returned %q, want %q", agent.Name(), "available")
	}
}

func TestRegistryDetectNoAvailable(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(&mockAgent{name: "unavailable1", available: errors.New("not available")}); err != nil {
		t.Fatalf("Register(unavailable1): %v", err)
	}
	if err := r.Register(&mockAgent{name: "unavailable2", available: errors.New("not available")}); err != nil {
		t.Fatalf("Register(unavailable2): %v", err)
	}

	_, err := r.Detect()
	if err == nil {
		t.Error("Detect should fail when no agents available")
	}
}

func TestEventTypes(t *testing.T) {
	// Test that event type constants are defined
	types := []EventType{
		EventText,
		EventToolUse,
		EventToolResult,
		EventFile,
		EventError,
		EventUsage,
		EventComplete,
	}

	for _, et := range types {
		if et == "" {
			t.Error("EventType constant is empty")
		}
	}
}

func TestFileOpConstants(t *testing.T) {
	// Test that file operation constants are defined
	ops := []FileOp{
		FileOpCreate,
		FileOpUpdate,
		FileOpDelete,
	}

	for _, op := range ops {
		if op == "" {
			t.Error("FileOp constant is empty")
		}
	}
}

func TestEventStruct(t *testing.T) {
	event := Event{
		Type:      EventText,
		Timestamp: time.Now(),
		Data:      map[string]any{"key": "value"},
		Text:      "test text",
	}

	if event.Type != EventText {
		t.Errorf("Event.Type = %q, want %q", event.Type, EventText)
	}
	if event.Text != "test text" {
		t.Errorf("Event.Text = %q, want %q", event.Text, "test text")
	}
}

func TestResponseStruct(t *testing.T) {
	response := Response{
		Files: []FileChange{
			{Path: "test.go", Operation: FileOpCreate},
		},
		Summary:  "Test summary",
		Messages: []string{"msg1", "msg2"},
		Usage: &UsageStats{
			InputTokens:  100,
			OutputTokens: 50,
		},
		Duration: 5 * time.Second,
	}

	if len(response.Files) != 1 {
		t.Errorf("Response.Files length = %d, want 1", len(response.Files))
	}
	if response.Summary != "Test summary" {
		t.Errorf("Response.Summary = %q, want %q", response.Summary, "Test summary")
	}
	if response.Usage.InputTokens != 100 {
		t.Errorf("Response.Usage.InputTokens = %d, want 100", response.Usage.InputTokens)
	}
}

func TestFileChangeStruct(t *testing.T) {
	fc := FileChange{
		Path:      "path/to/file.go",
		Operation: FileOpUpdate,
		Content:   "new content",
	}

	if fc.Path != "path/to/file.go" {
		t.Errorf("FileChange.Path = %q, want %q", fc.Path, "path/to/file.go")
	}
	if fc.Operation != FileOpUpdate {
		t.Errorf("FileChange.Operation = %q, want %q", fc.Operation, FileOpUpdate)
	}
}

func TestUsageStatsStruct(t *testing.T) {
	usage := UsageStats{
		InputTokens:  1000,
		OutputTokens: 500,
		CachedTokens: 200,
		CostUSD:      0.05,
	}

	if usage.InputTokens != 1000 {
		t.Errorf("UsageStats.InputTokens = %d, want 1000", usage.InputTokens)
	}
	if usage.CostUSD != 0.05 {
		t.Errorf("UsageStats.CostUSD = %f, want 0.05", usage.CostUSD)
	}
}

func TestToolCallStruct(t *testing.T) {
	tc := ToolCall{
		Name:        "Read",
		Description: "Read a file",
		Input:       map[string]any{"path": "test.go"},
	}

	if tc.Name != "Read" {
		t.Errorf("ToolCall.Name = %q, want %q", tc.Name, "Read")
	}
	if tc.Input["path"] != "test.go" {
		t.Errorf("ToolCall.Input[path] = %v, want %q", tc.Input["path"], "test.go")
	}
}

func TestQuestionStruct(t *testing.T) {
	q := Question{
		Text: "What should I do?",
		Options: []QuestionOption{
			{Label: "A", Description: "Option A"},
			{Label: "B", Description: "Option B"},
		},
	}

	if q.Text != "What should I do?" {
		t.Errorf("Question.Text = %q, want %q", q.Text, "What should I do?")
	}
	if len(q.Options) != 2 {
		t.Errorf("Question.Options length = %d, want 2", len(q.Options))
	}
}

func TestConfigStruct(t *testing.T) {
	cfg := Config{
		Command:     []string{"claude", "--model", "sonnet"},
		Environment: map[string]string{"API_KEY": "test"},
		Timeout:     10 * time.Minute,
		RetryCount:  5,
		RetryDelay:  2 * time.Second,
		WorkDir:     "/tmp/work",
	}

	if len(cfg.Command) != 3 {
		t.Errorf("Config.Command length = %d, want 3", len(cfg.Command))
	}
	if cfg.Timeout != 10*time.Minute {
		t.Errorf("Config.Timeout = %v, want %v", cfg.Timeout, 10*time.Minute)
	}
	if cfg.WorkDir != "/tmp/work" {
		t.Errorf("Config.WorkDir = %q, want %q", cfg.WorkDir, "/tmp/work")
	}
}
