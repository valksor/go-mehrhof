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

func TestStatusCommand_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())

	err := runStatus(StatusCmd, nil)
	if err == nil {
		t.Fatal("runStatus() expected error when no socket running, got nil")
	}
	if !strings.Contains(err.Error(), "no worktree socket") {
		t.Errorf("runStatus() error = %q, want 'no worktree socket'", err.Error())
	}
}

func TestStatusCommand_WithSocket(t *testing.T) {
	// Use testutil.TempDir for short paths - Unix sockets are limited to ~108 chars
	// The socket path is KVELMO_HOME/worktrees/<hash>.sock
	tmpDir := testutil.TempDir(t)
	t.Setenv(meta.EnvPrefix+"_HOME", tmpDir)

	// runStatus uses os.Getwd() to find the socket, so we must create
	// a socket for the actual working directory
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	sockPath := socket.WorktreeSocketPath(cwd)

	// Start a worktree socket
	wt := socket.NewWorktreeSocketSimple(sockPath, cwd)
	ctx := t.Context()

	// Channel to capture start error
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
			// Start returned nil - socket should exist, run test before returning
			if !socket.SocketExists(sockPath) {
				t.Fatalf("socket Start() returned nil but socket does not exist: %s", sockPath)
			}
			if err := runStatus(StatusCmd, nil); err != nil {
				t.Errorf("runStatus() error = %v, want nil", err)
			}

			return
		case <-deadline:
			t.Fatalf("socket not created within timeout; path=%s", sockPath)
		default:
			if socket.SocketExists(sockPath) {
				// Socket exists, run the test
				if err := runStatus(StatusCmd, nil); err != nil {
					t.Errorf("runStatus() error = %v, want nil", err)
				}

				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}
