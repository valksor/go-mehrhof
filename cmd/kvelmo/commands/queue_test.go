package commands

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/socket"
	"github.com/valksor/kvelmo/internal/testutil"
	"github.com/valksor/kvelmo/meta"
)

func TestQueueCommand(t *testing.T) {
	if QueueCmd.Use != "queue" {
		t.Errorf("Use = %s, want queue", QueueCmd.Use)
	}
	if QueueCmd.Short == "" {
		t.Error("Short description should exist")
	}
}

func TestQueueSubcommands(t *testing.T) {
	subs := QueueCmd.Commands()
	subNames := make(map[string]bool)
	for _, sub := range subs {
		subNames[sub.Name()] = true
	}
	for _, name := range []string{"add", "remove", "list", "reorder"} {
		if !subNames[name] {
			t.Errorf("missing subcommand %q", name)
		}
	}
}

func TestQueueAddCommand(t *testing.T) {
	if queueAddCmd.Use != "add <source>" {
		t.Errorf("Use = %s, want add <source>", queueAddCmd.Use)
	}
	if f := queueAddCmd.Flags().Lookup("title"); f == nil {
		t.Error("--title flag should exist")
	}
}

func TestQueueRemoveCommand(t *testing.T) {
	if queueRemoveCmd.Use != "remove <id>" {
		t.Errorf("Use = %s, want remove <id>", queueRemoveCmd.Use)
	}
}

func TestQueueListCommand(t *testing.T) {
	if queueListCmd.Use != "list" {
		t.Errorf("Use = %s, want list", queueListCmd.Use)
	}
	if f := queueListCmd.Flags().Lookup("json"); f == nil {
		t.Error("--json flag should exist")
	}
}

func TestQueueReorderCommand(t *testing.T) {
	if queueReorderCmd.Use != "reorder <id> <position>" {
		t.Errorf("Use = %s, want reorder <id> <position>", queueReorderCmd.Use)
	}
}

func TestRunQueueAdd_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())

	err := runQueueAdd(queueAddCmd, []string{"https://github.com/org/repo/issues/1"})
	if err == nil {
		t.Fatal("runQueueAdd() expected error when no socket running, got nil")
	}
	if !strings.Contains(err.Error(), "no worktree socket") {
		t.Errorf("runQueueAdd() error = %q, want 'no worktree socket'", err.Error())
	}
}

func TestRunQueueRemove_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())

	err := runQueueRemove(queueRemoveCmd, []string{"task-123"})
	if err == nil {
		t.Fatal("runQueueRemove() expected error when no socket running, got nil")
	}
	if !strings.Contains(err.Error(), "no worktree socket") {
		t.Errorf("runQueueRemove() error = %q, want 'no worktree socket'", err.Error())
	}
}

func TestRunQueueList_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())

	err := runQueueList(queueListCmd, nil)
	if err == nil {
		t.Fatal("runQueueList() expected error when no socket running, got nil")
	}
	if !strings.Contains(err.Error(), "no worktree socket") {
		t.Errorf("runQueueList() error = %q, want 'no worktree socket'", err.Error())
	}
}

func TestRunQueueReorder_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())

	err := runQueueReorder(queueReorderCmd, []string{"task-123", "2"})
	if err == nil {
		t.Fatal("runQueueReorder() expected error when no socket running, got nil")
	}
	if !strings.Contains(err.Error(), "no worktree socket") {
		t.Errorf("runQueueReorder() error = %q, want 'no worktree socket'", err.Error())
	}
}

func TestRunQueueReorder_InvalidPosition(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())

	err := runQueueReorder(queueReorderCmd, []string{"task-123", "not-a-number"})
	if err == nil {
		t.Fatal("runQueueReorder() expected error for invalid position, got nil")
	}
	if !strings.Contains(err.Error(), "invalid position") {
		t.Errorf("runQueueReorder() error = %q, want 'invalid position'", err.Error())
	}
}

