package web

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/valksor/kvelmo/internal/socket"
)

// ─── Option mutator coverage ────────────────────────────────────────────────

func TestWithTLS_SetsCertAndKey(t *testing.T) {
	srv, err := NewServer("", 0, WithTLS("/etc/ssl/cert.pem", "/etc/ssl/key.pem"))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	if srv.tlsCertFile != "/etc/ssl/cert.pem" {
		t.Errorf("tlsCertFile = %q", srv.tlsCertFile)
	}
	if srv.tlsKeyFile != "/etc/ssl/key.pem" {
		t.Errorf("tlsKeyFile = %q", srv.tlsKeyFile)
	}
}

func TestWithAPIOnly_DisablesStatic(t *testing.T) {
	dir := t.TempDir()
	// Even with a static dir, API-only mode should not register the /handler.
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(dir, 0, WithAPIOnly())
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	if !srv.apiOnly {
		t.Error("apiOnly flag should be true")
	}

	// "/" should not be served — the mux only has WS, health, metrics, and /api.
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rr := newRecorder()
	srv.httpServer.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("api-only mode / = %d, want 404", rr.Code)
	}
}

func TestWithGlobalSocketPath_StoresPath(t *testing.T) {
	srv, err := NewServer("", 0, WithGlobalSocketPath("/tmp/test-global.sock"))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	if srv.globalSocketPath != "/tmp/test-global.sock" {
		t.Errorf("globalSocketPath = %q", srv.globalSocketPath)
	}
}

func TestServerURL_TLSPrefix(t *testing.T) {
	srv, err := NewServer("", 0, WithTLS("cert.pem", "key.pem"))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	url := srv.URL()
	if !strings.HasPrefix(url, "https://") {
		t.Errorf("URL() = %q, want https:// prefix", url)
	}
}

// ─── Start lifecycle ────────────────────────────────────────────────────────

func TestServerStart_HTTP(t *testing.T) {
	srv, err := NewServer("", 0)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	startErr := make(chan error, 1)
	go func() { startErr <- srv.Start() }()

	// Probe /healthz to confirm the server is actually accepting connections.
	deadline := time.Now().Add(3 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatal("server never became ready")
		}
		resp, err := httpGet(t, srv.URL()+"/healthz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}

	if err := srv.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown: %v", err)
	}

	// Start should return when the server stops; ServeTLS/Serve return http.ErrServerClosed on graceful shutdown.
	if err := <-startErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		t.Errorf("Start: unexpected error: %v", err)
	}
}

func TestServerStart_TLS(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	if err := generateSelfSignedCert(certPath, keyPath); err != nil {
		t.Fatalf("generate cert: %v", err)
	}

	srv, err := NewServer("", 0, WithTLS(certPath, keyPath))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	startErr := make(chan error, 1)
	go func() { startErr <- srv.Start() }()

	// Give the TLS handshake a moment; we don't actually need to complete a
	// request, just confirm Start() reaches ServeTLS without returning early.
	time.Sleep(50 * time.Millisecond)

	if err := srv.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown: %v", err)
	}
	if err := <-startErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		t.Errorf("Start (TLS): unexpected error: %v", err)
	}
}

// ─── API endpoint coverage with real socket backend ─────────────────────────

func TestHandleAPIState_WithSocket(t *testing.T) {
	shortKvelmoHome(t)

	sockPath := socket.GlobalSocketPath()
	_ = os.MkdirAll(filepath.Dir(sockPath), 0o755)
	startTestSocket(t, sockPath, map[string]any{
		"projects.list": []any{map[string]any{"name": "p"}},
		"tasks.list":    []any{},
		"workers.stats": map[string]any{"count": 0},
		"system.health": map[string]any{"ok": true},
	})

	srv, err := NewServer("", 0)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/api/state", nil)
	rr := newRecorder()
	srv.handleAPIState(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	wantKeys := []string{"timestamp", "metrics", "projects", "tasks", "workers", "health"}
	for _, k := range wantKeys {
		if _, ok := body[k]; !ok {
			t.Errorf("response missing key %q", k)
		}
	}
}

func TestHandleAPITasks_WithSocket(t *testing.T) {
	shortKvelmoHome(t)

	sockPath := socket.GlobalSocketPath()
	_ = os.MkdirAll(filepath.Dir(sockPath), 0o755)
	startTestSocket(t, sockPath, map[string]any{
		"tasks.list": []any{map[string]any{"id": "t1", "state": "implementing"}},
	})

	srv, err := NewServer("", 0)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/api/tasks", nil)
	rr := newRecorder()
	srv.handleAPITasks(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := body["active"]; !ok {
		t.Error("response missing 'active'")
	}
}

// ─── WebSocket proxy coverage (handleGlobalWS + proxyConnections) ──────────

func TestHandleGlobalWS_ProxiesMessages(t *testing.T) {
	shortKvelmoHome(t)

	sockPath := socket.GlobalSocketPath()
	_ = os.MkdirAll(filepath.Dir(sockPath), 0o755)
	echoSocket(t, sockPath)

	srv := startWebServer(t)

	wsURL := strings.Replace(srv.URL(), "http://", "ws://", 1) + "/ws/global"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, dialResp, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial: %v", err)
	}
	if dialResp != nil && dialResp.Body != nil {
		_ = dialResp.Body.Close()
	}
	defer func() { _ = conn.CloseNow() }()

	// Send a line; the echo socket on the other end appends and replies with a JSONL response.
	req := `{"jsonrpc":"2.0","id":"1","method":"echo","params":null}` + "\n"
	if err := conn.Write(ctx, websocket.MessageText, []byte(req)); err != nil {
		t.Fatalf("ws write: %v", err)
	}
	_, msg, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("ws read: %v", err)
	}
	if !strings.Contains(string(msg), "echo") {
		t.Errorf("expected echo response, got %q", string(msg))
	}
}

