package socket

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidatePath(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		base        string
		requested   string
		wantErr     bool
		wantContain string // substring the result should contain
	}{
		{
			name:      "empty requested returns base",
			base:      tmpDir,
			requested: "",
			wantErr:   false,
		},
		{
			name:      "valid subdir",
			base:      tmpDir,
			requested: "subdir",
			wantErr:   false,
		},
		{
			name:      "valid nested path",
			base:      tmpDir,
			requested: "a/b/c",
			wantErr:   false,
		},
		{
			name:      "directory traversal with ..",
			base:      tmpDir,
			requested: "../etc/passwd",
			wantErr:   true,
		},
		{
			name:      "deep directory traversal",
			base:      tmpDir,
			requested: "a/b/../../../../../../etc/passwd",
			wantErr:   true,
		},
		{
			name:      "absolute path outside base",
			base:      tmpDir,
			requested: "/etc/passwd",
			wantErr:   true,
		},
		{
			name:        "absolute path inside base",
			base:        tmpDir,
			requested:   filepath.Join(tmpDir, "inside"),
			wantErr:     false,
			wantContain: "inside",
		},
		{
			name:      "path with double dots in name (valid)",
			base:      tmpDir,
			requested: "file..name",
			wantErr:   false,
		},
		{
			name:      "base path exact match",
			base:      tmpDir,
			requested: tmpDir,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidatePath(tt.base, tt.requested)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !tt.wantErr {
				if !filepath.IsAbs(got) {
					t.Errorf("ValidatePath() returned non-absolute path: %s", got)
				}
				// Check that result contains expected substring when wantContain is set
				if tt.wantContain != "" && !strings.Contains(got, tt.wantContain) {
					t.Errorf("ValidatePath() = %q, want string containing %q", got, tt.wantContain)
				}
			}
		})
	}
}

func TestValidatePathWithRoots(t *testing.T) {
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	tests := []struct {
		name      string
		roots     []string
		requested string
		wantErr   bool
	}{
		{
			name:      "path in first root",
			roots:     []string{tmpDir1, tmpDir2},
			requested: filepath.Join(tmpDir1, "file.txt"),
			wantErr:   false,
		},
		{
			name:      "path in second root",
			roots:     []string{tmpDir1, tmpDir2},
			requested: filepath.Join(tmpDir2, "file.txt"),
			wantErr:   false,
		},
		{
			name:      "path outside all roots",
			roots:     []string{tmpDir1, tmpDir2},
			requested: "/etc/passwd",
			wantErr:   true,
		},
		{
			name:      "empty roots",
			roots:     []string{},
			requested: tmpDir1,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidatePathWithRoots(tt.roots, tt.requested)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePathWithRoots() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidatePath_SymlinkEscape(t *testing.T) {
	// Create a temp dir structure with a symlink pointing outside
	tmpDir := t.TempDir()
	linkPath := filepath.Join(tmpDir, "escape")

	// Create symlink to /etc (or skip if can't create symlinks)
	if err := os.Symlink("/etc", linkPath); err != nil {
		t.Skip("Cannot create symlinks, skipping symlink test")
	}

	// ValidatePath now resolves symlinks and correctly rejects paths that
	// escape the base directory via symlink. This prevents symlink-based
	// directory traversal attacks.
	_, err := ValidatePath(tmpDir, "escape")
	if err == nil {
		t.Error("ValidatePath should reject symlinks that escape the base directory")
	}
}

func TestValidatePath_SymlinkWithinBase(t *testing.T) {
	// Create a temp dir structure with a symlink pointing inside the base
	tmpDir := t.TempDir()
	subdir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatalf("create subdir: %v", err)
	}
	linkPath := filepath.Join(tmpDir, "link-to-subdir")

	// Create symlink to subdir (within base)
	if err := os.Symlink(subdir, linkPath); err != nil {
		t.Skip("Cannot create symlinks, skipping symlink test")
	}

	// Symlinks within the base directory should be allowed
	result, err := ValidatePath(tmpDir, "link-to-subdir")
	if err != nil {
		t.Errorf("ValidatePath should allow symlinks within base directory: %v", err)
	}

	// The resolved path should be the actual subdir
	// Use EvalSymlinks on expected value too (macOS has /var -> /private/var)
	expectedSubdir, err := filepath.EvalSymlinks(subdir)
	if err != nil {
		t.Fatalf("resolve expected subdir: %v", err)
	}
	if result != expectedSubdir {
		t.Errorf("ValidatePath() = %q, want %q", result, expectedSubdir)
	}
}
