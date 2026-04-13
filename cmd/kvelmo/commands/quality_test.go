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

func TestQualityRespondCommand_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())

	// Set required flags
	_ = qualityRespondCmd.Flags().Set("prompt-id", "test-prompt")
	_ = qualityRespondCmd.Flags().Set("yes", "true")
	_ = qualityRespondCmd.Flags().Set("no", "false")

	err := runQualityRespond(qualityRespondCmd, nil)
	if err == nil {
		t.Fatal("runQualityRespond() expected error when no socket running, got nil")
	}
	if !strings.Contains(err.Error(), "no worktree socket") {
		t.Errorf("runQualityRespond() error = %q, want 'no worktree socket'", err.Error())
	}
}

func TestQualityRespondCommand_MustSpecifyYesOrNo(t *testing.T) {
	tmpDir := testutil.TempDir(t)
	t.Setenv(meta.EnvPrefix+"_HOME", tmpDir)

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
			runMustSpecifyTest(t)

			return
		case <-deadline:
			t.Fatalf("socket not created within timeout; path=%s", sockPath)
		default:
			if socket.SocketExists(sockPath) {
				runMustSpecifyTest(t)

				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func runMustSpecifyTest(t *testing.T) {
	t.Helper()

	// Reset flags to neither yes nor no
	_ = qualityRespondCmd.Flags().Set("prompt-id", "test-prompt")
	_ = qualityRespondCmd.Flags().Set("yes", "false")
	_ = qualityRespondCmd.Flags().Set("no", "false")

	err := runQualityRespond(qualityRespondCmd, nil)
	if err == nil {
		t.Fatal("runQualityRespond() expected error when neither --yes nor --no specified, got nil")
	}
	if !strings.Contains(err.Error(), "must specify --yes or --no") {
		t.Errorf("runQualityRespond() error = %q, want 'must specify --yes or --no'", err.Error())
	}
}

func TestQualityRespondCommand_BothYesAndNo(t *testing.T) {
	tmpDir := testutil.TempDir(t)
	t.Setenv(meta.EnvPrefix+"_HOME", tmpDir)

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
			runBothYesNoTest(t)

			return
		case <-deadline:
			t.Fatalf("socket not created within timeout; path=%s", sockPath)
		default:
			if socket.SocketExists(sockPath) {
				runBothYesNoTest(t)

				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func runBothYesNoTest(t *testing.T) {
	t.Helper()

	_ = qualityRespondCmd.Flags().Set("prompt-id", "test-prompt")
	_ = qualityRespondCmd.Flags().Set("yes", "true")
	_ = qualityRespondCmd.Flags().Set("no", "true")

	err := runQualityRespond(qualityRespondCmd, nil)
	if err == nil {
		t.Fatal("runQualityRespond() expected error when both --yes and --no specified, got nil")
	}
	if !strings.Contains(err.Error(), "cannot specify both --yes and --no") {
		t.Errorf("runQualityRespond() error = %q, want 'cannot specify both --yes and --no'", err.Error())
	}
}

func TestQualityRespondCommand_WithSocket(t *testing.T) {
	tmpDir := testutil.TempDir(t)
	t.Setenv(meta.EnvPrefix+"_HOME", tmpDir)

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
			runQualityWithSocketTest(t)

			return
		case <-deadline:
			t.Fatalf("socket not created within timeout; path=%s", sockPath)
		default:
			if socket.SocketExists(sockPath) {
				runQualityWithSocketTest(t)

				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func runQualityWithSocketTest(t *testing.T) {
	t.Helper()

	// Set flags for a valid request
	_ = qualityRespondCmd.Flags().Set("prompt-id", "nonexistent-prompt")
	_ = qualityRespondCmd.Flags().Set("yes", "true")
	_ = qualityRespondCmd.Flags().Set("no", "false")

	err := runQualityRespond(qualityRespondCmd, nil)
	if err == nil {
		return // If somehow it succeeded, that's fine
	}
	// Should NOT be "no worktree socket" since socket is running
	if strings.Contains(err.Error(), "no worktree socket") {
		t.Errorf("runQualityRespond() got 'no worktree socket' error with socket running: %v", err)
	}
}
