package apiagent_test

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent/apiagent"
)

// fakeProvider implements the Provider interface for testing Base.
type fakeProvider struct {
	name      string
	available error
}

func (f *fakeProvider) Name() string     { return f.name }
func (f *fakeProvider) Available() error { return f.available }
func (f *fakeProvider) BuildRequest(ctx context.Context, _ *apiagent.APIConfig, _ []apiagent.Message, _ []apiagent.ToolDef) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, http.MethodPost, "https://example.com", nil)
}

func (f *fakeProvider) ParseStream(_ context.Context, body io.ReadCloser) (<-chan apiagent.Chunk, error) {
	ch := make(chan apiagent.Chunk, 1)
	go func() {
		defer close(ch)
		defer body.Close() //nolint:errcheck // close is best-effort
		ch <- apiagent.Chunk{Type: apiagent.ChunkDone}
	}()

	return ch, nil
}

func TestBase_NameAndAvailable(t *testing.T) {
	p := &fakeProvider{name: "fake"}
	b := apiagent.NewBase(p, apiagent.APIConfig{})

	if b.Name() != "fake" {
		t.Errorf("Name = %q", b.Name())
	}
	if err := b.Available(); err != nil {
		t.Errorf("Available: %v", err)
	}
}

func TestBase_ConnectAndConnected(t *testing.T) {
	p := &fakeProvider{name: "fake"}
	b := apiagent.NewBase(p, apiagent.APIConfig{})

	if b.Connected() {
		t.Error("Connected() should be false before Connect")
	}
	if err := b.Connect(context.Background()); err != nil {
		t.Errorf("Connect: %v", err)
	}
	if !b.Connected() {
		t.Error("Connected() should be true after Connect")
	}
}

func TestBase_HandlePermissionIsNoop(t *testing.T) {
	b := apiagent.NewBase(&fakeProvider{name: "x"}, apiagent.APIConfig{})
	if err := b.HandlePermission("req-1", true); err != nil {
		t.Errorf("HandlePermission: %v", err)
	}
}

func TestBase_Close(t *testing.T) {
	b := apiagent.NewBase(&fakeProvider{name: "x"}, apiagent.APIConfig{})
	_ = b.Connect(context.Background())

	if err := b.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	if b.Connected() {
		t.Error("Connected() should be false after Close")
	}
}

func TestBase_Interrupt(t *testing.T) {
	b := apiagent.NewBase(&fakeProvider{name: "x"}, apiagent.APIConfig{})
	if err := b.Interrupt(); err != nil {
		t.Errorf("Interrupt: %v", err)
	}
}

func TestBase_WithEnv_APIKey(t *testing.T) {
	b := apiagent.NewBase(&fakeProvider{name: "x"}, apiagent.APIConfig{})

	cloned := b.WithEnv("OPENAI_API_KEY", "sk-new")
	if cloned == nil {
		t.Fatal("WithEnv returned nil")
	}
	// Different env vars should still produce a clone (no-op for key).
	if other := b.WithEnv("OTHER", "value"); other == nil {
		t.Fatal("WithEnv with unknown key returned nil")
	}
}

func TestBase_WithArgs(t *testing.T) {
	b := apiagent.NewBase(&fakeProvider{name: "x"}, apiagent.APIConfig{})
	if cloned := b.WithArgs("--flag", "value"); cloned == nil {
		t.Error("WithArgs returned nil")
	}
}

func TestBase_WithTimeout(t *testing.T) {
	b := apiagent.NewBase(&fakeProvider{name: "x"}, apiagent.APIConfig{})
	if cloned := b.WithTimeout(5 * time.Second); cloned == nil {
		t.Error("WithTimeout returned nil")
	}
}

func TestBase_WithWorkDir(t *testing.T) {
	b := apiagent.NewBase(&fakeProvider{name: "x"}, apiagent.APIConfig{})
	if cloned := b.WithWorkDir("/tmp/test"); cloned == nil {
		t.Error("WithWorkDir returned nil")
	}
}

func TestBase_SendPromptNotConnected(t *testing.T) {
	b := apiagent.NewBase(&fakeProvider{name: "x"}, apiagent.APIConfig{})
	// Without Connect, SendPrompt should error.
	if _, err := b.SendPrompt(context.Background(), "hello"); err == nil {
		t.Error("expected error when not connected")
	}
}

func TestBase_DefaultsApplied(t *testing.T) {
	b := apiagent.NewBase(&fakeProvider{name: "x"}, apiagent.APIConfig{})
	if b == nil {
		t.Fatal("NewBase returned nil")
	}
	// Defaults should be applied; we can't introspect private fields but we can
	// at least observe Connect doesn't immediately fail.
	if err := b.Connect(context.Background()); err != nil {
		t.Errorf("Connect with defaults: %v", err)
	}
}