func TestHandleGlobalWS_SocketUnavailable(t *testing.T) {
	// Point KVELMO_HOME at a place where no socket exists. handleGlobalWS
	// should accept the WS upgrade, write an error frame, and close.
	shortKvelmoHome(t)

	srv := startWebServer(t)

	wsURL := strings.Replace(srv.URL(), "http://", "ws://", 1) + "/ws/global"
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	conn, dialResp, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial: %v", err)
	}
	if dialResp != nil && dialResp.Body != nil {
		_ = dialResp.Body.Close()
	}
	defer func() { _ = conn.CloseNow() }()

	_, msg, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("ws read error frame: %v", err)
	}
	if !strings.Contains(string(msg), "error") {
		t.Errorf("expected error frame, got %q", string(msg))
	}
}

// ─── handleWorktreeWS proxy round-trip ─────────────────────────────────────

func TestHandleWorktreeWS_ProxiesMessages(t *testing.T) {
	shortKvelmoHome(t)

	// Use a stable worktree id so we can pre-compute the socket path.
	worktreeID := "/tmp/kv-worktree-test"
	sockPath := socket.WorktreeSocketPath(worktreeID)
	_ = os.MkdirAll(filepath.Dir(sockPath), 0o755)
	startTestSocket(t, sockPath, map[string]any{
		"ping": map[string]string{"reply": "pong"},
	})

	srv := startWebServer(t)

	// URL-encode the worktree ID so the handler can decode the path back.
	encodedID := strings.ReplaceAll(worktreeID, "/", "%2F")
	wsURL := strings.Replace(srv.URL(), "http://", "ws://", 1) + "/ws/worktree/" + encodedID

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, dialResp, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial: %v", err)
	}
	if dialResp != nil && dialResp.Body != nil {
		_ = dialResp.Body.Close()
	}
	defer func() { _ = conn.CloseNow() }()

	req := `{"jsonrpc":"2.0","id":"1","method":"ping","params":null}` + "\n"
	if err := conn.Write(ctx, websocket.MessageText, []byte(req)); err != nil {
		t.Fatalf("ws write: %v", err)
	}
	_, msg, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("ws read: %v", err)
	}
	if !strings.Contains(string(msg), "pong") {
		t.Errorf("expected pong response, got %q", string(msg))
	}
}

