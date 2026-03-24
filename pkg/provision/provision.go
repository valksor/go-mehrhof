// Package provision copies configuration files and symlinks dependency
// directories from a source repository into a newly created git worktree.
// This prevents agent setup failures caused by missing .env, node_modules,
// .venv, and similar artifacts that are not tracked by git.
package provision

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Options configures worktree provisioning.
type Options struct {
	// CopyPatterns are glob patterns for files to copy (e.g., ".env*").
	CopyPatterns []string

	// SymlinkPatterns are directory names to symlink from source (e.g., "node_modules").
	SymlinkPatterns []string

	// SetupCommands are shell commands to run after provisioning.
	// Each command runs with the worktree directory as its working directory.
	SetupCommands []string
}

// Result describes what was provisioned.
type Result struct {
	FilesCopied     []string `json:"files_copied"`
	SymlinksCreated []string `json:"symlinks_created"`
	CommandsRun     []string `json:"commands_run"`
}

// Empty returns true if nothing was provisioned.
func (r *Result) Empty() bool {
	return len(r.FilesCopied) == 0 && len(r.SymlinksCreated) == 0 && len(r.CommandsRun) == 0
}

// Provision copies config files and creates symlinks from srcDir to worktreeDir.
// It merges all copy/symlink patterns from opts, globs against srcDir, and
// reproduces the matched items in worktreeDir.
func Provision(srcDir, worktreeDir string, opts Options) (*Result, error) {
	if srcDir == "" || worktreeDir == "" {
		return nil, fmt.Errorf("provision: srcDir and worktreeDir must be non-empty")
	}

	result := &Result{}

	// Copy files matching glob patterns.
	for _, pattern := range opts.CopyPatterns {
		matches, err := filepath.Glob(filepath.Join(srcDir, pattern))
		if err != nil {
			slog.Warn("provision: bad glob pattern", "pattern", pattern, "error", err)
			continue
		}

		for _, src := range matches {
			info, statErr := os.Stat(src)
			if statErr != nil || info.IsDir() {
				continue // skip directories and stat errors
			}

			rel, _ := filepath.Rel(srcDir, src)
			dst := filepath.Join(worktreeDir, rel)

			// Ensure parent directory exists.
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return result, fmt.Errorf("provision: mkdir for %s: %w", rel, err)
			}

			if err := copyFile(src, dst); err != nil {
				return result, fmt.Errorf("provision: copy %s: %w", rel, err)
			}

			result.FilesCopied = append(result.FilesCopied, rel)
		}
	}

	// Create symlinks for matching directories.
	for _, name := range opts.SymlinkPatterns {
		srcPath := filepath.Join(srcDir, name)
		info, err := os.Stat(srcPath)
		if err != nil || !info.IsDir() {
			continue // source directory does not exist
		}

		dstPath := filepath.Join(worktreeDir, name)

		// Skip if destination already exists.
		if _, err := os.Lstat(dstPath); err == nil {
			continue
		}

		if err := os.Symlink(srcPath, dstPath); err != nil {
			return result, fmt.Errorf("provision: symlink %s: %w", name, err)
		}

		result.SymlinksCreated = append(result.SymlinksCreated, name)
	}

	// Run setup commands.
	for _, cmd := range opts.SetupCommands {
		if strings.TrimSpace(cmd) == "" {
			continue
		}

		c := exec.Command("sh", "-c", cmd)
		c.Dir = worktreeDir

		if out, err := c.CombinedOutput(); err != nil {
			return result, fmt.Errorf("provision: command %q failed: %w\n%s", cmd, err, string(out))
		}

		result.CommandsRun = append(result.CommandsRun, cmd)
	}

	return result, nil
}

// Preview returns what Provision would do without executing anything.
// It checks which files match copy patterns and which symlink targets exist.
func Preview(srcDir string, opts Options) (*Result, error) {
	if srcDir == "" {
		return nil, fmt.Errorf("preview: srcDir must be non-empty")
	}

	result := &Result{}

	for _, pattern := range opts.CopyPatterns {
		matches, err := filepath.Glob(filepath.Join(srcDir, pattern))
		if err != nil {
			continue
		}

		for _, src := range matches {
			info, statErr := os.Stat(src)
			if statErr != nil || info.IsDir() {
				continue
			}

			rel, _ := filepath.Rel(srcDir, src)
			result.FilesCopied = append(result.FilesCopied, rel)
		}
	}

	for _, name := range opts.SymlinkPatterns {
		srcPath := filepath.Join(srcDir, name)
		info, err := os.Stat(srcPath)
		if err != nil || !info.IsDir() {
			continue
		}

		result.SymlinksCreated = append(result.SymlinksCreated, name)
	}

	result.CommandsRun = make([]string, 0, len(opts.SetupCommands))
	for _, cmd := range opts.SetupCommands {
		if strings.TrimSpace(cmd) != "" {
			result.CommandsRun = append(result.CommandsRun, cmd)
		}
	}

	return result, nil
}

// copyFile copies a single file from src to dst, preserving permissions.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return fmt.Errorf("create dest: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy data: %w", err)
	}

	return nil
}
