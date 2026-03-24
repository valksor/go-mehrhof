package provision

import (
	"os"
	"path/filepath"
)

// commonCopyPatterns are file patterns copied for all project types.
var commonCopyPatterns = []string{
	".env",
	".env.*",
	".tool-versions",
}

// DefaultOptions returns provisioning defaults based on project type detection.
// It inspects the project directory for known manifest files (go.mod,
// package.json, pyproject.toml, etc.) and returns appropriate copy/symlink
// patterns for each detected ecosystem.
func DefaultOptions(projectDir string) Options {
	opts := Options{
		CopyPatterns: append([]string{}, commonCopyPatterns...),
	}

	if isGoProject(projectDir) {
		opts.CopyPatterns = append(opts.CopyPatterns, "go.sum")
	}

	if isNodeProject(projectDir) {
		opts.CopyPatterns = append(opts.CopyPatterns, ".nvmrc", ".node-version")
		opts.SymlinkPatterns = append(opts.SymlinkPatterns, "node_modules")
	}

	if isPythonProject(projectDir) {
		opts.CopyPatterns = append(opts.CopyPatterns, ".python-version")
		opts.SymlinkPatterns = append(opts.SymlinkPatterns, ".venv", "venv")
	}

	return opts
}

// MergeOptions merges user-configured overrides into base options.
// User patterns are appended (not replaced), and duplicates are removed.
func MergeOptions(base, override Options) Options {
	merged := Options{
		CopyPatterns:    dedup(append(base.CopyPatterns, override.CopyPatterns...)),
		SymlinkPatterns: dedup(append(base.SymlinkPatterns, override.SymlinkPatterns...)),
		SetupCommands:   append(base.SetupCommands, override.SetupCommands...),
	}

	return merged
}

func isGoProject(dir string) bool {
	return fileExists(filepath.Join(dir, "go.mod"))
}

func isNodeProject(dir string) bool {
	return fileExists(filepath.Join(dir, "package.json"))
}

func isPythonProject(dir string) bool {
	return fileExists(filepath.Join(dir, "pyproject.toml")) ||
		fileExists(filepath.Join(dir, "setup.py")) ||
		fileExists(filepath.Join(dir, "requirements.txt"))
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dedup(items []string) []string {
	seen := make(map[string]bool, len(items))
	result := make([]string, 0, len(items))

	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}
