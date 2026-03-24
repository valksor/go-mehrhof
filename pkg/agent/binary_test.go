package agent

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestFallbackCommandPaths(t *testing.T) {
	paths := fallbackCommandPaths("claude")

	if len(paths) < 3 {
		t.Fatalf("fallbackCommandPaths() returned %d paths, want at least 3", len(paths))
	}

	if got := paths[0]; got != filepath.Join("/opt/homebrew/bin", "claude") {
		t.Fatalf("fallbackCommandPaths()[0] = %q, want %q", got, filepath.Join("/opt/homebrew/bin", "claude"))
	}

	if runtime.GOOS == "darwin" {
		foundBun := false
		for _, path := range paths {
			if filepath.Base(filepath.Dir(path)) == "bin" && filepath.Base(filepath.Dir(filepath.Dir(path))) == ".bun" {
				foundBun = true

				break
			}
		}
		if !foundBun {
			t.Fatal("fallbackCommandPaths() should include ~/.bun/bin on darwin")
		}
	}
}
