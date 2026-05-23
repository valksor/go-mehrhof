package commands

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/valksor/kvelmo/internal/socket"
)

// makeStaleSocket creates a unix socket file with no listener (a dangling
// socket file): it listens, disables unlink-on-close, then closes. The file
// remains on disk with os.ModeSocket set but nothing accepts connections, so
// isStaleSocket treats it as stale.
func makeStaleSocket(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "unix", path)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	if ul, ok := ln.(*net.UnixListener); ok {
		ul.SetUnlinkOnClose(false)
	}
	_ = ln.Close()
	if !socket.SocketExists(path) {
		t.Fatalf("stale socket file not present at %s", path)
	}
}

// TestRunCleanup_RemovesStaleSocket exercises the actual removal path with
// --force against a real dangling worktree socket file.
func TestRunCleanup_RemovesStaleSocket(t *testing.T) {
	if err := CleanupCmd.Flags().Set("force", "true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = CleanupCmd.Flags().Set("force", "false") })

	shortKvelmoHome(t)
	chdirToShortTemp(t)

	// Place a stale socket file under the worktrees dir so cleanup discovers it.
	stalePath := filepath.Join(socket.BaseDir(), "worktrees", "stale.sock")
	makeStaleSocket(t, stalePath)

	out := captureStdout(t, func() {
		if err := runCleanup(CleanupCmd, nil); err != nil {
			t.Errorf("runCleanup: %v", err)
		}
	})
	if !strings.Contains(out, "Removed") || !strings.Contains(out, "Cleaned up") {
		t.Errorf("cleanup removal output:\n%s", out)
	}
	if socket.SocketExists(stalePath) {
		t.Error("stale socket should have been removed")
	}
}

// TestStatsAll_WithWorktreeData drives runStatsAll across a project list whose
// worktree socket serves task.history (the per-project aggregation path).
func TestStatsAll_WithWorktreeData(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)

	// The worktree socket lives at the cwd-derived path; register that same
	// path as a project so runStatsAll dials it for history.
	cwd, _ := os.Getwd()
	wstub := startStubWorktreeSocket(t)
	wstub.SetResponse("task.history", map[string]any{
		"tasks": []any{
			map[string]any{"id": "t1", "title": "Done", "final_state": "finished", "started_at": "2026-05-01T10:00:00Z", "completed_at": "2026-05-01T10:30:00Z"},
		},
	})

	gstub := startStubGlobalSocket(t)
	gstub.SetResponse("tasks.list", map[string]any{
		"tasks": []any{
			map[string]any{"id": "p1", "path": cwd, "state": "implementing"},
		},
	})

	out := captureStdout(t, func() {
		if err := runStatsAll(); err != nil {
			t.Errorf("runStatsAll: %v", err)
		}
	})
	if out == "" {
		t.Error("stats all (with data) produced no output")
	}
}

// TestWorktreeJSONFlags drives the --json output branch of worktree-socket
// commands that support it.
func TestWorktreeJSONFlags(t *testing.T) {
	cases := []struct {
		name   string
		flag   *bool
		method string
		fn     func() error
	}{
		{"eventlog", &eventlogJSON, "eventlog.query", func() error { return runEventlog(EventlogCmd, nil) }},
		{"ci", &ciJSON, "ci.status", func() error { return runCIStatus(CICmd, nil) }},
		{"hooks", &hooksJSON, "hooks.list", func() error { return runHooks(HooksCmd, nil) }},
		{"policy", &policyJSON, "policy.check", func() error { return runPolicyCheck(nil, nil) }},
		{"forkcompare", &forkCompareJSON, "fork.compare", func() error { return runForkCompare(nil, nil) }},
		{"forklist", &forkListJSON, "fork.list", func() error { return runForkList(nil, nil) }},
		{"codegraph", &codegraphJSON, "codegraph.callers", func() error { return runCodegraphCallers(nil, []string{"x"}) }},
		{"taglist", &tagListJSON, "task.tag", func() error { return runTagList(tagListCmd, nil) }},
		{"checkpoints", &checkpointsJSON, "checkpoints", func() error { return runCheckpoints(CheckpointsCmd, nil) }},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			setBoolPtr(t, c.flag, true)
			shortKvelmoHome(t)
			chdirToShortTemp(t)
			_ = startStubWorktreeSocket(t)

			out := captureStdout(t, func() {
				if err := c.fn(); err != nil {
					t.Errorf("%s json: %v", c.name, err)
				}
			})
			if out == "" {
				t.Errorf("%s json produced no output", c.name)
			}
		})
	}
}

// TestGlobalJSONFlags drives the --json output branch of global-socket commands.
func TestGlobalJSONFlags(t *testing.T) {
	cases := []struct {
		name string
		flag *bool
		fn   func() error
	}{
		{"strategy", &strategyJSON, func() error { return runStrategyList(StrategyCmd, nil) }},
		{"grouplist", &groupListJSON, func() error { return runGroupList(nil, nil) }},
		{"backuplist", &backupListJSON, func() error { return runBackupList(nil, nil) }},
		{"memorystats", &memoryStatsJSON, func() error { return runMemoryStats(memoryStatsCmd, nil) }},
		{"projects", &projectsJSON, func() error { return runProjects(ProjectsCmd, nil) }},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			setBoolPtr(t, c.flag, true)
			shortKvelmoHome(t)
			// strategy.list needs a bare-array default; the stub already serves it.
			stub := startStubGlobalSocket(t)
			stub.SetResponse("strategy.list", []string{"react"})

			out := captureStdout(t, func() {
				if err := c.fn(); err != nil {
					t.Errorf("%s json: %v", c.name, err)
				}
			})
			if out == "" {
				t.Errorf("%s json produced no output", c.name)
			}
		})
	}
}
