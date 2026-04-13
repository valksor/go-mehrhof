package browser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadState_Missing(t *testing.T) {
	state, err := LoadState(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err != nil {
		t.Fatalf("LoadState() missing error = %v, want nil", err)
	}
	if state == nil {
		t.Fatal("LoadState() missing = nil, want empty state")
	}
	if len(state.Cookies) != 0 {
		t.Errorf("LoadState() missing cookies = %v, want empty", state.Cookies)
	}
}

func TestSaveLoadState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "browser.json")

	original := &BrowserState{
		Cookies: []Cookie{
			{Name: "session", Value: "abc123", Domain: "example.com", Path: "/"},
			{Name: "pref", Value: "dark", Domain: "example.com", Path: "/", Secure: true},
		},
		LocalStorage: map[string]map[string]string{
			"https://example.com": {"theme": "dark", "lang": "en"},
		},
	}

	if err := original.Save(path); err != nil {
		t.Fatalf("BrowserState.Save() error = %v", err)
	}

	got, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}

	if len(got.Cookies) != 2 {
		t.Errorf("cookies count = %d, want 2", len(got.Cookies))
	}

	found := false
	for _, c := range got.Cookies {
		if c.Name == "session" && c.Value == "abc123" && c.Domain == "example.com" {
			found = true
		}
	}
	if !found {
		t.Error("session cookie not found after round-trip")
	}

	if got.LocalStorage["https://example.com"]["theme"] != "dark" {
		t.Errorf("localStorage theme = %q, want dark", got.LocalStorage["https://example.com"]["theme"])
	}
}

func TestSaveState_CreatesParentDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "subdir", "browser.json")
	if err := (&BrowserState{}).Save(path); err != nil {
		t.Fatalf("BrowserState.Save() nested dir error = %v", err)
	}
}

