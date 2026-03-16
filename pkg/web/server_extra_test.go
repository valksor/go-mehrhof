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

func TestHandleStatic_Index(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>homepage</html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(dir, 0)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("/ status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), "homepage") {
		t.Error("/ should serve index.html content")
	}
}

func TestHandleStatic_SPAFallback_Dashboard(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>spa</html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(dir, 0)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	w := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("/dashboard SPA fallback status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), "spa") {
		t.Error("/dashboard SPA fallback should serve index.html content")
	}
}

func TestHandleStatic_SPAFallback_DeepRoute(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>deep-route</html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(dir, 0)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	req := httptest.NewRequest(http.MethodGet, "/project/123/tasks", nil)
	w := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("/project/123/tasks SPA fallback status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), "deep-route") {
		t.Error("deep route SPA fallback should serve index.html content")
	}
}

func TestHandleStatic_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.js"), []byte("console.log('hi')"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(dir, 0)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	req := httptest.NewRequest(http.MethodGet, "/app.js", nil)
	w := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("/app.js status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), "console.log") {
		t.Error("/app.js should serve the actual JS file content")
	}
}

func TestHandleStatic_ExistingFile_CSS(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "theme.css"), []byte(".dark { background: #000; }"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(dir, 0)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	req := httptest.NewRequest(http.MethodGet, "/theme.css", nil)
	w := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("/theme.css status = %d, want %d", w.Code, http.StatusOK)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/css") {
		t.Errorf("Content-Type = %q, want text/css", ct)
	}
}

func TestHandleStatic_NoStaticDir_NoEmbedded(t *testing.T) {
	srv, err := NewServer("", 0)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	// Clear the embedded FS to test the 404 path
	srv.embeddedFS = nil

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	srv.handleStatic(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("/ with no static dir or embedded FS status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestServerShutdown(t *testing.T) {
	srv, err := NewServer("", 0)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Shutdown should not error
	err = srv.Shutdown(context.Background())
	if err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}

func TestServerMultipleOptions(t *testing.T) {
	creator := &mockWorktreeCreator{}
	origins := []string{"https://custom.example.com"}

	srv, err := NewServer("", 0, WithAllowedOrigins(origins), WithWorktreeCreator(creator))
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	if len(srv.allowedOrigins) != 1 {
		t.Errorf("allowedOrigins length = %d, want 1", len(srv.allowedOrigins))
	}
	if srv.worktreeCreator == nil {
		t.Error("worktreeCreator should not be nil")
	}

	// Verify the custom origin is accepted
	req := httptest.NewRequest(http.MethodGet, "/ws/global", nil)
	req.Header.Set("Origin", "https://custom.example.com")
	if !srv.checkOrigin(req) {
		t.Error("custom origin should be accepted")
	}
}

func TestSecurityHeaders_AllPresent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(dir, 0)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	// Request a static file and verify security headers
	req := httptest.NewRequest(http.MethodGet, "/index.html", nil)
	w := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(w, req)

	headers := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"X-XSS-Protection":       "1; mode=block",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}

	for header, want := range headers {
		got := w.Header().Get(header)
		if got != want {
			t.Errorf("Header %s = %q, want %q", header, got, want)
		}
	}
}

func TestHandleWorktreeWS_EmptyIDPath(t *testing.T) {
	srv, err := NewServer("", 0)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	req := httptest.NewRequest(http.MethodGet, "/ws/worktree/", nil)
	w := httptest.NewRecorder()

	srv.handleWorktreeWS(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("empty ID path status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if !strings.Contains(w.Body.String(), "missing worktree id") {
		t.Errorf("body = %q, want to contain 'missing worktree id'", w.Body.String())
	}
}

func TestHandleWorktreeWS_PathWithoutPrefix(t *testing.T) {
	srv, err := NewServer("", 0)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	req := httptest.NewRequest(http.MethodGet, "/ws/other/path", nil)
	w := httptest.NewRecorder()

	srv.handleWorktreeWS(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("wrong prefix path status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestServerPort_RandomPort(t *testing.T) {
	srv, err := NewServer("", 0)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	port := srv.Port()
	if port <= 0 || port > 65535 {
		t.Errorf("Port() = %d, want valid port number", port)
	}

	url := srv.URL()
	if !strings.HasPrefix(url, "http://localhost:") {
		t.Errorf("URL() = %q, want http://localhost: prefix", url)
	}
}

func TestHandleStatic_NestedAssets(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	assetsDir := filepath.Join(dir, "assets", "images")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "logo.svg"), []byte("<svg></svg>"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(dir, 0)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	req := httptest.NewRequest(http.MethodGet, "/assets/images/logo.svg", nil)
	w := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("/assets/images/logo.svg status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), "<svg>") {
		t.Error("should serve actual SVG file content")
	}
}
