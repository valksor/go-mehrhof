package paths

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/valksor/kvelmo/pkg/meta"
)

const testBaseDir = "/test/base"

func TestNewPathResolver(t *testing.T) {
	t.Parallel()

	baseDir := "/custom/base/dir"
	resolver := NewPathResolver(baseDir)

	if resolver.BaseDir() != baseDir {
		t.Errorf("BaseDir() = %q, want %q", resolver.BaseDir(), baseDir)
	}
}

func TestDefaultPathResolver(t *testing.T) {
	// Not parallel - uses t.Setenv which modifies global state
	t.Run("uses KVELMO_HOME env var when set", func(t *testing.T) {
		customHome := "/custom/kvelmo/home"
		t.Setenv(meta.EnvPrefix+"_HOME", customHome)

		resolver := DefaultPathResolver()

		if resolver.BaseDir() != customHome {
			t.Errorf("BaseDir() = %q, want %q", resolver.BaseDir(), customHome)
		}
	})

	t.Run("uses default path when env var not set", func(t *testing.T) {
		t.Setenv(meta.EnvPrefix+"_HOME", "")

		resolver := DefaultPathResolver()

		home, _ := os.UserHomeDir()
		expected := filepath.Join(home, meta.GlobalDir)
		if resolver.BaseDir() != expected {
			t.Errorf("BaseDir() = %q, want %q", resolver.BaseDir(), expected)
		}
	})
}

func TestPathResolver_GlobalSocketPath(t *testing.T) {
	t.Parallel()

	baseDir := testBaseDir
	resolver := NewPathResolver(baseDir)

	got := resolver.GlobalSocketPath()
	want := filepath.Join(baseDir, "global.sock")

	if got != want {
		t.Errorf("GlobalSocketPath() = %q, want %q", got, want)
	}
}

func TestPathResolver_GlobalLockPath(t *testing.T) {
	t.Parallel()

	baseDir := testBaseDir
	resolver := NewPathResolver(baseDir)

	got := resolver.GlobalLockPath()
	want := filepath.Join(baseDir, "global.lock")

	if got != want {
		t.Errorf("GlobalLockPath() = %q, want %q", got, want)
	}
}

func TestPathResolver_WorktreeSocketPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		worktreeDir string
		wantSuffix  string
	}{
		{
			name:        "absolute path",
			worktreeDir: "/Users/test/project",
			wantSuffix:  ".sock",
		},
		{
			name:        "relative path gets resolved",
			worktreeDir: "relative/path",
			wantSuffix:  ".sock",
		},
		{
			name:        "path with spaces",
			worktreeDir: "/Users/test/my project",
			wantSuffix:  ".sock",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			baseDir := testBaseDir
			resolver := NewPathResolver(baseDir)

			got := resolver.WorktreeSocketPath(tt.worktreeDir)

			// Verify structure: baseDir/worktrees/<hash>.sock
			if !strings.HasPrefix(got, filepath.Join(baseDir, "worktrees")) {
				t.Errorf("WorktreeSocketPath() = %q, should start with worktrees dir", got)
			}
			if !strings.HasSuffix(got, tt.wantSuffix) {
				t.Errorf("WorktreeSocketPath() = %q, should end with %q", got, tt.wantSuffix)
			}
		})
	}
}

func TestPathResolver_WorktreeSocketPath_Deterministic(t *testing.T) {
	t.Parallel()

	baseDir := testBaseDir
	resolver := NewPathResolver(baseDir)
	worktreeDir := "/Users/test/project"

	// Same input should produce same output
	path1 := resolver.WorktreeSocketPath(worktreeDir)
	path2 := resolver.WorktreeSocketPath(worktreeDir)

	if path1 != path2 {
		t.Errorf("WorktreeSocketPath() not deterministic: %q != %q", path1, path2)
	}
}

func TestPathResolver_WorktreeSocketPath_UniqueHashes(t *testing.T) {
	t.Parallel()

	baseDir := testBaseDir
	resolver := NewPathResolver(baseDir)

	path1 := resolver.WorktreeSocketPath("/project/a")
	path2 := resolver.WorktreeSocketPath("/project/b")

	if path1 == path2 {
		t.Error("Different worktree dirs should produce different socket paths")
	}
}

func TestPathResolver_MemoryDir(t *testing.T) {
	t.Parallel()

	baseDir := testBaseDir
	resolver := NewPathResolver(baseDir)

	got := resolver.MemoryDir()
	want := filepath.Join(baseDir, "memory")

	if got != want {
		t.Errorf("MemoryDir() = %q, want %q", got, want)
	}
}

func TestPathResolver_ConfigPath(t *testing.T) {
	t.Parallel()

	baseDir := testBaseDir
	resolver := NewPathResolver(baseDir)

	got := resolver.ConfigPath()
	want := filepath.Join(baseDir, meta.ConfigFile)

	if got != want {
		t.Errorf("ConfigPath() = %q, want %q", got, want)
	}
}

func TestPathResolver_EnvPath(t *testing.T) {
	t.Parallel()

	baseDir := testBaseDir
	resolver := NewPathResolver(baseDir)

	got := resolver.EnvPath()
	want := filepath.Join(baseDir, ".env")

	if got != want {
		t.Errorf("EnvPath() = %q, want %q", got, want)
	}
}