func TestLoadState_InvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(path, []byte("not valid json {{{"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadState(path)
	if err == nil {
		t.Error("LoadState() invalid JSON expected error, got nil")
	}
}

func TestBrowserProfilesDir(t *testing.T) {
	got := BrowserProfilesDir()
	if !filepath.IsAbs(got) {
		t.Errorf("BrowserProfilesDir() = %q is not absolute", got)
	}
	if !strings.HasSuffix(got, "browser-profiles") {
		t.Errorf("BrowserProfilesDir() = %q, want suffix browser-profiles", got)
	}
}

func TestWorktreesDir(t *testing.T) {
	got := WorktreesDir()
	if !filepath.IsAbs(got) {
		t.Errorf("WorktreesDir() = %q is not absolute", got)
	}
	if !strings.HasSuffix(got, "worktrees") {
		t.Errorf("WorktreesDir() = %q, want suffix worktrees", got)
	}
}

func TestGlobalProfilePath(t *testing.T) {
	got := GlobalProfilePath("default")
	if !filepath.IsAbs(got) {
		t.Errorf("GlobalProfilePath() = %q is not absolute", got)
	}
	if filepath.Base(got) != "default.json" {
		t.Errorf("GlobalProfilePath() base = %q, want default.json", filepath.Base(got))
	}
}

func TestWorkTreeStatePath(t *testing.T) {
	got := WorktreeStatePath("abc123def456")
	if !filepath.IsAbs(got) {
		t.Errorf("WorktreeStatePath() = %q is not absolute", got)
	}
	if filepath.Base(got) != "browser.json" {
		t.Errorf("WorktreeStatePath() base = %q, want browser.json", filepath.Base(got))
	}
}

func TestWorkTreeHash(t *testing.T) {
	h1 := WorktreeHash("/some/path")
	if len(h1) != 16 {
		t.Errorf("WorktreeHash() length = %d, want 16", len(h1))
	}
	h2 := WorktreeHash("/some/path")
	if h1 != h2 {
		t.Error("WorktreeHash() is not deterministic")
	}
	h3 := WorktreeHash("/other/path")
	if h1 == h3 {
		t.Error("WorktreeHash() produces same hash for different paths")
	}
}

func TestMergeState_EmptyProfiles(t *testing.T) {
	// Use a unique worktree hash so we don't interfere with real data.
	worktreeHash := WorktreeHash("/test/merge/" + t.Name())
	profileName := "test-merge-" + t.Name()

	mergedPath := filepath.Join(Paths(), "tmp", "merged-"+worktreeHash+".json")
	t.Cleanup(func() { _ = os.Remove(mergedPath) })

	got, err := MergeState(profileName, worktreeHash)
	if err != nil {
		t.Fatalf("MergeState() error = %v", err)
	}
	if got == "" {
		t.Error("MergeState() returned empty path")
	}
	if _, err := os.Stat(got); err != nil {
		t.Errorf("MergeState() path %q does not exist: %v", got, err)
	}
}

func TestMergeState_WithCookies(t *testing.T) {
	worktreeHash := WorktreeHash("/test/merge/cookies/" + t.Name())
	profileName := "test-cookies-" + t.Name()

	// Create a worktree state file manually
	wtPath := WorktreeStatePath(worktreeHash)
	if err := os.MkdirAll(filepath.Dir(wtPath), 0o755); err != nil {
		t.Fatalf("create worktree dir: %v", err)
	}
	wtState := &BrowserState{
		Cookies: []Cookie{{Name: "wt-cookie", Value: "wt-value", Domain: "example.com", Path: "/"}},
	}
	if err := wtState.Save(wtPath); err != nil {
		t.Fatalf("save worktree state: %v", err)
	}

	mergedPath := filepath.Join(Paths(), "tmp", "merged-"+worktreeHash+".json")
	t.Cleanup(func() {
		_ = os.Remove(mergedPath)
		_ = os.Remove(wtPath)
		_ = os.Remove(filepath.Dir(wtPath))
	})

	got, err := MergeState(profileName, worktreeHash)
	if err != nil {
		t.Fatalf("MergeState() error = %v", err)
	}

	merged, err := LoadState(got)
	if err != nil {
		t.Fatalf("LoadState(merged) error = %v", err)
	}
	if len(merged.Cookies) != 1 || merged.Cookies[0].Name != "wt-cookie" {
		t.Errorf("merged cookies = %v, want wt-cookie", merged.Cookies)
	}
}

func TestExtractWorktreeState(t *testing.T) {
	worktreeHash := WorktreeHash("/test/extract/" + t.Name())
	profileName := "test-extract-" + t.Name()

	// Create a temp merged state file (not in the kvelmo dir)
	mergedPath := filepath.Join(t.TempDir(), "merged.json")
	merged := &BrowserState{
		Cookies: []Cookie{{Name: "new-cookie", Value: "new-value", Domain: "example.com", Path: "/"}},
	}
	if err := merged.Save(mergedPath); err != nil {
		t.Fatalf("save merged: %v", err)
	}

	wtPath := WorktreeStatePath(worktreeHash)
	t.Cleanup(func() {
		_ = os.Remove(wtPath)
		_ = os.Remove(filepath.Dir(wtPath))
	})

	if err := ExtractWorktreeState(mergedPath, profileName, worktreeHash); err != nil {
		t.Fatalf("ExtractWorktreeState() error = %v", err)
	}

	if _, err := os.Stat(wtPath); err != nil {
		t.Errorf("worktree state not created: %v", err)
	}
}

func TestUpdateGlobalProfile(t *testing.T) {
	profileName := "test-profile-" + t.Name()
	path := GlobalProfilePath(profileName)

	t.Cleanup(func() { _ = os.Remove(path) })

	state := &BrowserState{
		Cookies: []Cookie{{Name: "auth", Value: "token123", Domain: "example.com", Path: "/"}},
	}

	if err := UpdateGlobalProfile(profileName, state); err != nil {
		t.Fatalf("UpdateGlobalProfile() error = %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("global profile not created: %v", err)
	}
}
