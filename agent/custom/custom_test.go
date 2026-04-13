package custom_test

import (
	"context"
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent"
	"github.com/valksor/kvelmo/agent/custom"
)

func TestDefaultConfig(t *testing.T) {
	cfg := custom.DefaultConfig("myagent", []string{"echo"})
	if cfg.Name != "myagent" {
		t.Errorf("DefaultConfig().Name = %q, want %q", cfg.Name, "myagent")
	}
	if cfg.InputFormat != "json" {
		t.Errorf("DefaultConfig().InputFormat = %q, want json", cfg.InputFormat)
	}
	if cfg.OutputFormat != "ndjson" {
		t.Errorf("DefaultConfig().OutputFormat = %q, want ndjson", cfg.OutputFormat)
	}
	if cfg.Timeout == 0 {
		t.Error("DefaultConfig().Timeout should not be zero")
	}
}

func TestNew(t *testing.T) {
	a := custom.New("test", []string{"echo"})
	if a == nil {
		t.Fatal("New() returned nil")
	}
}

func TestNewWithConfig_Defaults(t *testing.T) {
	// Empty name and command should get defaults
	cfg := custom.Config{}
	a := custom.NewWithConfig(cfg)
	if a == nil {
		t.Fatal("NewWithConfig() returned nil")
	}
}

func TestName(t *testing.T) {
	a := custom.New("myagent", []string{"echo"})
	if a.Name() != "myagent" {
		t.Errorf("Name() = %q, want myagent", a.Name())
	}
}

func TestConnected_NewAgent(t *testing.T) {
	a := custom.New("test", []string{"echo"})
	if a.Connected() {
		t.Error("Connected() should return false for a new agent")
	}
}

func TestAvailable_EchoExists(t *testing.T) {
	a := custom.New("test", []string{"echo"})
	// echo is always available on Unix
	if err := a.Available(); err != nil {
		t.Errorf("Available() with 'echo' command returned unexpected error: %v", err)
	}
}

func TestAvailable_BinaryNotFound(t *testing.T) {
	a := custom.New("test", []string{"nonexistent-binary-kvelmo-test-xyz"})
	err := a.Available()
	if err == nil {
		t.Error("Available() with nonexistent command should return an error")
	}
}

func TestWithEnv(t *testing.T) {
	a := custom.New("test", []string{"echo"})
	b := a.WithEnv("MY_KEY", "my_value")
	if b == nil {
		t.Fatal("WithEnv() returned nil")
	}
	if b == a {
		t.Error("WithEnv() should return a new agent")
	}
}

func TestWithEnv_Chains(t *testing.T) {
	a := custom.New("test", []string{"echo"})
	b := a.WithEnv("K1", "v1").WithEnv("K2", "v2")
	if b == nil {
		t.Fatal("chained WithEnv() returned nil")
	}
}

func TestWithArgs(t *testing.T) {
	a := custom.New("test", []string{"echo"})
	b := a.WithArgs("--foo", "--bar")
	if b == nil {
		t.Fatal("WithArgs() returned nil")
	}
}

func TestWithWorkDir(t *testing.T) {
	a := custom.New("test", []string{"echo"})
	b := a.WithWorkDir("/tmp")
	if b == nil {
		t.Fatal("WithWorkDir() returned nil")
	}
}

func TestWithTimeout(t *testing.T) {
	a := custom.New("test", []string{"echo"})
	b := a.WithTimeout(2 * time.Minute)
	if b == nil {
		t.Fatal("WithTimeout() returned nil")
	}
}

func TestSendPrompt_NotConnected(t *testing.T) {
	a := custom.New("test", []string{"echo"})
	_, err := a.SendPrompt(context.Background(), "hello")
	if err == nil {
		t.Error("SendPrompt() on disconnected agent should return an error")
	}
}

func TestHandlePermission(t *testing.T) {
	a := custom.New("test", []string{"echo"})
	// Custom agent HandlePermission is always a no-op
	if err := a.HandlePermission("req-1", true); err != nil {
		t.Errorf("HandlePermission() returned unexpected error: %v", err)
	}
}

