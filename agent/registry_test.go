package agent

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockAgent is a test implementation of Agent.
type mockAgent struct {
	name      string
	available bool
	connected bool
}

func (m *mockAgent) Name() string { return m.name }
func (m *mockAgent) Available() error {
	if !m.available {
		return errors.New("not available")
	}

	return nil
}

func (m *mockAgent) Connect(ctx context.Context) error {
	m.connected = true

	return nil
}
func (m *mockAgent) Connected() bool { return m.connected }
func (m *mockAgent) SendPrompt(ctx context.Context, prompt string) (<-chan Event, error) {
	ch := make(chan Event, 1)
	ch <- Event{Type: EventComplete}
	close(ch)

	return ch, nil
}
func (m *mockAgent) HandlePermission(requestID string, approved bool) error { return nil }
func (m *mockAgent) Close() error {
	m.connected = false

	return nil
}
func (m *mockAgent) Interrupt() error                  { return nil }
func (m *mockAgent) WithEnv(key, value string) Agent   { return m }
func (m *mockAgent) WithArgs(args ...string) Agent     { return m }
func (m *mockAgent) WithWorkDir(dir string) Agent      { return m }
func (m *mockAgent) WithTimeout(d time.Duration) Agent { return m }

func TestRegistryRegister(t *testing.T) {
	r := NewRegistry()

	agent1 := &mockAgent{name: "agent1", available: true}
	agent2 := &mockAgent{name: "agent2", available: true}

	// First registration should succeed
	if err := r.Register(agent1); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Second registration with same name should fail
	dup := &mockAgent{name: "agent1", available: true}
	if err := r.Register(dup); err == nil {
		t.Error("Expected error for duplicate registration")
	}

	// Different name should succeed
	if err := r.Register(agent2); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if r.Count() != 2 {
		t.Errorf("Expected 2 agents, got %d", r.Count())
	}
}

func TestRegistryGet(t *testing.T) {
	r := NewRegistry()
	agent := &mockAgent{name: "test", available: true}
	_ = r.Register(agent)

	// Get existing agent
	got, err := r.Get("test")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Name() != "test" {
		t.Errorf("Expected name 'test', got %q", got.Name())
	}

	// Get non-existent agent
	_, err = r.Get("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent agent")
	}
}

func TestRegistryDefault(t *testing.T) {
	r := NewRegistry()

	// No agents registered
	_, err := r.GetDefault()
	if err == nil {
		t.Error("Expected error when no agents registered")
	}

	// Register first agent - becomes default
	agent1 := &mockAgent{name: "first", available: true}
	_ = r.Register(agent1)

	def, err := r.GetDefault()
	if err != nil {
		t.Fatalf("GetDefault failed: %v", err)
	}
	if def.Name() != "first" {
		t.Errorf("Expected default 'first', got %q", def.Name())
	}

	// Register second, first still default
	agent2 := &mockAgent{name: "second", available: true}
	_ = r.Register(agent2)

	def, _ = r.GetDefault()
	if def.Name() != "first" {
		t.Errorf("Expected default still 'first', got %q", def.Name())
	}

	// Change default
	_ = r.SetDefault("second")
	def, _ = r.GetDefault()
	if def.Name() != "second" {
		t.Errorf("Expected default 'second', got %q", def.Name())
	}

	// Set non-existent as default
	if err := r.SetDefault("nonexistent"); err == nil {
		t.Error("Expected error setting non-existent default")
	}
}

func TestRegistryList(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&mockAgent{name: "zebra", available: true})
	_ = r.Register(&mockAgent{name: "alpha", available: true})
	_ = r.Register(&mockAgent{name: "beta", available: true})

	list := r.List()
	if len(list) != 3 {
		t.Fatalf("Expected 3 agents, got %d", len(list))
	}

	// Should be sorted
	expected := []string{"alpha", "beta", "zebra"}
	for i, name := range expected {
		if list[i] != name {
			t.Errorf("Expected %q at index %d, got %q", name, i, list[i])
		}
	}
}

func TestRegistryAvailable(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&mockAgent{name: "available1", available: true})
	_ = r.Register(&mockAgent{name: "unavailable", available: false})
	_ = r.Register(&mockAgent{name: "available2", available: true})

	avail := r.Available()
	if len(avail) != 2 {
		t.Fatalf("Expected 2 available agents, got %d", len(avail))
	}

	// Should only contain available agents
	for _, name := range avail {
		if name == "unavailable" {
			t.Error("Unavailable agent should not be in Available() list")
		}
	}
}

func TestRegistryDetect(t *testing.T) {
	r := NewRegistry()

	// No agents
	_, err := r.Detect()
	if err == nil {
		t.Error("Expected error when no agents available")
	}

	// Only unavailable agent
	_ = r.Register(&mockAgent{name: "unavailable", available: false})
	_, err = r.Detect()
	if err == nil {
		t.Error("Expected error when no agents available")
	}

	// Add available agent
	_ = r.Register(&mockAgent{name: "available", available: true})
	agent, err := r.Detect()
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if agent.Name() != "available" {
		t.Errorf("Expected 'available', got %q", agent.Name())
	}
}

func TestRegistryUnregister(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&mockAgent{name: "agent1", available: true})
	_ = r.Register(&mockAgent{name: "agent2", available: true})

	if err := r.Unregister("agent1"); err != nil {
		t.Fatalf("Unregister failed: %v", err)
	}

	if r.Count() != 1 {
		t.Errorf("Expected 1 agent after unregister, got %d", r.Count())
	}

	// Unregister non-existent
	if err := r.Unregister("nonexistent"); err == nil {
		t.Error("Expected error unregistering non-existent agent")
	}
}

func TestDefaultPermissionHandler(t *testing.T) {
	tests := []struct {
		tool     string
		expected bool
	}{
		{"Read", true},
		{"read_file", true},
		{"Glob", true},
		{"glob", true},
		{"Grep", true},
		{"grep", true},
		{"Write", false},
		{"Bash", false},
		{"Execute", false},
	}

	for _, tt := range tests {
		req := PermissionRequest{Tool: tt.tool}
		result := DefaultPermissionHandler(req)
		if result != tt.expected {
			t.Errorf("DefaultPermissionHandler(%q) = %v, want %v", tt.tool, result, tt.expected)
		}
	}
}