func TestHandleWorktreeWS_CreatorError(t *testing.T) {
	shortKvelmoHome(t)

	creator := &failingWorktreeCreator{}
	srv, err := NewServer("", 0, WithWorktreeCreator(creator))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	startErr := make(chan error, 1)
	go func() { startErr <- srv.Start() }()
	t.Cleanup(func() {
		_ = srv.Shutdown(context.Background())
		<-startErr
	})

	// Wait for readiness.
	deadline := time.Now().Add(3 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatal("server never became ready")
		}
		resp, err := httpGet(t, srv.URL()+"/healthz")
		if err == nil {
			_ = resp.Body.Close()

			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	resp, err := httpGet(t, srv.URL()+"/ws/worktree/"+strings.ReplaceAll("/some/path", "/", "%2F"))
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}

// ─── WorktreeCreatorClient ──────────────────────────────────────────────────

func TestGetOrCreateWorktreeSocket_Success(t *testing.T) {
	shortKvelmoHome(t)

	sockPath := socket.GlobalSocketPath()
	_ = os.MkdirAll(filepath.Dir(sockPath), 0o755)
	startTestSocket(t, sockPath, map[string]any{
		"worktrees.create": map[string]string{"socket_path": "/path/to/sock"},
	})

	client := NewWorktreeCreatorClient(sockPath)
	got, err := client.GetOrCreateWorktreeSocket("/project")
	if err != nil {
		t.Fatalf("GetOrCreateWorktreeSocket: %v", err)
	}
	if got != "/path/to/sock" {
		t.Errorf("socket_path = %v, want /path/to/sock", got)
	}
}

func TestGetOrCreateWorktreeSocket_ServerError(t *testing.T) {
	shortKvelmoHome(t)

	sockPath := socket.GlobalSocketPath()
	_ = os.MkdirAll(filepath.Dir(sockPath), 0o755)

	// Server that returns an error for worktrees.create.
	sockSrv := socket.NewServer(sockPath)
	sockSrv.Handle("worktrees.create", func(_ context.Context, req *socket.Request) (*socket.Response, error) {
		return socket.NewErrorResponse(req.ID, -32000, "creation failed"), nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = sockSrv.Start(ctx)
		close(done)
	}()
	t.Cleanup(func() { cancel(); <-done })

	// Wait for socket.
	for range 100 {
		if _, err := os.Stat(sockPath); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	client := NewWorktreeCreatorClient(sockPath)
	_, err := client.GetOrCreateWorktreeSocket("/project")
	if err == nil || !strings.Contains(err.Error(), "creation failed") {
		t.Errorf("expected creation failed error, got %v", err)
	}
}

type failingWorktreeCreator struct{}

func (f *failingWorktreeCreator) GetOrCreateWorktreeSocket(_ string) (any, error) {
	return nil, errors.New("simulated creation failure")
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func startWebServer(t *testing.T) *Server {
	t.Helper()
	srv, err := NewServer("", 0)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	startErr := make(chan error, 1)
	go func() { startErr <- srv.Start() }()
	t.Cleanup(func() {
		_ = srv.Shutdown(context.Background())
		<-startErr
	})

	// Wait until the server is accepting connections.
	deadline := time.Now().Add(3 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatal("server never became ready")
		}
		resp, err := httpGet(t, srv.URL()+"/healthz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return srv
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// startTestSocket runs a Unix socket server that answers the given RPC methods
// with the given results. The server is stopped during test cleanup.
func startTestSocket(t *testing.T, path string, responses map[string]any) {
	t.Helper()
	sockSrv := socket.NewServer(path)
	for method, result := range responses {
		sockSrv.Handle(method, func(_ context.Context, req *socket.Request) (*socket.Response, error) {
			return socket.NewResultResponse(req.ID, result)
		})
	}

	ctx, cancel := context.WithCancel(context.Background())
	startErr := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		startErr <- sockSrv.Start(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	// Wait for the socket file to appear, surfacing the Start error if it failed early.
	for range 100 {
		select {
		case err := <-startErr:
			if err != nil {
				t.Fatalf("socket server failed to start: %v (path %s)", err, path)
			}
		default:
		}
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("socket %s never appeared", path)
}

// shortKvelmoHome creates a short temp dir (avoids the macOS 104-byte Unix
// socket path limit) and sets KVELMO_HOME for the duration of the test.
// t.TempDir resolves to long /var/folders/... paths on macOS that exceed
// the Unix domain socket name limit, so we deliberately MkdirTemp in /tmp.
func shortKvelmoHome(t *testing.T) {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "kv-") //nolint:usetesting // need short path under macOS sockaddr_un limit
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	t.Setenv("KVELMO_HOME", dir)
}

// echoSocket runs a Unix socket that echoes any line it receives, decorating
// it so the response is recognizable. Used for WS proxy round-trip tests.
func echoSocket(t *testing.T, path string) {
	t.Helper()
	startTestSocket(t, path, map[string]any{
		"echo": map[string]string{"reply": "echo"},
	})
}

func newRecorder() *responseRecorder { return &responseRecorder{header: http.Header{}} }

type responseRecorder struct {
	Code   int
	header http.Header
	Body   *bytes.Buffer
}

func (r *responseRecorder) Header() http.Header {
	if r.header == nil {
		r.header = http.Header{}
	}

	return r.header
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.Body == nil {
		r.Body = &bytes.Buffer{}
	}
	if r.Code == 0 {
		r.Code = http.StatusOK
	}

	return r.Body.Write(b)
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.Code = statusCode
}

// httpGet wraps http.Get with a context-aware request to satisfy linting.
func httpGet(t *testing.T, url string) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	return http.DefaultClient.Do(req)
}

func generateSelfSignedCert(certPath, keyPath string) error {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return err
	}

	certOut, err := os.Create(certPath)
	if err != nil {
		return err
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		_ = certOut.Close()

		return err
	}
	if err := certOut.Close(); err != nil {
		return err
	}

	keyOut, err := os.Create(keyPath)
	if err != nil {
		return err
	}
	keyBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		_ = keyOut.Close()

		return err
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes}); err != nil {
		_ = keyOut.Close()

		return err
	}

	return keyOut.Close()
}
