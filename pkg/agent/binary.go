package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// ResolveCommandPath finds a command in PATH and common GUI-launch locations.
func ResolveCommandPath(name string) (string, error) {
	path, lookErr := exec.LookPath(name)
	if lookErr == nil {
		return path, nil
	}

	for _, candidate := range fallbackCommandPaths(name) {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	return "", lookErr
}

func fallbackCommandPaths(name string) []string {
	base := make([]string, 0, 5)
	base = append(base,
		filepath.Join("/opt/homebrew/bin", name),
		filepath.Join("/usr/local/bin", name),
		filepath.Join("/usr/bin", name),
	)

	if runtime.GOOS != "darwin" {
		return base
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return base
	}

	return append(base,
		filepath.Join(home, ".local", "bin", name),
		filepath.Join(home, ".bun", "bin", name),
	)
}
