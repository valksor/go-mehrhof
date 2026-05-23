package commands

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/socket"
	"github.com/valksor/kvelmo/meta"
)

// shortKvelmoHome creates a short temp dir (avoids the macOS 104-byte Unix
// socket path limit) and sets KVELMO_HOME for the duration of the test.
// t.TempDir resolves to long /var/folders/... paths on macOS that exceed
// the Unix domain socket name limit, so we deliberately MkdirTemp in /tmp.
func shortKvelmoHome(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "kv-") //nolint:usetesting // need short path under macOS sockaddr_un limit
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	t.Setenv(meta.EnvPrefix+"_HOME", dir)

	return dir
}

// stubResponses maps RPC method name to a JSON response body. Used to register
// canned successful responses on a test socket so runX commands can exercise
// their full happy-path code (not just the no-socket error branch).
type stubResponses map[string]any

// defaultStubResponses returns canned responses that satisfy the JSON shapes
// expected by the largest number of runX handlers. Most commands only require
// a small subset of fields; supplying a broad envelope keeps tests forgiving.
func defaultStubResponses() stubResponses {
	return stubResponses{
		// Generic status/metadata-style replies
		"ping":            map[string]any{"ok": true},
		"status":          map[string]any{"state": "implemented", "task_id": "t1"},
		"agent.status":    map[string]any{"agent_available": true, "checks": []any{}},
		"system.health":   map[string]any{"status": "healthy", "components": []any{}},
		"system.diagnose": map[string]any{"checks": []any{}, "ok": true},
		"shutdown":        map[string]any{"ok": true},

		// Activity / audit / export — include sample entries so per-entry print branches execute
		"activity.query": map[string]any{
			"entries": []any{
				map[string]any{
					"timestamp":   "2026-05-19T10:00:00Z",
					"method":      "ping",
					"duration_ms": 5,
					"error":       "",
					"user_id":     "u1",
					"task_id":     "t1",
				},
				map[string]any{
					"timestamp":   "2026-05-19T10:00:01Z",
					"method":      "tasks.list",
					"duration_ms": 12,
					"error":       "boom",
					"user_id":     "u1",
				},
			},
			"count":   2,
			"enabled": true,
		},
		"export": map[string]any{
			"tasks": []any{
				map[string]any{"id": "t1", "path": "/p", "state": "implementing"},
			},
			"activity": []any{
				map[string]any{
					"timestamp":   "2026-05-19T10:00:00Z",
					"method":      "ping",
					"duration_ms": 5,
					"user_id":     "u1",
				},
			},
		},

		// Tasks — populate with sample data so loops execute
		"tasks.list": []any{
			map[string]any{
				"id":      "t1",
				"path":    "/p",
				"state":   "implementing",
				"text":    "Sample task",
				"created": "2026-05-19T09:00:00Z",
			},
		},
		"tasks.batch":  map[string]any{"results": []any{map[string]any{"id": "t1", "status": "ok"}}},
		"task.history": map[string]any{"tasks": []any{map[string]any{"id": "t1", "state": "finished"}}, "limit": 10},
		"task.search":  map[string]any{"results": []any{map[string]any{"id": "t1", "score": 0.9}}},
		"task.refresh": map[string]any{"updated": false},
		"task.finish":  map[string]any{"state": "finished"},
		"task.export":  map[string]any{"id": "t1", "path": "/tmp/x"},
		"task.tag":     map[string]any{"tags": []any{}},
		"abandon":      map[string]any{"ok": true},
		"abort":        map[string]any{"status": "aborted", "state": "paused"},
		"reset":        map[string]any{"state": "none"},
		"delete":       map[string]any{"deleted": true},
		"update":       map[string]any{"updated": false},
		"stop":         map[string]any{"ok": true},
		"undo":         map[string]any{"state": "planning"},
		"redo":         map[string]any{"state": "implementing"},
		"start":        map[string]any{"task_id": "t1", "state": "loaded"},
		"plan":         map[string]any{"state": "planned"},
		"implement":    map[string]any{"state": "implemented"},
		"simplify":     map[string]any{"state": "simplified"},
		"optimize":     map[string]any{"state": "optimized"},
		"review":       map[string]any{"state": "reviewing"},
		"submit":       map[string]any{"state": "submitted"},

		// Checkpoints
		"checkpoints":        map[string]any{"checkpoints": []any{}},
		"checkpoint.goto":    map[string]any{"sha": "deadbeefcafebabe"},
		"checkpoint.preview": map[string]any{"diff": ""},

		// Queue
		"queue.add":     map[string]any{"queued": true, "id": "q1"},
		"queue.remove":  map[string]any{"removed": true},
		"queue.list":    map[string]any{"queue": []any{}},
		"queue.reorder": map[string]any{"reordered": true},

		// Memory
		"memory.search":   map[string]any{"results": []any{}},
		"memory.stats":    map[string]any{"count": 0},
		"memory.clear":    map[string]any{"cleared": 0},
		"memory.outcomes": map[string]any{"outcomes": []any{}},

		// Workers
		"workers.list": map[string]any{"workers": []any{
			map[string]any{"id": "w1", "agent": "claude", "status": "idle"},
		}},
		"workers.add":    map[string]any{"id": "w1"},
		"workers.remove": map[string]any{"removed": true},
		"workers.stats":  map[string]any{"count": 1, "active": 0, "idle": 1},

		// Projects
		"projects.list": []any{
			map[string]any{"path": "/p1", "name": "proj1", "state": "implementing"},
			map[string]any{"path": "/p2", "name": "proj2", "state": "reviewing"},
		},
		"projects.register":   map[string]any{"path": "/tmp/p"},
		"projects.unregister": map[string]any{"unregistered": true},

		// Strategy / discovery
		"strategy.list":  map[string]any{"strategies": []any{}},
		"discovery.scan": map[string]any{"commands": []any{}},

		// Providers
		"providers.test": map[string]any{"ok": true},

		// Settings
		"settings.get": map[string]any{},
		"settings.set": map[string]any{"ok": true},

		// Backup / restore
		"backup.create":  map[string]any{"path": "/tmp/b.bk"},
		"backup.list":    map[string]any{"backups": []any{}},
		"backup.restore": map[string]any{"restored": true},

		// Logs
		"logs.tail": map[string]any{"lines": []any{}},

		// Notify
		"notify.test": map[string]any{"sent": true},

		// Catalog
		"catalog.list":   map[string]any{"items": []any{}},
		"catalog.import": map[string]any{"imported": false},
		"catalog.get":    map[string]any{"item": map[string]any{}},

		// Cache
		"cache.stats": map[string]any{"hits": 0, "misses": 0},
		"cache.clear": map[string]any{"cleared": 0},

		// CI / autofix / hooks / policy
		"ci.status":      map[string]any{"runs": []any{}},
		"autofix.status": map[string]any{"attempts": 0},
		"hooks.list":     map[string]any{"hooks": []any{}},
		"policy.check":   map[string]any{"ok": true},
		"config.check":   map[string]any{"ok": true, "issues": []any{}},

		// Codegraph
		"codegraph.stats":   map[string]any{"symbols": 0},
		"codegraph.index":   map[string]any{"indexed": 0},
		"codegraph.search":  map[string]any{"results": []any{}},
		"codegraph.callers": map[string]any{"callers": []any{}},
		"codegraph.deps":    map[string]any{"deps": []any{}},

		// Adversarial
		"adversarial.run":     map[string]any{"started": true},
		"adversarial.results": map[string]any{"results": []any{}},

		// Risk / failclass / eventlog / progress
		"risk.evaluate":   map[string]any{"score": 0.0},
		"risk.history":    map[string]any{"history": []any{}},
		"failclass.stats": map[string]any{"flaky": 0, "genuine": 0},
		"eventlog.query":  map[string]any{"events": []any{}},
		"progress.get":    map[string]any{"progress": 0.0},

		// Fork
		"fork.create":  map[string]any{"id": "f1"},
		"fork.list":    map[string]any{"forks": []any{}},
		"fork.compare": map[string]any{"comparison": map[string]any{}},
		"fork.select":  map[string]any{"selected": true},

		// Provision
		"provision.preview": map[string]any{"actions": []any{}},

		// Review extras
		"review.list":              map[string]any{"reviews": []any{}},
		"review.view":              map[string]any{"id": 1, "content": ""},
		"review.checklist.get":     map[string]any{"items": []any{}},
		"review.checklist.check":   map[string]any{"ok": true},
		"review.checklist.uncheck": map[string]any{"ok": true},

		// Remote ops
		"remote.approve": map[string]any{"approved": true},
		"remote.merge":   map[string]any{"merged": true},

		// Screenshots
		"screenshots.list":    map[string]any{"screenshots": []any{}},
		"screenshots.get":     map[string]any{"path": "/tmp/x.png"},
		"screenshots.capture": map[string]any{"id": "s1", "path": "/tmp/x.png"},
		"screenshots.delete":  map[string]any{"deleted": true},

		// Security
		"security.scan": map[string]any{"findings": []any{}},

		// Reporting / changelog
		"report.generate":    map[string]any{"path": "/tmp/r.json"},
		"changelog.generate": map[string]any{"changelog": ""},

		// Files
		"files.list":   map[string]any{"files": []any{}},
		"files.search": map[string]any{"results": []any{}},
		"browse":       map[string]any{"items": []any{}},

		// Chat
		"chat.send":    map[string]any{"ok": true},
		"chat.history": map[string]any{"messages": []any{}},
		"chat.stop":    map[string]any{"stopped": true},
		"chat.clear":   map[string]any{"cleared": 0},

		// Recordings
		"recordings.list": map[string]any{"recordings": []any{}},
		"recordings.view": map[string]any{"recording": map[string]any{}},

		// Task groups
		"taskgroup.create": map[string]any{"id": "g1"},
		"taskgroup.add":    map[string]any{"added": true},
		"taskgroup.list":   map[string]any{"groups": []any{}},
		"taskgroup.status": map[string]any{"status": map[string]any{}},
		"taskgroup.submit": map[string]any{"submitted": true},
		"taskgroup.remove": map[string]any{"removed": true},

		// Suggestions / metrics / approve
		"suggestions.list": map[string]any{"suggestions": []any{}},
		"metrics.history":  map[string]any{"snapshots": []any{}},
		"approve":          map[string]any{"approved": true},
		"approve.node":     map[string]any{"approved": true},

		// Recap
		"recap": map[string]any{"summary": ""},

		// Git
		"git.status":       map[string]any{"branch": "main", "files": []any{}},
		"git.diff":         map[string]any{"diff": ""},
		"git.diff_against": map[string]any{"diff": ""},
		"git.log":          map[string]any{"commits": []any{}},

		// Jobs
		"jobs.list": map[string]any{"jobs": []any{}},
		"jobs.get":  map[string]any{"job": map[string]any{}},

		// Show
		"show.spec": map[string]any{"content": ""},
		"show.plan": map[string]any{"content": ""},

		// Quality
		"quality.run":       map[string]any{"findings": []any{}},
		"quality.respond":   map[string]any{"ok": true},
		"quality.failclass": map[string]any{"flaky": 0},

		// Browser (agent-friendly methods)
		"browser.navigate":   map[string]any{"url": "https://example.com", "title": "Example"},
		"browser.snapshot":   map[string]any{"snapshot": "<html></html>"},
		"browser.screenshot": map[string]any{"path": "/tmp/x.png"},
		"browser.click":      map[string]any{"ok": true},
		"browser.type":       map[string]any{"ok": true},
		"browser.wait":       map[string]any{"ok": true},
		"browser.eval":       map[string]any{"result": "2"},
		"browser.console":    map[string]any{"logs": []any{}},
		"browser.network":    map[string]any{"requests": []any{}},
		"browser.fill":       map[string]any{"ok": true},
		"browser.select":     map[string]any{"ok": true},
		"browser.hover":      map[string]any{"ok": true},
		"browser.focus":      map[string]any{"ok": true},
		"browser.scroll":     map[string]any{"ok": true},
		"browser.press":      map[string]any{"ok": true},
		"browser.back":       map[string]any{"ok": true},
		"browser.forward":    map[string]any{"ok": true},
		"browser.reload":     map[string]any{"ok": true},
		"browser.dialog":     map[string]any{"ok": true},
		"browser.upload":     map[string]any{"ok": true},
		"browser.pdf":        map[string]any{"path": "/tmp/x.pdf"},
		"browser.status":     map[string]any{"running": false},
		"browser.config.get": map[string]any{"config": map[string]any{}},
		"browser.config.set": map[string]any{"ok": true},
		"browser.install":    map[string]any{"installed": true},
	}
}

