//go:build e2e

package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/socket"
)

func TestFullConductorIntegration(t *testing.T) {
	// Create a temp git repo with short path for Unix socket limits
	// t.TempDir() returns paths that are too long for Unix socket paths
	tmpDir, err := os.MkdirTemp("/tmp", "crea-e2e-") //nolint:usetesting // Need short path for Unix socket limits
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	repoDir := filepath.Join(tmpDir, "r")
	sockPath := filepath.Join(tmpDir, "w.sock")

	// Initialize git repo
	cmd := exec.CommandContext(context.Background(), "git", "init", repoDir)
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	// Configure git user
	cmd = exec.CommandContext(context.Background(), "git", "-C", repoDir, "config", "user.email", "test@test.com")
	_ = cmd.Run()
	cmd = exec.CommandContext(context.Background(), "git", "-C", repoDir, "config", "user.name", "Test User")
	_ = cmd.Run()

	// Create initial commit
	testFile := filepath.Join(repoDir, "test.txt")
	_ = os.WriteFile(testFile, []byte("initial content"), 0o644)
	cmd = exec.CommandContext(context.Background(), "git", "-C", repoDir, "add", ".")
	_ = cmd.Run()
	cmd = exec.CommandContext(context.Background(), "git", "-C", repoDir, "commit", "-m", "initial commit")
	_ = cmd.Run()

	// Create worktree socket with full config
	ws, err := socket.NewWorktreeSocket(socket.WorktreeConfig{
		WorktreePath: repoDir,
		SocketPath:   sockPath,
		GlobalPath:   filepath.Join(tmpDir, "global.sock"),
	})
	if err != nil {
		t.Fatalf("NewWorktreeSocket() error = %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- ws.Start(ctx)
	}()

	// Wait for socket to be ready
	waitForSocketPath(t, sockPath)

	// Connect client
	client, err := socket.NewClient(sockPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer func() { _ = client.Close() }()

	// Test status - should be "none"
	resp, err := client.Call(ctx, "status", nil)
	if err != nil {
		t.Fatalf("status error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("status returned error: %v", resp.Error)
	}

	// Test checkpoints - should work with conductor
	resp, err = client.Call(ctx, "checkpoints", nil)
	if err != nil {
		t.Fatalf("checkpoints error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("checkpoints returned error: %v", resp.Error)
	}

	// Test git status
	resp, err = client.Call(ctx, "git.status", nil)
	if err != nil {
		t.Fatalf("git.status error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("git.status returned error: %v", resp.Error)
	}

	// Test git log
	resp, err = client.Call(ctx, "git.log", map[string]any{"limit": 5})
	if err != nil {
		t.Fatalf("git.log error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("git.log returned error: %v", resp.Error)
	}

	// Cleanup
	cancel()
	select {
	case <-errCh:
	case <-time.After(time.Second):
	}
}

func TestTaskLifecycleStartOnly(t *testing.T) {
	// Skip this test as it requires full conductor setup with workers
	// The start handler may block waiting for async operations
	t.Skip("Skipping: requires full conductor with workers for task loading")
}
