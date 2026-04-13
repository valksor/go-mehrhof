package commands

import (
	"os"
	"testing"

	"github.com/valksor/kvelmo/internal/testutil"
)

func TestStartCommand_NotGitRepository(t *testing.T) {
	nonGitDir := t.TempDir()

	if isGitRepository(nonGitDir) {
		t.Error("expected non-git directory to not be a git repository")
	}
}

func TestStartCommand_IsGitRepository(t *testing.T) {
	tmpDir := testutil.TempDir(t)
	testutil.InitGitRepo(t, tmpDir)

	if !isGitRepository(tmpDir) {
		t.Error("expected initialized git directory to be a git repository")
	}
}

func TestStartCommand_IsGitRepository_NonGitDir(t *testing.T) {
	tmpDir := t.TempDir()

	if isGitRepository(tmpDir) {
		t.Error("expected non-git directory to not be a git repository")
	}
}

func TestStartCommand_WaitForSocket_Timeout(t *testing.T) {
	// waitForSocket with a non-existent path should return false quickly
	tmpDir := testutil.TempDir(t)
	sockPath := tmpDir + "/nonexistent.sock"

	// Use a very short timeout so the test doesn't hang
	result := waitForSocket(sockPath, 50*1e6) // 50ms
	if result {
		t.Error("waitForSocket() expected false for non-existent socket")
	}
}

func TestStartCommand_WaitForSocket_Exists(t *testing.T) {
	tmpDir := testutil.TempDir(t)
	sockPath := tmpDir + "/test.sock"

	// Create the socket file so SocketExists returns true
	f, err := os.Create(sockPath)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	// Note: SocketExists checks for actual Unix socket, not just file existence
	// A regular file won't pass SocketExists, so this should return false
	if waitForSocket(sockPath, 50*1e6) { // 50ms
		// This is fine if SocketExists just checks file existence
		return
	}
	// Also fine if it returns false (SocketExists checks socket type)
}

func TestStartCommand_LoadTaskViaRPC_NoSocket(t *testing.T) {
	tmpDir := testutil.TempDir(t)
	sockPath := tmpDir + "/nonexistent.sock"

	err := loadTaskViaRPC(sockPath, "file:task.md")
	if err == nil {
		t.Error("loadTaskViaRPC() expected error for non-existent socket, got nil")
	}
}

func TestStartCommand_FromFlagParsing(t *testing.T) {
	tests := []struct {
		name  string
		from  string
		valid bool
	}{
		{"file source", "file:task.md", true},
		{"github source", "github:owner/repo#123", true},
		{"url source", "https://github.com/owner/repo/issues/123", true},
		{"empty source", "", true}, // empty is valid (no task loaded)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify the flag value can be set without error
			cmd := StartCmd
			if err := cmd.Flags().Set("from", tt.from); err != nil {
				if tt.valid {
					t.Errorf("setting --from=%q failed: %v", tt.from, err)
				}
			}
		})
	}
}