func TestClose_Unconnected(t *testing.T) {
	a := custom.New("test", []string{"echo"})
	if err := a.Close(); err != nil {
		t.Errorf("Close() on unconnected agent returned unexpected error: %v", err)
	}
}

func TestClose_Idempotent(t *testing.T) {
	a := custom.New("test", []string{"echo"})
	_ = a.Close()
	// Second close should not panic or error
	if err := a.Close(); err != nil {
		t.Errorf("second Close() returned unexpected error: %v", err)
	}
}

func TestConnect_WithCat(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	a := custom.NewWithConfig(custom.Config{
		Name:         "test",
		Command:      []string{"cat"},
		Timeout:      5 * time.Second,
		InputFormat:  "text",
		OutputFormat: "text",
		PermissionHandler: func(req agent.PermissionRequest) bool {
			return true
		},
	})

	if err := a.Connect(ctx); err != nil {
		t.Fatalf("Connect() with cat returned error: %v", err)
	}

	if !a.Connected() {
		t.Error("Connected() should return true after successful Connect()")
	}

	// Clean up the cat process
	if err := a.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

func TestConnect_DoesNotMutateConfigCommand(t *testing.T) {
	// Regression test: Connect must not mutate the Config.Command slice via append.
	command := make([]string, 2, 10) // Extra capacity to trigger append mutation
	command[0] = "cat"
	command[1] = "--help"
	original := make([]string, len(command))
	copy(original, command)

	a := custom.NewWithConfig(custom.Config{
		Name:    "test",
		Command: command,
		Args:    []string{"--extra"},
		Timeout: 2 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Connect may fail (cat --help --extra exits), but that's fine — we're testing slice mutation
	_ = a.Connect(ctx)
	_ = a.Close()

	// Verify the original command slice was not mutated
	for i, v := range original {
		if command[i] != v {
			t.Errorf("Config.Command[%d] mutated: got %q, want %q", i, command[i], v)
		}
	}
}

func TestTimeout_EnforcedOnConnect(t *testing.T) {
	// Agent with very short timeout connecting to a long-running process
	a := custom.NewWithConfig(custom.Config{
		Name:         "test",
		Command:      []string{"sleep", "60"},
		Timeout:      200 * time.Millisecond,
		InputFormat:  "text",
		OutputFormat: "text",
	})

	// Use Background context — timeout should come from Config.Timeout
	ctx := context.Background()
	if err := a.Connect(ctx); err != nil {
		t.Fatalf("Connect() returned error: %v", err)
	}

	// Wait for the timeout to kill the process
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			_ = a.Close()
			t.Fatal("process was not killed by timeout within 2s")
		default:
			if !a.Connected() {
				// Process was killed by timeout
				_ = a.Close()

				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func TestSendPrompt_ConcurrentAccess(t *testing.T) {
	// Regression test: SendPrompt must not race with readOutput on the events channel.
	// This test verifies no panic under -race.
	a := custom.NewWithConfig(custom.Config{
		Name:         "test",
		Command:      []string{"cat"},
		Timeout:      5 * time.Second,
		InputFormat:  "text",
		OutputFormat: "text",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := a.Connect(ctx); err != nil {
		t.Fatalf("Connect() returned error: %v", err)
	}
	defer func() { _ = a.Close() }()

	// Send multiple prompts concurrently — the race detector will catch data races
	ch, err := a.SendPrompt(ctx, "hello")
	if err != nil {
		t.Fatalf("first SendPrompt() returned error: %v", err)
	}

	// Read events to avoid blocking
	go func() {
		for range ch {
		}
	}()

	// Second prompt replaces the channel — must not race
	ch2, err := a.SendPrompt(ctx, "world")
	if err != nil {
		t.Fatalf("second SendPrompt() returned error: %v", err)
	}
	go func() {
		for range ch2 {
		}
	}()
}
