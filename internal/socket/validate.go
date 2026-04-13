package socket

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidatePath ensures a path stays within the allowed base directory.
// It returns the cleaned absolute path if valid, or an error if the path
// would escape the base directory (directory traversal attack).
// This function resolves symlinks to prevent symlink-based directory traversal.
func ValidatePath(basePath, requestedPath string) (string, error) {
	// Clean and make absolute
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return "", fmt.Errorf("invalid base path: %w", err)
	}

	// Resolve symlinks in base path to get real path for comparison
	realBase, err := filepath.EvalSymlinks(absBase)
	if err != nil {
		return "", fmt.Errorf("resolve base path: %w", err)
	}

	// Handle empty requested path as base path
	if requestedPath == "" {
		return realBase, nil
	}

	// If requested path is relative, join with base
	var absRequested string
	if filepath.IsAbs(requestedPath) {
		absRequested = filepath.Clean(requestedPath)
	} else {
		absRequested = filepath.Clean(filepath.Join(absBase, requestedPath))
	}

	// Try to resolve symlinks in the requested path
	// If the path doesn't exist (creating a new file), walk up to find the nearest
	// existing ancestor and resolve that, then reconstruct the path
	realRequested, err := filepath.EvalSymlinks(absRequested)
	if err != nil {
		if os.IsNotExist(err) {
			// Path doesn't exist - walk up to find nearest existing ancestor
			realRequested, err = resolveNonExistentPath(absRequested)
			if err != nil {
				return "", fmt.Errorf("resolve path: %w", err)
			}
		} else {
			return "", fmt.Errorf("resolve path: %w", err)
		}
	}

	// Ensure the requested path is within or equal to the base path
	// Add trailing separator to base for proper prefix matching
	// Handle root path case where realBase is "/" to avoid "//"
	baseWithSep := realBase
	if !strings.HasSuffix(realBase, string(filepath.Separator)) {
		baseWithSep += string(filepath.Separator)
	}

	if realRequested != realBase && !strings.HasPrefix(realRequested, baseWithSep) {
		return "", fmt.Errorf("path escapes allowed directory: %s", requestedPath)
	}

	return realRequested, nil
}

// resolveNonExistentPath resolves a path that doesn't exist by walking up to
// the nearest existing ancestor, resolving its symlinks, then reconstructing
// the path with the non-existent suffix.
func resolveNonExistentPath(absPath string) (string, error) {
	// Collect path components that don't exist
	var nonExistent []string
	current := absPath

	for {
		if _, err := os.Stat(current); err == nil {
			// Found an existing path - resolve it
			realCurrent, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", err
			}
			// Reconstruct the path with non-existent suffix
			for i := len(nonExistent) - 1; i >= 0; i-- {
				realCurrent = filepath.Join(realCurrent, nonExistent[i])
			}

			return realCurrent, nil
		}

		// Path doesn't exist, save the component and move up
		nonExistent = append(nonExistent, filepath.Base(current))
		parent := filepath.Dir(current)
		if parent == current {
			// Reached root without finding existing path
			return "", fmt.Errorf("no existing ancestor found for path: %s", absPath)
		}
		current = parent
	}
}

// ValidatePathWithRoots checks if a path is within any of the allowed root directories.
// Useful when multiple directories are allowed (e.g., home dir and project dirs).
func ValidatePathWithRoots(roots []string, requestedPath string) (string, error) {
	if len(roots) == 0 {
		return "", errors.New("no allowed directories configured")
	}

	// Try each root
	for _, root := range roots {
		if valid, err := ValidatePath(root, requestedPath); err == nil {
			return valid, nil
		}
	}

	return "", fmt.Errorf("path not within allowed directories: %s", requestedPath)
}
