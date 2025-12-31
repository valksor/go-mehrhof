package conductor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/valksor/go-mehrhof/internal/agent"
	"github.com/valksor/go-mehrhof/internal/events"
)

// DeleteFileSentinel is a special marker that indicates a file should be deleted
// This provides an alternative to setting operation: delete in YAML blocks
const DeleteFileSentinel = "__DELETE_FILE__"

// ensureDirExists creates the directory for the given file path if it doesn't exist.
// This is a helper to avoid code duplication when writing files.
func ensureDirExists(path string) error {
	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

// validatePathInWorkspace ensures that a resolved path is within the workspace root.
// This prevents path traversal attacks when applying file changes from AI agent output.
func validatePathInWorkspace(resolved, root string) error {
	// Get the relative path from root to resolved
	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	// Check if the relative path starts with ".." which would escape the root
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return fmt.Errorf("path outside workspace: %s", resolved)
	}
	return nil
}

// applyFiles writes agent file changes to disk
func applyFiles(ctx context.Context, c *Conductor, files []agent.FileChange) error {
	root := c.opts.WorkDir
	if c.git != nil {
		root = c.git.Root()
	}

	// Resolve symlinks in root path for accurate validation (handles macOS /var -> /private/var symlinks)
	resolvedRoot := root
	if res, err := filepath.EvalSymlinks(root); err == nil {
		resolvedRoot = res
	}
	// If root doesn't exist yet, EvalSymlinks will fail, so we use root as-is

	var stats struct {
		created int
		updated int
		deleted int
	}

	for _, fc := range files {
		path := filepath.Join(root, fc.Path)

		// Validate the path is within workspace (prevent path traversal attacks)
		// Resolve symlinks in the target path and validate it stays within root
		resolvedPath := path
		if res, err := filepath.EvalSymlinks(path); err == nil {
			resolvedPath = res
		}
		// Validate against both the original root and resolved root to handle symlinked paths
		if err := validatePathInWorkspace(resolvedPath, root); err != nil {
			if err := validatePathInWorkspace(resolvedPath, resolvedRoot); err != nil {
				return fmt.Errorf("invalid file path %q: %w", fc.Path, err)
			}
		}

		// Check for delete sentinel in content (alternative to operation: delete)
		if fc.Content == DeleteFileSentinel {
			fc.Operation = agent.FileOpDelete
		}

		switch fc.Operation {
		case agent.FileOpCreate:
			// Ensure directory exists
			if err := ensureDirExists(path); err != nil {
				return fmt.Errorf("create directory for %s: %w", path, err)
			}

			// Write file
			if err := os.WriteFile(path, []byte(fc.Content), 0o644); err != nil {
				return fmt.Errorf("write file %s: %w", path, err)
			}
			stats.created++

			c.eventBus.PublishRaw(events.Event{
				Type: events.TypeFileChanged,
				Data: map[string]any{
					"path":      fc.Path,
					"operation": "create",
				},
			})

		case agent.FileOpUpdate:
			// Ensure directory exists
			if err := ensureDirExists(path); err != nil {
				return fmt.Errorf("create directory for %s: %w", path, err)
			}

			// Write file
			if err := os.WriteFile(path, []byte(fc.Content), 0o644); err != nil {
				return fmt.Errorf("write file %s: %w", path, err)
			}
			stats.updated++

			c.eventBus.PublishRaw(events.Event{
				Type: events.TypeFileChanged,
				Data: map[string]any{
					"path":      fc.Path,
					"operation": "update",
				},
			})

		case agent.FileOpDelete:
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("delete file %s: %w", path, err)
			}
			stats.deleted++

			c.eventBus.PublishRaw(events.Event{
				Type: events.TypeFileChanged,
				Data: map[string]any{
					"path":      fc.Path,
					"operation": "delete",
				},
			})
		}
	}

	// Publish summary of file operations
	if stats.created > 0 || stats.updated > 0 || stats.deleted > 0 {
		c.eventBus.PublishRaw(events.Event{
			Type: events.TypeProgress,
			Data: map[string]any{
				"message": fmt.Sprintf("Files: %d created, %d updated, %d deleted",
					stats.created, stats.updated, stats.deleted),
			},
		})
	}

	return nil
}