func TestRunQueueAdd_WithSocket(t *testing.T) {
	tmpDir := testutil.TempDir(t)
	t.Setenv(meta.EnvPrefix+"_HOME", tmpDir)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	sockPath := socket.WorktreeSocketPath(cwd)

	wt := socket.NewWorktreeSocketSimple(sockPath, cwd)
	ctx := t.Context()

	startErr := make(chan error, 1)
	go func() { startErr <- wt.Start(ctx) }()

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

			err = runQueueAdd(queueAddCmd, []string{"https://github.com/org/repo/issues/1"})
			if err == nil {
				return
			}
			if strings.Contains(err.Error(), "no worktree socket") {
				t.Errorf("runQueueAdd() got 'no worktree socket' error with socket running: %v", err)
			}

			return
		case <-deadline:
			t.Fatalf("socket not created within timeout; path=%s", sockPath)
		default:
			if socket.SocketExists(sockPath) {
				err = runQueueAdd(queueAddCmd, []string{"https://github.com/org/repo/issues/1"})
				if err == nil {
					return
				}
				if strings.Contains(err.Error(), "no worktree socket") {
					t.Errorf("runQueueAdd() got 'no worktree socket' error with socket running: %v", err)
				}

				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestRunQueueRemove_WithSocket(t *testing.T) {
	tmpDir := testutil.TempDir(t)
	t.Setenv(meta.EnvPrefix+"_HOME", tmpDir)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	sockPath := socket.WorktreeSocketPath(cwd)

	wt := socket.NewWorktreeSocketSimple(sockPath, cwd)
	ctx := t.Context()

	startErr := make(chan error, 1)
	go func() { startErr <- wt.Start(ctx) }()

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

			err = runQueueRemove(queueRemoveCmd, []string{"task-123"})
			if err == nil {
				return
			}
			if strings.Contains(err.Error(), "no worktree socket") {
				t.Errorf("runQueueRemove() got 'no worktree socket' error with socket running: %v", err)
			}

			return
		case <-deadline:
			t.Fatalf("socket not created within timeout; path=%s", sockPath)
		default:
			if socket.SocketExists(sockPath) {
				err = runQueueRemove(queueRemoveCmd, []string{"task-123"})
				if err == nil {
					return
				}
				if strings.Contains(err.Error(), "no worktree socket") {
					t.Errorf("runQueueRemove() got 'no worktree socket' error with socket running: %v", err)
				}

				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestRunQueueList_WithSocket(t *testing.T) {
	tmpDir := testutil.TempDir(t)
	t.Setenv(meta.EnvPrefix+"_HOME", tmpDir)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	sockPath := socket.WorktreeSocketPath(cwd)

	wt := socket.NewWorktreeSocketSimple(sockPath, cwd)
	ctx := t.Context()

	startErr := make(chan error, 1)
	go func() { startErr <- wt.Start(ctx) }()

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

			err = runQueueList(queueListCmd, nil)
			if err == nil {
				return
			}
			if strings.Contains(err.Error(), "no worktree socket") {
				t.Errorf("runQueueList() got 'no worktree socket' error with socket running: %v", err)
			}

			return
		case <-deadline:
			t.Fatalf("socket not created within timeout; path=%s", sockPath)
		default:
			if socket.SocketExists(sockPath) {
				err = runQueueList(queueListCmd, nil)
				if err == nil {
					return
				}
				if strings.Contains(err.Error(), "no worktree socket") {
					t.Errorf("runQueueList() got 'no worktree socket' error with socket running: %v", err)
				}

				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestRunQueueReorder_WithSocket(t *testing.T) {
	tmpDir := testutil.TempDir(t)
	t.Setenv(meta.EnvPrefix+"_HOME", tmpDir)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	sockPath := socket.WorktreeSocketPath(cwd)

	wt := socket.NewWorktreeSocketSimple(sockPath, cwd)
	ctx := t.Context()

	startErr := make(chan error, 1)
	go func() { startErr <- wt.Start(ctx) }()

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

			err = runQueueReorder(queueReorderCmd, []string{"task-123", "2"})
			if err == nil {
				return
			}
			if strings.Contains(err.Error(), "no worktree socket") {
				t.Errorf("runQueueReorder() got 'no worktree socket' error with socket running: %v", err)
			}

			return
		case <-deadline:
			t.Fatalf("socket not created within timeout; path=%s", sockPath)
		default:
			if socket.SocketExists(sockPath) {
				err = runQueueReorder(queueReorderCmd, []string{"task-123", "2"})
				if err == nil {
					return
				}
				if strings.Contains(err.Error(), "no worktree socket") {
					t.Errorf("runQueueReorder() got 'no worktree socket' error with socket running: %v", err)
				}

				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}
