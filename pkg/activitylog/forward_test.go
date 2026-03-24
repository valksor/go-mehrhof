package activitylog

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func testEntry() Entry {
	return Entry{
		Timestamp:  time.Now(),
		Method:     "test.method",
		DurationMs: 42,
		ParamsSize: 100,
	}
}

func TestNewForwarder(t *testing.T) {
	f := NewForwarder("https://example.com/hook", "Bearer abc")
	if f.url != "https://example.com/hook" {
		t.Errorf("url = %q, want %q", f.url, "https://example.com/hook")
	}
	if f.auth != "Bearer abc" {
		t.Errorf("auth = %q, want %q", f.auth, "Bearer abc")
	}
	if f.client == nil {
		t.Fatal("client should not be nil")
	}
}

func TestForward_SendsCorrectBody(t *testing.T) {
	entry := testEntry()

	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		received, err = io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := NewForwarder(srv.URL, "")
	f.Forward(context.Background(), entry)

	var got Entry
	if err := json.Unmarshal(received, &got); err != nil {
		t.Fatalf("unmarshal received body: %v", err)
	}
	if got.Method != entry.Method {
		t.Errorf("Method = %q, want %q", got.Method, entry.Method)
	}
	if got.DurationMs != entry.DurationMs {
		t.Errorf("DurationMs = %d, want %d", got.DurationMs, entry.DurationMs)
	}
	if got.ParamsSize != entry.ParamsSize {
		t.Errorf("ParamsSize = %d, want %d", got.ParamsSize, entry.ParamsSize)
	}
}

func TestForward_SetsContentType(t *testing.T) {
	var contentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := NewForwarder(srv.URL, "")
	f.Forward(context.Background(), testEntry())

	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}
}

func TestForward_WithAuthHeader(t *testing.T) {
	var authHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := NewForwarder(srv.URL, "Bearer token-123")
	f.Forward(context.Background(), testEntry())

	if authHeader != "Bearer token-123" {
		t.Errorf("Authorization = %q, want %q", authHeader, "Bearer token-123")
	}
}

func TestForward_WithoutAuthHeader(t *testing.T) {
	var hasAuth bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hasAuth = r.Header["Authorization"]
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := NewForwarder(srv.URL, "")
	f.Forward(context.Background(), testEntry())

	if hasAuth {
		t.Error("Authorization header should not be set when auth is empty")
	}
}

func TestForward_ErrorEndpointDoesNotPanic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	f := NewForwarder(srv.URL, "")
	// Should not panic — Forward is best-effort.
	f.Forward(context.Background(), testEntry())
}

func TestForwardAsync_FiresGoroutine(t *testing.T) {
	var mu sync.Mutex
	var called bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		called = true
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := NewForwarder(srv.URL, "")
	f.ForwardAsync(context.Background(), testEntry())

	// Wait briefly for the goroutine to complete.
	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		done := called
		mu.Unlock()
		if done {
			return
		}
		select {
		case <-deadline:
			t.Fatal("ForwardAsync did not call the endpoint within timeout")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestValidateURL_Reachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("expected HEAD request, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := NewForwarder(srv.URL, "")
	if err := f.ValidateURL(context.Background()); err != nil {
		t.Errorf("ValidateURL on reachable server: %v", err)
	}
}

func TestValidateURL_Unreachable(t *testing.T) {
	// Start and immediately close a server to get an unreachable URL.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	f := NewForwarder(url, "")
	if err := f.ValidateURL(context.Background()); err == nil {
		t.Error("ValidateURL on closed server should return an error")
	}
}
