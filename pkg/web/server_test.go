package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test checkOrigin with wildcard allowing all origins.
func TestCheckOrigin_AllowAll(t *testing.T) {
	srv, err := NewServer("", 0, WithAllowedOrigins([]string{"*"}))
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	tests := []struct {
		name   string
		origin string
		want   bool
	}{
		{"external origin", "https://example.com", true},
		{"localhost", "http://localhost:3000", true},
		{"any domain", "https://evil.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/ws/global", nil)
			req.Header.Set("Origin", tt.origin)

			if got := srv.checkOrigin(req); got != tt.want {
				t.Errorf("checkOrigin() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test checkOrigin with exact origin matching.
func TestCheckOrigin_ExactMatch(t *testing.T) {
	allowed := []string{"https://app.example.com", "https://admin.example.com"}
	srv, err := NewServer("", 0, WithAllowedOrigins(allowed))
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	tests := []struct {
		name   string
		origin string
		want   bool
	}{
		{"allowed origin 1", "https://app.example.com", true},
		{"allowed origin 2", "https://admin.example.com", true},
		{"not in list", "https://other.example.com", false},
		{"localhost still allowed", "http://localhost:3000", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/ws/global", nil)
			req.Header.Set("Origin", tt.origin)

			if got := srv.checkOrigin(req); got != tt.want {
				t.Errorf("checkOrigin() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test checkOrigin allows localhost variants by default.
func TestCheckOrigin_LocalhostVariants(t *testing.T) {
	srv, err := NewServer("", 0) // No explicit allowed origins
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	tests := []struct {
		name   string
		origin string
		want   bool
	}{
		{"localhost http", "http://localhost:3000", true},
		{"localhost https", "https://localhost:3000", true},
		{"127.0.0.1 http", "http://127.0.0.1:8080", true},
		{"127.0.0.1 https", "https://127.0.0.1:8080", true},
		{"::1 http", "http://[::1]:8080", true},
		{"no origin header", "", true}, // Same-origin request
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/ws/global", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			if got := srv.checkOrigin(req); got != tt.want {
				t.Errorf("checkOrigin() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test checkOrigin rejects non-localhost by default.
func TestCheckOrigin_RejectNonLocalhost(t *testing.T) {
	srv, err := NewServer("", 0) // No explicit allowed origins
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	tests := []struct {
		name   string
		origin string
		want   bool
	}{
		{"external domain", "https://example.com", false},
		{"subdomain of localhost", "http://foo.localhost:3000", false},
		{"localhost typo", "http://localhst:3000", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/ws/global", nil)
			req.Header.Set("Origin", tt.origin)

			if got := srv.checkOrigin(req); got != tt.want {
				t.Errorf("checkOrigin() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test checkOrigin with invalid URL in origin header.
func TestCheckOrigin_InvalidURL(t *testing.T) {
	srv, err := NewServer("", 0)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	req := httptest.NewRequest(http.MethodGet, "/ws/global", nil)
	req.Header.Set("Origin", "not-a-valid-url://\\invalid")

	if got := srv.checkOrigin(req); got != false {
		t.Errorf("checkOrigin() with invalid URL = %v, want false", got)
	}
}

// Test security headers middleware.
func TestSecurityHeaders(t *testing.T) {
	srv, err := NewServer("", 0)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(w, req)

	expectedHeaders := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"X-XSS-Protection":       "1; mode=block",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}

	for header, want := range expectedHeaders {
		if got := w.Header().Get(header); got != want {
			t.Errorf("Header %s = %q, want %q", header, got, want)
		}
	}
}

// Test WithAllowedOrigins option function.
func TestWithAllowedOrigins(t *testing.T) {
	origins := []string{"https://example.com", "https://test.com"}
	srv, err := NewServer("", 0, WithAllowedOrigins(origins))
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	if len(srv.allowedOrigins) != 2 {
		t.Errorf("allowedOrigins length = %d, want 2", len(srv.allowedOrigins))
	}
}

// Test WithWorktreeCreator option function.
func TestWithWorktreeCreator(t *testing.T) {
	creator := &mockWorktreeCreator{}
	srv, err := NewServer("", 0, WithWorktreeCreator(creator))
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	if srv.worktreeCreator == nil {
		t.Error("worktreeCreator should not be nil")
	}
}

type mockWorktreeCreator struct{}

func (m *mockWorktreeCreator) GetOrCreateWorktreeSocket(_ string) (interface{}, error) {
	return struct{}{}, nil // Return non-nil empty struct to satisfy interface
}

// Test static file serving with nested path.
func TestHandleStatic_NestedPath(t *testing.T) {
	tmpDir := t.TempDir()
	// Create nested directory structure
	assetsDir := filepath.Join(tmpDir, "assets", "js")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "app.js"), []byte("console.log('test')"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(tmpDir, 0)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	req := httptest.NewRequest(http.MethodGet, "/assets/js/app.js", nil)
	w := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("/assets/js/app.js status = %d, want %d", w.Code, http.StatusOK)
	}
}

// Test static file SPA fallback for non-existent file.
// With SPA routing, non-existent files should fall back to index.html.
func TestHandleStatic_SPAFallback(t *testing.T) {
	tmpDir := t.TempDir()
	// Create only index.html
	if err := os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte("<html>test</html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(tmpDir, 0)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	req := httptest.NewRequest(http.MethodGet, "/nonexistent-route", nil)
	w := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(w, req)

	// SPA fallback: non-existent routes serve index.html with 200
	if w.Code != http.StatusOK {
		t.Errorf("/nonexistent-route status = %d, want %d (SPA fallback)", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), "<html>test</html>") {
		t.Error("SPA fallback should serve index.html content")
	}
}

// Test worktree WebSocket handler with invalid path (not a prefix match).
func TestHandleWorktreeWS_InvalidPath(t *testing.T) {
	srv, err := NewServer("", 0)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	// Create request with path that doesn't match the prefix
	req := httptest.NewRequest(http.MethodGet, "/ws/worktree", nil)
	w := httptest.NewRecorder()

	srv.handleWorktreeWS(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("handleWorktreeWS invalid path status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestNewServer(t *testing.T) {
	srv, err := NewServer("", 0)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	if srv == nil {
		t.Fatal("NewServer returned nil")
	}
	_ = srv.Shutdown(context.Background())
}

func TestServerEmbeddedStaticFallback(t *testing.T) {
	// With no static dir but embedded assets, root should be served from embedded FS
	srv, err := NewServer("", 0)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(w, req)

	// With embedded assets, should get 200 (serves embedded index.html)
	if w.Code != http.StatusOK {
		t.Errorf("/ with embedded assets status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestServerWorktreeWSPathParsing(t *testing.T) {
	srv, err := NewServer("", 0)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	// Missing worktree ID should fail
	req := httptest.NewRequest(http.MethodGet, "/ws/worktree/", nil)
	w := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(w, req)

	// Should fail because it's not a WebSocket upgrade
	if w.Code == http.StatusOK {
		t.Error("/ws/worktree/ should not return 200 without WebSocket upgrade")
	}
}

func TestServerPort(t *testing.T) {
	srv, err := NewServer("", 0)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	port := srv.Port()
	if port <= 0 {
		t.Errorf("Port() = %d, want > 0", port)
	}
}

func TestServerURL(t *testing.T) {
	srv, err := NewServer("", 0)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	url := srv.URL()
	if url == "" {
		t.Error("URL() should not be empty")
	}
	if !strings.HasPrefix(url, "http://localhost:") {
		t.Errorf("URL() = %q, want prefix http://localhost:", url)
	}
}

func TestServerWorktreeWS_MissingID(t *testing.T) {
	srv, err := NewServer("", 0)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	// Call handler directly with a short path that produces < 4 split parts
	req := httptest.NewRequest(http.MethodGet, "/ws/worktree", nil)
	w := httptest.NewRecorder()

	srv.handleWorktreeWS(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("handleWorktreeWS short path status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestServerGlobalWS_NonUpgrade(t *testing.T) {
	srv, err := NewServer("", 0)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	// A plain GET to /ws/global without WebSocket upgrade headers should fail
	req := httptest.NewRequest(http.MethodGet, "/ws/global", nil)
	w := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(w, req)

	if w.Code == http.StatusSwitchingProtocols {
		t.Error("/ws/global without upgrade headers should not return 101")
	}
}

func TestServerWorktreeWS_EmptyID(t *testing.T) {
	srv, err := NewServer("", 0)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	// Call handler directly with trailing slash but no ID
	req := httptest.NewRequest(http.MethodGet, "/ws/worktree/", nil)
	w := httptest.NewRecorder()

	srv.handleWorktreeWS(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("handleWorktreeWS empty ID status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestServerStatic_ContentType(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a CSS file
	if err := os.WriteFile(filepath.Join(tmpDir, "style.css"), []byte("body { color: red; }"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(tmpDir, 0)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	req := httptest.NewRequest(http.MethodGet, "/style.css", nil)
	w := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("/style.css status = %d, want %d", w.Code, http.StatusOK)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/css") {
		t.Errorf("Content-Type = %q, want text/css", ct)
	}
}

func TestServerStatic(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a simple index.html in the temp dir
	if err := os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte("<html>test</html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(tmpDir, 0)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("/ with static dir status = %d, want %d", w.Code, http.StatusOK)
	}
}
