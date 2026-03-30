package commands

import (
	"strings"
	"testing"
	"time"

	"github.com/valksor/kvelmo/pkg/meta"
	"github.com/valksor/kvelmo/pkg/socket"
)

func TestWorkersCommand_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())

	err := runWorkers(WorkersCmd, nil)
	if err == nil {
		t.Fatal("runWorkers() expected error when no socket running, got nil")
	}
	if !strings.Contains(err.Error(), "not running") {
		t.Errorf("runWorkers() error = %q, want 'not running'", err.Error())
	}
}

func TestWorkersCommand_EmptyPool(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv(meta.EnvPrefix+"_HOME", tmpDir)

	sockPath := socket.GlobalSocketPath()

	global := socket.NewGlobalSocket(sockPath)
	ctx := t.Context()

	go func() { _ = global.Start(ctx) }()
	time.Sleep(50 * time.Millisecond)

	if !socket.SocketExists(sockPath) {
		t.Fatal("global socket not created")
	}

	if err := runWorkers(WorkersCmd, nil); err != nil {
		t.Errorf("runWorkers() error = %v, want nil", err)
	}
}
