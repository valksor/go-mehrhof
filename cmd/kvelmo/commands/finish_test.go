package commands

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/valksor/kvelmo/pkg/meta"
	"github.com/valksor/kvelmo/pkg/socket"
	"github.com/valksor/kvelmo/pkg/testutil"
)

func TestFinishCommand_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())

	err := runFinish(FinishCmd, nil)
	if err == nil {
		t.Fatal("runFinish() expected error when no socket running, got nil")
	}
	if !strings.Contains(err.Error(), "no worktree socket") {
		t.Errorf("runFinish() error = %q, want 'no worktree socket'", err.Error())
	}
}

func TestFinishCommand_WithSocket(t *testing.T) {
	// Use testutil.TempDir for short paths - Unix sockets are limited to ~108 chars
	tmpDir := testutil.TempDir(t)
	t.Setenv(meta.EnvPrefix+"_HOME", tmpDir)

	// runFinish uses os.Getwd() to find the socket, so we must create
	// a socket for the actual working directory
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	sockPath := socket.WorktreeSocketPath(cwd)

	// Start a worktree socket
	wt := socket.NewWorktreeSocketSimple(sockPath, cwd)
	ctx := t.Context()

	startErr := make(chan error, 1)
	go func() { startErr <- wt.Start(ctx) }()

	// Wait for socket to start (with timeout)
	deadline := time.After(500 * time.Millisecond)
	for {
		select {
		case err := <-startErr:
			if err != nil && ctx.Err() == nil {
				t.Fatalf("socket Start() error: %v", err)
			}
			if !socket.SocketExists(sockPath) {
				t.Fatalf("socket Start() returned nil but socket does not exist: %s", sockPath)
			}
			// finish will try to call task.finish RPC, which the simple socket doesn't handle
			// so we expect an RPC error, not a "no socket" error
			err = runFinish(FinishCmd, nil)
			if err == nil {
				return // Unexpected success is fine
			}
			// Should NOT be "no worktree socket" error since socket is running
			if strings.Contains(err.Error(), "no worktree socket") {
				t.Errorf("runFinish() got 'no worktree socket' error with socket running: %v", err)
			}

			return
		case <-deadline:
			t.Fatalf("socket not created within timeout; path=%s", sockPath)
		default:
			if socket.SocketExists(sockPath) {
				err = runFinish(FinishCmd, nil)
				if err == nil {
					return
				}
				if strings.Contains(err.Error(), "no worktree socket") {
					t.Errorf("runFinish() got 'no worktree socket' error with socket running: %v", err)
				}

				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestRefreshCommand_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())

	err := runRefresh(RefreshCmd, nil)
	if err == nil {
		t.Fatal("runRefresh() expected error when no socket running, got nil")
	}
	if !strings.Contains(err.Error(), "no worktree socket") {
		t.Errorf("runRefresh() error = %q, want 'no worktree socket'", err.Error())
	}
}
