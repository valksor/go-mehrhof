package browser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPaths(t *testing.T) {
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".valksor", "kvelmo")

	got := Paths()
	if got != expected {
		t.Errorf("Paths() = %q, want %q", got, expected)
	}
}

func TestRuntimeDir(t *testing.T) {
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".valksor", "kvelmo", "runtime")

	got := RuntimeDir()
	if got != expected {
		t.Errorf("RuntimeDir() = %q, want %q", got, expected)
	}
}

func TestNodeBinaryPath(t *testing.T) {
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".valksor", "kvelmo", "runtime", "node")

	got := NodeBinaryPath()
	if got != expected {
		t.Errorf("NodeBinaryPath() = %q, want %q", got, expected)
	}
}

func TestBinaryPath(t *testing.T) {
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".valksor", "kvelmo", "bin", "playwright-cli")

	got := BinaryPath()
	if got != expected {
		t.Errorf("BinaryPath() = %q, want %q", got, expected)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.Headless {
		t.Error("DefaultConfig().Headless should be true")
	}

	if cfg.Browser != "chromium" {
		t.Errorf("DefaultConfig().Browser = %q, want %q", cfg.Browser, "chromium")
	}

	if cfg.Profile != "default" {
		t.Errorf("DefaultConfig().Profile = %q, want %q", cfg.Profile, "default")
	}

	if cfg.Timeout != 30 {
		t.Errorf("DefaultConfig().Timeout = %d, want %d", cfg.Timeout, 30)
	}
}

func TestWorktreeHash(t *testing.T) {
	// Same path should produce same hash
	path := "/home/user/project"
	hash1 := WorktreeHash(path)
	hash2 := WorktreeHash(path)

	if hash1 != hash2 {
		t.Errorf("WorktreeHash should be deterministic: %q != %q", hash1, hash2)
	}

	// Different paths should produce different hashes
	hash3 := WorktreeHash("/different/path")
	if hash1 == hash3 {
		t.Error("Different paths should produce different hashes")
	}

	// Hash should be 16 chars (8 bytes hex encoded)
	if len(hash1) != 16 {
		t.Errorf("WorktreeHash length = %d, want 16", len(hash1))
	}
}

func TestMergeStates(t *testing.T) {
	a := &BrowserState{
		Cookies: []Cookie{
			{Name: "session", Value: "old", Domain: "example.com", Path: "/"},
			{Name: "global", Value: "value", Domain: "auth.com", Path: "/"},
		},
		LocalStorage: map[string]map[string]string{
			"https://example.com": {"key1": "value1"},
		},
	}

	b := &BrowserState{
		Cookies: []Cookie{
			{Name: "session", Value: "new", Domain: "example.com", Path: "/"},
			{Name: "local", Value: "value", Domain: "local.com", Path: "/"},
		},
		LocalStorage: map[string]map[string]string{
			"https://example.com": {"key2": "value2"},
			"https://other.com":   {"key3": "value3"},
		},
	}

	merged := mergeStates(a, b)

	// Should have 3 cookies (global from a, session from b overrides a, local from b)
	if len(merged.Cookies) != 3 {
		t.Errorf("merged cookies count = %d, want 3", len(merged.Cookies))
	}

	// Find the session cookie - should have new value
	for _, c := range merged.Cookies {
		if c.Name == "session" && c.Domain == "example.com" {
			if c.Value != "new" {
				t.Errorf("session cookie value = %q, want %q", c.Value, "new")
			}
		}
	}

	// LocalStorage should be merged
	if len(merged.LocalStorage) != 2 {
		t.Errorf("merged localStorage origins = %d, want 2", len(merged.LocalStorage))
	}

	// example.com should have both keys
	if len(merged.LocalStorage["https://example.com"]) != 2 {
		t.Errorf("example.com localStorage keys = %d, want 2", len(merged.LocalStorage["https://example.com"]))
	}
}

func TestDiffStates(t *testing.T) {
	a := &BrowserState{
		Cookies: []Cookie{
			{Name: "global", Value: "same", Domain: "auth.com", Path: "/"},
		},
		LocalStorage: map[string]map[string]string{
			"https://auth.com": {"token": "abc"},
		},
	}

	b := &BrowserState{
		Cookies: []Cookie{
			{Name: "global", Value: "same", Domain: "auth.com", Path: "/"},
			{Name: "local", Value: "new", Domain: "project.com", Path: "/"},
		},
		LocalStorage: map[string]map[string]string{
			"https://auth.com":    {"token": "abc"},
			"https://project.com": {"state": "xyz"},
		},
	}

	diff := diffStates(a, b)

	// Only the local cookie should be in diff (global is same)
	if len(diff.Cookies) != 1 {
		t.Errorf("diff cookies count = %d, want 1", len(diff.Cookies))
	}

	if diff.Cookies[0].Name != "local" {
		t.Errorf("diff cookie name = %q, want %q", diff.Cookies[0].Name, "local")
	}

	// Only project.com localStorage should be in diff
	if len(diff.LocalStorage) != 1 {
		t.Errorf("diff localStorage origins = %d, want 1", len(diff.LocalStorage))
	}

	if _, ok := diff.LocalStorage["https://project.com"]; !ok {
		t.Error("diff should contain project.com localStorage")
	}
}
