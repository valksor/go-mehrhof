package tui

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/socket"
	"github.com/valksor/kvelmo/internal/testutil"
)

// stubServer is a minimal Unix-socket JSON-RPC server for exercising the TUI
// command handlers without standing up a real conductor. It reads newline
// delimited requests and replies with a canned result keyed by method name.
type stubServer struct {
	t        *testing.T
	ln       net.Listener
	path     string
	mu       sync.Mutex
	results  map[string]json.RawMessage // method -> result payload
	errors   map[string]*socket.RPCError
	gotCalls []stubCall // recorded requests, in order
	wg       sync.WaitGroup
}

type stubCall struct {
	method string
	params json.RawMessage
}

// newStubServer starts a stub server listening on a fresh temp socket path and
// registers cleanup. Handlers are registered via on / onError before/after
// connecting; access is mutex-guarded so it is safe to register concurrently.
func newStubServer(t *testing.T) *stubServer {
	t.Helper()

	return newStubServerAt(t, testutil.TempSocketPath(t))
}

// newStubServerAt starts a stub server listening on a specific socket path.
// Used by tests that drive the path-resolving entry points (executeCommand,
// the Model command methods) which compute the socket path from KVELMO_HOME.
func newStubServerAt(t *testing.T, path string) *stubServer {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "unix", path)
	if err != nil {
		t.Fatalf("listen %s: %v", path, err)
	}
	s := &stubServer{
		t:       t,
		ln:      ln,
		path:    path,
		results: make(map[string]json.RawMessage),
		errors:  make(map[string]*socket.RPCError),
	}
	s.wg.Add(1)
	go s.serve()
	t.Cleanup(func() {
		_ = ln.Close()
		s.wg.Wait()
	})

	return s
}

// on registers a canned result (any JSON-marshalable value) for a method.
func (s *stubServer) on(method string, result any) {
	b, err := json.Marshal(result)
	if err != nil {
		s.t.Fatalf("marshal stub result for %s: %v", method, err)
	}
	s.mu.Lock()
	s.results[method] = b
	s.mu.Unlock()
}

// onError registers an RPC error response for a method.
func (s *stubServer) onError(method string, code int, message string) {
	s.mu.Lock()
	s.errors[method] = &socket.RPCError{Code: code, Message: message}
	s.mu.Unlock()
}

// calls returns a copy of the recorded calls.
func (s *stubServer) calls() []stubCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]stubCall, len(s.gotCalls))
	copy(out, s.gotCalls)

	return out
}

// client dials the stub server and returns a connected socket client.
func (s *stubServer) client(t *testing.T) *socket.Client {
	t.Helper()
	c, err := socket.NewClient(s.path, socket.WithTimeout(2*time.Second))
	if err != nil {
		t.Fatalf("dial stub: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	return c
}

func (s *stubServer) serve() {
	defer s.wg.Done()
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.wg.Add(1)
		go s.handleConn(conn)
	}
}

func (s *stubServer) handleConn(conn net.Conn) {
	defer s.wg.Done()
	defer func() { _ = conn.Close() }()
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var req socket.Request
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}

		s.mu.Lock()
		s.gotCalls = append(s.gotCalls, stubCall{method: req.Method, params: append(json.RawMessage(nil), req.Params...)})
		rpcErr := s.errors[req.Method]
		result, hasResult := s.results[req.Method]
		s.mu.Unlock()

		resp := &socket.Response{ID: req.ID}
		switch {
		case rpcErr != nil:
			resp.Error = rpcErr
		case hasResult:
			resp.Result = result
		default:
			// Default success with an empty object so handlers that ignore the
			// body still succeed.
			resp.Result = json.RawMessage(`{}`)
		}

		out, err := socket.EncodeResponse(resp)
		if err != nil {
			return
		}
		if _, err := conn.Write(out); err != nil {
			return
		}
	}
}