// stubSocketServer wraps a socket.Server with thread-safe response stubbing.
type stubSocketServer struct {
	srv       *socket.Server
	mu        sync.RWMutex
	responses stubResponses
}

// startStubGlobalSocket spins up a global socket at the standard path (under
// the test's KVELMO_HOME) that answers every well-known method with a canned
// response. Returns when the socket file is observable on disk.
func startStubGlobalSocket(t *testing.T) *stubSocketServer {
	t.Helper()

	return startStubSocketAt(t, socket.GlobalSocketPath())
}

// startStubWorktreeSocket spins up a worktree socket at the path derived from
// the current working directory.
func startStubWorktreeSocket(t *testing.T) *stubSocketServer {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	return startStubSocketAt(t, socket.WorktreeSocketPath(cwd))
}

func startStubSocketAt(t *testing.T, path string) *stubSocketServer {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir socket parent: %v", err)
	}

	stub := &stubSocketServer{
		srv:       socket.NewServer(path),
		responses: defaultStubResponses(),
	}

	// Wildcard-ish registration: register a handler for every default method.
	// The handler reads the current response (allowing per-test overrides via
	// SetResponse) and returns it. Unknown methods will not match and will
	// surface as a socket-level error, which is fine for negative tests.
	for method := range stub.responses {
		stub.srv.Handle(method, func(_ context.Context, req *socket.Request) (*socket.Response, error) {
			stub.mu.RLock()
			payload, ok := stub.responses[method]
			stub.mu.RUnlock()
			if !ok {
				return socket.NewErrorResponse(req.ID, -32601, "method not found"), nil
			}

			return socket.NewResultResponse(req.ID, payload)
		})
	}

	ctx, cancel := context.WithCancel(context.Background())
	startErr := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		startErr <- stub.srv.Start(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	// Wait for socket file.
	for range 100 {
		select {
		case err := <-startErr:
			if err != nil {
				t.Fatalf("stub socket Start: %v (path %s)", err, path)
			}
		default:
		}
		if _, err := os.Stat(path); err == nil {
			return stub
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("stub socket %s never appeared", path)

	return nil
}

// SetResponse overrides the canned response for a single method.
func (s *stubSocketServer) SetResponse(method string, payload any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.responses[method] = payload
}

// SetError makes a method respond with a JSON-RPC error envelope.
func (s *stubSocketServer) SetError(method string, code int, message string) {
	s.srv.Handle(method, func(_ context.Context, req *socket.Request) (*socket.Response, error) {
		return socket.NewErrorResponse(req.ID, code, message), nil
	})
}
