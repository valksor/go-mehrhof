package commands

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"github.com/valksor/kvelmo/pkg/meta"
	"github.com/valksor/kvelmo/pkg/socket"
)

// isTestBinary reports whether the current process is a Go test binary.
// Used to prevent subprocess spawning during tests (which would re-execute
// the test binary itself, causing a fork bomb).
func isTestBinary() bool {
	return flag.Lookup("test.v") != nil
}

// ensureWorktreeSocket checks if the worktree socket is running and auto-starts
// it if missing. Returns the socket path. This provides a seamless experience
// where users don't need to manually run 'kvelmo start' before other commands.
func ensureWorktreeSocket() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	wtPath := socket.WorktreeSocketPath(cwd)
	if socket.SocketExists(wtPath) {
		return wtPath, nil
	}

	// Don't spawn subprocesses from test binaries — os.Executable() returns
	// the test binary path, which would fork-bomb by re-executing itself.
	if isTestBinary() {
		return "", fmt.Errorf("no worktree socket running\nRun '%s start' first", meta.Name)
	}

	// Socket not running — try to auto-start in background
	slog.Info("worktree socket not running, auto-starting...")

	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("no worktree socket running\nRun '%s start' first", meta.Name)
	}

	bgCmd := exec.Command(exe, "start", "--foreground") //nolint:noctx // Detached process must outlive caller
	bgCmd.Dir = cwd
	bgCmd.Stdout = nil
	bgCmd.Stderr = nil

	if err := bgCmd.Start(); err != nil {
		return "", fmt.Errorf("no worktree socket running\nRun '%s start' first", meta.Name)
	}

	// Wait for socket to become available
	if !waitForSocket(wtPath, 10*time.Second) {
		return "", fmt.Errorf("socket auto-start failed\nRun '%s start' first", meta.Name)
	}

	fmt.Fprintf(os.Stderr, "Auto-started %s (PID %d)\n", meta.Name, bgCmd.Process.Pid)

	return wtPath, nil
}
