package socket

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBaseDir(t *testing.T) {
	t.Parallel()

	got := BaseDir()
	if got == "" {
		t.Error("BaseDir() returned empty string")
	}
	if !filepath.IsAbs(got) {
		t.Errorf("BaseDir() = %q, want absolute path", got)
	}
}

func TestGlobalSocketPath(t *testing.T) {
	t.Parallel()

	got := GlobalSocketPath()
	if got == "" {
		t.Error("GlobalSocketPath() returned empty string")
	}
	if !strings.HasSuffix(got, "global.sock") {
		t.Errorf("GlobalSocketPath() = %q, want path ending in global.sock", got)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("GlobalSocketPath() = %q, want absolute path", got)
	}
}

func TestGlobalLockPath_Format(t *testing.T) {
	t.Parallel()

	got := GlobalLockPath()
	if got == "" {
		t.Error("GlobalLockPath() returned empty string")
	}
	if !strings.HasSuffix(got, ".lock") {
		t.Errorf("GlobalLockPath() = %q, want path ending in .lock", got)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("GlobalLockPath() = %q, want absolute path", got)
	}
}

func TestWorktreeSocketPath_Variants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dir  string
	}{
		{name: "absolute path", dir: "/home/user/project"},
		{name: "another project", dir: "/home/user/other"},
		{name: "nested path", dir: "/var/projects/myapp/worktree"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := WorktreeSocketPath(tt.dir)
			if got == "" {
				t.Errorf("WorktreeSocketPath(%q) returned empty string", tt.dir)
			}
			if !filepath.IsAbs(got) {
				t.Errorf("WorktreeSocketPath(%q) = %q, want absolute path", tt.dir, got)
			}
			// Path should end with .sock
			if !strings.HasSuffix(got, ".sock") {
				t.Errorf("WorktreeSocketPath(%q) = %q, want path ending in .sock", tt.dir, got)
			}
			// Path should contain "worktrees" component
			if !strings.Contains(got, "worktrees") {
				t.Errorf("WorktreeSocketPath(%q) = %q, want path containing 'worktrees'", tt.dir, got)
			}
		})
	}
}

func TestWorktreeSocketPath_SameInputSameOutput(t *testing.T) {
	t.Parallel()

	dir := "/home/user/myproject"

	path1 := WorktreeSocketPath(dir)
	path2 := WorktreeSocketPath(dir)

	if path1 != path2 {
		t.Errorf("WorktreeSocketPath() not deterministic: %q != %q", path1, path2)
	}
}

func TestWorktreeSocketPath_DifferentInputsDifferentPaths(t *testing.T) {
	t.Parallel()

	path1 := WorktreeSocketPath("/project/alpha")
	path2 := WorktreeSocketPath("/project/beta")

	if path1 == path2 {
		t.Error("Different inputs should produce different WorktreeSocketPath values")
	}
}

func TestWorktreeLockPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dir  string
	}{
		{name: "absolute project path", dir: "/home/user/project"},
		{name: "another project", dir: "/opt/work/service"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := WorktreeLockPath(tt.dir)
			if got == "" {
				t.Errorf("WorktreeLockPath(%q) returned empty string", tt.dir)
			}
			if !strings.HasSuffix(got, ".lock") {
				t.Errorf("WorktreeLockPath(%q) = %q, want path ending in .lock", tt.dir, got)
			}
			// The sock path without .sock suffix should match the lock path without .lock suffix
			sockPath := WorktreeSocketPath(tt.dir)
			wantBase := strings.TrimSuffix(sockPath, ".sock")
			wantLock := wantBase + ".lock"
			if got != wantLock {
				t.Errorf("WorktreeLockPath(%q) = %q, want %q", tt.dir, got, wantLock)
			}
		})
	}
}

func TestWorktreeLockPath_TransformsSockToLock(t *testing.T) {
	t.Parallel()

	dir := "/home/user/someproject"
	lockPath := WorktreeLockPath(dir)
	sockPath := WorktreeSocketPath(dir)

	// They should share the same base (without extension)
	sockBase := strings.TrimSuffix(sockPath, ".sock")
	lockBase := strings.TrimSuffix(lockPath, ".lock")

	if sockBase != lockBase {
		t.Errorf("WorktreeLockPath base %q does not match WorktreeSocketPath base %q", lockBase, sockBase)
	}
}

func TestWorktreeIDFromPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dir  string
	}{
		{name: "absolute path", dir: "/home/user/project"},
		{name: "another absolute path", dir: "/var/projects/service"},
		{name: "deep path", dir: "/home/user/.local/src/myapp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := WorktreeIDFromPath(tt.dir)
			if got == "" {
				t.Errorf("WorktreeIDFromPath(%q) returned empty string", tt.dir)
			}
			// Should be 16 hex characters (8 bytes hex-encoded)
			if len(got) != 16 {
				t.Errorf("WorktreeIDFromPath(%q) = %q, want 16-char hex string (got %d chars)", tt.dir, got, len(got))
			}
		})
	}
}

func TestWorktreeIDFromPath_Deterministic(t *testing.T) {
	t.Parallel()

	dir := "/home/user/stable-project"

	id1 := WorktreeIDFromPath(dir)
	id2 := WorktreeIDFromPath(dir)

	if id1 != id2 {
		t.Errorf("WorktreeIDFromPath() not deterministic: %q != %q", id1, id2)
	}
}

func TestWorktreeIDFromPath_DifferentInputsDifferentIDs(t *testing.T) {
	t.Parallel()

	id1 := WorktreeIDFromPath("/project/one")
	id2 := WorktreeIDFromPath("/project/two")

	if id1 == id2 {
		t.Error("Different inputs should produce different WorktreeIDFromPath values")
	}
}

func TestSocketExists_NonExistentPath(t *testing.T) {
	t.Parallel()

	got := SocketExists("/this/path/does/not/exist/socket.sock")
	if got {
		t.Error("SocketExists() = true for non-existent path, want false")
	}
}

func TestSocketExists_RegularFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	regularFile := filepath.Join(dir, "regular.txt")

	if err := os.WriteFile(regularFile, []byte("content"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	got := SocketExists(regularFile)
	if got {
		t.Error("SocketExists() = true for regular file, want false")
	}
}

func TestSocketExists_ActualSocket(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")

	lc := net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "unix", sockPath)
	if err != nil {
		t.Fatalf("net.Listen error = %v", err)
	}
	defer func() { _ = ln.Close() }()

	got := SocketExists(sockPath)
	if !got {
		t.Errorf("SocketExists() = false for actual unix socket at %q, want true", sockPath)
	}
}

func TestSocketExists_Directory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	got := SocketExists(dir)
	if got {
		t.Error("SocketExists() = true for directory, want false")
	}
}