func TestPathResolver_EnsureDir(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	subDir := filepath.Join(baseDir, "nested", "path")
	resolver := NewPathResolver(subDir)

	err := resolver.EnsureDir()
	if err != nil {
		t.Fatalf("EnsureDir() error = %v", err)
	}

	// Verify directories exist
	if _, err := os.Stat(subDir); os.IsNotExist(err) {
		t.Error("EnsureDir() did not create base directory")
	}

	worktreesDir := filepath.Join(subDir, "worktrees")
	if _, err := os.Stat(worktreesDir); os.IsNotExist(err) {
		t.Error("EnsureDir() did not create worktrees directory")
	}
}

func TestPathResolver_EnsureDir_AlreadyExists(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	resolver := NewPathResolver(baseDir)

	// Call twice - second should not fail
	if err := resolver.EnsureDir(); err != nil {
		t.Fatalf("EnsureDir() first call error = %v", err)
	}
	if err := resolver.EnsureDir(); err != nil {
		t.Fatalf("EnsureDir() second call error = %v", err)
	}
}

func TestPaths_Injection(t *testing.T) {
	// Not parallel - modifies package-level state
	defer ResetForTesting()

	customDir := "/injected/test/dir"
	SetPaths(NewPathResolver(customDir))

	got := Paths().BaseDir()
	if got != customDir {
		t.Errorf("Paths().BaseDir() = %q, want %q", got, customDir)
	}
}

func TestPaths_ResetForTesting(t *testing.T) {
	// Not parallel - modifies package-level state
	defer ResetForTesting()

	customDir := "/injected/test/dir"
	SetPaths(NewPathResolver(customDir))

	ResetForTesting()

	// After reset, should use default resolver
	got := Paths().BaseDir()
	if got == customDir {
		t.Error("ResetForTesting() did not clear injected resolver")
	}
}

func TestPaths_DefaultWithoutInjection(t *testing.T) {
	// Not parallel - modifies package-level state
	defer ResetForTesting()
	ResetForTesting() // Ensure clean state

	t.Setenv(meta.EnvPrefix+"_HOME", "/env/based/path")

	got := Paths().BaseDir()
	if got != "/env/based/path" {
		t.Errorf("Paths().BaseDir() = %q, want /env/based/path", got)
	}
}

func TestWorktreeIDFromPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		worktreeDir string
	}{
		{
			name:        "absolute path",
			worktreeDir: "/Users/test/project",
		},
		{
			name:        "relative path",
			worktreeDir: "relative/path",
		},
		{
			name:        "path with spaces",
			worktreeDir: "/Users/test/my project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			id := WorktreeIDFromPath(tt.worktreeDir)

			// ID should be non-empty hex string (16 chars = 8 bytes hex)
			if len(id) != 16 {
				t.Errorf("WorktreeIDFromPath() = %q, want 16 char hex string", id)
			}
		})
	}
}

func TestWorktreeIDFromPath_Deterministic(t *testing.T) {
	t.Parallel()

	worktreeDir := "/Users/test/project"

	id1 := WorktreeIDFromPath(worktreeDir)
	id2 := WorktreeIDFromPath(worktreeDir)

	if id1 != id2 {
		t.Errorf("WorktreeIDFromPath() not deterministic: %q != %q", id1, id2)
	}
}

func TestWorktreeIDFromPath_UniqueForDifferentPaths(t *testing.T) {
	t.Parallel()

	id1 := WorktreeIDFromPath("/project/a")
	id2 := WorktreeIDFromPath("/project/b")

	if id1 == id2 {
		t.Error("Different paths should produce different IDs")
	}
}

// Test backwards-compatible package-level functions.
func TestBackwardsCompatibleFunctions(t *testing.T) {
	// Not parallel - modifies package-level state
	defer ResetForTesting()

	customDir := "/test/backwards/compat"
	SetPaths(NewPathResolver(customDir))

	t.Run("BaseDir", func(t *testing.T) {
		if got := BaseDir(); got != customDir {
			t.Errorf("BaseDir() = %q, want %q", got, customDir)
		}
	})

	t.Run("GlobalSocketPath", func(t *testing.T) {
		want := filepath.Join(customDir, "global.sock")
		if got := GlobalSocketPath(); got != want {
			t.Errorf("GlobalSocketPath() = %q, want %q", got, want)
		}
	})

	t.Run("GlobalLockPath", func(t *testing.T) {
		want := filepath.Join(customDir, "global.lock")
		if got := GlobalLockPath(); got != want {
			t.Errorf("GlobalLockPath() = %q, want %q", got, want)
		}
	})

	t.Run("WorktreeSocketPath", func(t *testing.T) {
		got := WorktreeSocketPath("/some/project")
		if !strings.HasPrefix(got, filepath.Join(customDir, "worktrees")) {
			t.Errorf("WorktreeSocketPath() = %q, should start with worktrees dir", got)
		}
	})

	t.Run("MemoryDir", func(t *testing.T) {
		want := filepath.Join(customDir, "memory")
		if got := MemoryDir(); got != want {
			t.Errorf("MemoryDir() = %q, want %q", got, want)
		}
	})

	t.Run("ConfigPath", func(t *testing.T) {
		want := filepath.Join(customDir, meta.ConfigFile)
		if got := ConfigPath(); got != want {
			t.Errorf("ConfigPath() = %q, want %q", got, want)
		}
	})

	t.Run("EnvPath", func(t *testing.T) {
		want := filepath.Join(customDir, ".env")
		if got := EnvPath(); got != want {
			t.Errorf("EnvPath() = %q, want %q", got, want)
		}
	})

	t.Run("EnsureDir", func(t *testing.T) {
		tmpDir := t.TempDir()
		SetPaths(NewPathResolver(tmpDir))

		if err := EnsureDir(); err != nil {
			t.Errorf("EnsureDir() error = %v", err)
		}
	})
}
