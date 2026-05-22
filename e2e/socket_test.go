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

// socketWaitTimeout bounds how long waitForSocketPath polls for a socket file.
const socketWaitTimeout = 2 * time.Second

// waitForSocketPath polls until the socket file exists or the timeout is reached.
func waitForSocketPath(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(socketWaitTimeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("socket %s not ready after %v", path, socketWaitTimeout)
}

func TestGlobalSocketE2E(t *testing.T) {
	// Create a temp directory for the socket
	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "global.sock")

	// Create and start global socket
	gs := socket.NewGlobalSocket(sockPath)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- gs.Start(ctx)
	}()

	// Wait for socket to be ready
	waitForSocketPath(t, sockPath)

	// Connect client
	client, err := socket.NewClient(sockPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer func() { _ = client.Close() }()

	// Test ping
	resp, err := client.Call(ctx, "ping", nil)
	if err != nil {
		t.Fatalf("ping error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("ping returned error: %v", resp.Error)
	}

	// Test list projects (should be empty)
	resp, err = client.Call(ctx, "projects.list", nil)
	if err != nil {
		t.Fatalf("projects.list error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("projects.list returned error: %v", resp.Error)
	}

	// Test register project
	regParams := map[string]any{
		"path": "/tmp/test-project",
	}
	resp, err = client.Call(ctx, "projects.register", regParams)
	if err != nil {
		t.Fatalf("projects.register error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("projects.register returned error: %v", resp.Error)
	}

	// Test list projects (should have one)
	resp, err = client.Call(ctx, "projects.list", nil)
	if err != nil {
		t.Fatalf("projects.list error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("projects.list returned error: %v", resp.Error)
	}

	// Test worker stats
	resp, err = client.Call(ctx, "workers.stats", nil)
	if err != nil {
		t.Fatalf("workers.stats error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("workers.stats returned error: %v", resp.Error)
	}

	// Cleanup
	cancel()
	select {
	case <-errCh:
	case <-time.After(time.Second):
	}
}

func TestWorktreeSocketE2E(t *testing.T) {
	// Create a temp git repo
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	sockPath := filepath.Join(tmpDir, "worktree.sock")

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
	_ = os.WriteFile(testFile, []byte("initial"), 0o644)
	cmd = exec.CommandContext(context.Background(), "git", "-C", repoDir, "add", ".")
	_ = cmd.Run()
	cmd = exec.CommandContext(context.Background(), "git", "-C", repoDir, "commit", "-m", "initial")
	_ = cmd.Run()

	// Create worktree socket
	ws := socket.NewWorktreeSocketSimple(sockPath, repoDir)

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

	// Test status
	resp, err := client.Call(ctx, "status", nil)
	if err != nil {
		t.Fatalf("status error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("status returned error: %v", resp.Error)
	}

	// Test git.status
	resp, err = client.Call(ctx, "git.status", nil)
	if err != nil {
		t.Fatalf("git.status error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("git.status returned error: %v", resp.Error)
	}

	// Test git.log
	resp, err = client.Call(ctx, "git.log", map[string]any{"limit": 10})
	if err != nil {
		t.Fatalf("git.log error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("git.log returned error: %v", resp.Error)
	}

	// Note: checkpoints requires conductor, which is not set in simple mode
	// Testing git.diff instead
	resp, err = client.Call(ctx, "git.diff", nil)
	if err != nil {
		t.Fatalf("git.diff error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("git.diff returned error: %v", resp.Error)
	}

	// Cleanup
	cancel()
	select {
	case <-errCh:
	case <-time.After(time.Second):
	}
}

func TestSocketCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "test.sock")

	// Create a stale socket file
	_ = os.WriteFile(sockPath, []byte{}, 0o644)

	// CleanupStaleSocket should remove it
	_, _ = socket.CleanupStaleSocket(sockPath)

	// Verify it's gone
	if _, err := os.Stat(sockPath); !os.IsNotExist(err) {
		t.Error("stale socket should have been removed")
	}
}

func TestConcurrentClients(t *testing.T) {
	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "global.sock")

	gs := socket.NewGlobalSocket(sockPath)

	ctx := t.Context()

	go func() { _ = gs.Start(ctx) }()
	waitForSocketPath(t, sockPath)

	// Connect multiple clients concurrently
	const numClients = 5
	results := make(chan error, numClients)

	for range numClients {
		go func() {
			client, err := socket.NewClient(sockPath, socket.WithTimeout(5*time.Second))
			if err != nil {
				results <- err

				return
			}
			defer func() { _ = client.Close() }()

			_, err = client.Call(ctx, "ping", nil)
			results <- err
		}()
	}

	// Check all clients succeeded
	for i := range numClients {
		if err := <-results; err != nil {
			t.Errorf("client %d failed: %v", i, err)
		}
	}
}

func TestProtocolErrors(t *testing.T) {
	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "global.sock")

	gs := socket.NewGlobalSocket(sockPath)

	ctx := t.Context()

	go func() { _ = gs.Start(ctx) }()
	waitForSocketPath(t, sockPath)

	client, err := socket.NewClient(sockPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer func() { _ = client.Close() }()

	// Test unknown method - should return an error (either from Call or in resp.Error)
	resp, err := client.Call(ctx, "nonexistent.method", nil)

	// Either the call returns an error, or the response has an error
	hasError := err != nil || (resp != nil && resp.Error != nil)
	if !hasError {
		t.Error("expected error for unknown method")
	}
}
