package commands

import (
	"strings"
	"testing"
	"time"

	"github.com/valksor/kvelmo/pkg/meta"
	"github.com/valksor/kvelmo/pkg/socket"
	"github.com/valksor/kvelmo/pkg/testutil"
)

func TestListCommand_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", testutil.TempDir(t))

	err := runList(ListCmd, nil)
	if err == nil {
		t.Fatal("runList() expected error when no socket running, got nil")
	}
	// Check for either old or new error message format
	errMsg := err.Error()
	if !strings.Contains(errMsg, "not running") && !strings.Contains(errMsg, "socket not found") && !strings.Contains(errMsg, "socket not responding") {
		t.Errorf("runList() error = %q, want socket connection error", errMsg)
	}
}

func TestListCommand_EmptyProjects(t *testing.T) {
	// Use testutil.TempDir for short paths - Unix sockets are limited to ~108 chars
	tmpDir := testutil.TempDir(t)
	t.Setenv(meta.EnvPrefix+"_HOME", tmpDir)

	sockPath := socket.GlobalSocketPath()

	global := socket.NewGlobalSocket(sockPath)
	ctx := t.Context()

	// Channel to capture start error
	startErr := make(chan error, 1)
	go func() { startErr <- global.Start(ctx) }()

	// Wait for socket to start (with timeout)
	deadline := time.After(500 * time.Millisecond)
	for {
		select {
		case err := <-startErr:
			if err != nil && ctx.Err() == nil {
				t.Fatalf("global socket Start() error: %v", err)
			}
			// Start returned nil - socket should exist, run test before returning
			if !socket.SocketExists(sockPath) {
				t.Fatalf("socket Start() returned nil but socket does not exist: %s", sockPath)
			}
			if err := runList(ListCmd, nil); err != nil {
				t.Errorf("runList() error = %v, want nil", err)
			}

			return
		case <-deadline:
			t.Fatalf("global socket not created within timeout; path=%s", sockPath)
		default:
			if socket.SocketExists(sockPath) {
				// Socket exists, run the test
				if err := runList(ListCmd, nil); err != nil {
					t.Errorf("runList() error = %v, want nil", err)
				}

				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}
