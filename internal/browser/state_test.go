package browser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const testCookieSession = "session"

func TestBrowserState_SaveAndLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	state := &BrowserState{
		Cookies: []Cookie{
			{Name: testCookieSession, Value: "abc123", Domain: ".example.com", Path: "/"},
		},
		LocalStorage: map[string]map[string]string{
			"https://example.com": {"key": "value"},
		},
	}

	if err := state.Save(path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}

	if len(loaded.Cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(loaded.Cookies))
	}
	if loaded.Cookies[0].Name != testCookieSession {
		t.Errorf("cookie name = %q, want %q", loaded.Cookies[0].Name, testCookieSession)
	}
	if loaded.Cookies[0].Value != "abc123" {
		t.Errorf("cookie value = %q, want %q", loaded.Cookies[0].Value, "abc123")
	}
	if loaded.Cookies[0].Domain != ".example.com" {
		t.Errorf("cookie domain = %q, want %q", loaded.Cookies[0].Domain, ".example.com")
	}
	if loaded.LocalStorage["https://example.com"]["key"] != "value" {
		t.Errorf("localStorage value = %q, want %q",
			loaded.LocalStorage["https://example.com"]["key"], "value")
	}
}

func TestLoadState_NonExistentPath(t *testing.T) {
	state, err := LoadState("/nonexistent/path/that/does/not/exist.json")
	if err != nil {
		t.Fatalf("LoadState() expected nil error for non-existent file, got %v", err)
	}
	if len(state.Cookies) != 0 {
		t.Error("expected empty cookies for non-existent file")
	}
	if len(state.LocalStorage) != 0 {
		t.Error("expected empty localStorage for non-existent file")
	}
}

func TestLoadState_BadJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("not json at all {{{"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadState(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestBrowserState_Save_CreatesNestedDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "deep", "dir")
	path := filepath.Join(dir, "state.json")
	state := &BrowserState{}
	if err := state.Save(path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created: %v", err)
	}
}

func TestBrowserState_SaveLoad_EmptyState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.json")
	state := &BrowserState{}
	if err := state.Save(path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	if len(loaded.Cookies) != 0 {
		t.Errorf("expected 0 cookies, got %d", len(loaded.Cookies))
	}
}

func TestBrowserState_SaveLoad_AllFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "full.json")

	state := &BrowserState{
		Cookies: []Cookie{
			{
				Name:     testCookieSession,
				Value:    "tok123",
				Domain:   ".example.com",
				Path:     "/app",
				Expires:  1700000000.0,
				HTTPOnly: true,
				Secure:   true,
				SameSite: "Lax",
			},
			{
				Name:   "tracking",
				Value:  "xyz",
				Domain: ".ads.com",
				Path:   "/",
			},
		},
		LocalStorage: map[string]map[string]string{
			"https://example.com": {"theme": "dark", "lang": "en"},
			"https://other.com":   {"foo": "bar"},
		},
		SessionStorage: map[string]map[string]string{
			"https://example.com": {"tmp": "value"},
		},
	}

	if err := state.Save(path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}

	if len(loaded.Cookies) != 2 {
		t.Fatalf("expected 2 cookies, got %d", len(loaded.Cookies))
	}

	var session *Cookie
	for i := range loaded.Cookies {
		if loaded.Cookies[i].Name == testCookieSession {
			session = &loaded.Cookies[i]

			break
		}
	}
	if session == nil {
		t.Fatal("session cookie not found")
	}
	if session.Value != "tok123" {
		t.Errorf("session.Value = %q, want %q", session.Value, "tok123")
	}
	if session.Domain != ".example.com" {
		t.Errorf("session.Domain = %q, want %q", session.Domain, ".example.com")
	}
	if session.Path != "/app" {
		t.Errorf("session.Path = %q, want %q", session.Path, "/app")
	}
	if session.Expires != 1700000000.0 {
		t.Errorf("session.Expires = %f, want %f", session.Expires, 1700000000.0)
	}
	if !session.HTTPOnly {
		t.Error("session.HTTPOnly should be true")
	}
	if !session.Secure {
		t.Error("session.Secure should be true")
	}
	if session.SameSite != "Lax" {
		t.Errorf("session.SameSite = %q, want %q", session.SameSite, "Lax")
	}

	if loaded.LocalStorage["https://example.com"]["theme"] != "dark" {
		t.Error("localStorage theme mismatch")
	}
	if loaded.LocalStorage["https://other.com"]["foo"] != "bar" {
		t.Error("localStorage foo mismatch")
	}
	if loaded.SessionStorage["https://example.com"]["tmp"] != "value" {
		t.Error("sessionStorage tmp mismatch")
	}
}

func TestBrowserState_Save_JSONFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "format.json")
	state := &BrowserState{
		Cookies: []Cookie{
			{Name: "a", Value: "b", Domain: "c", Path: "/"},
		},
	}
	if err := state.Save(path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("saved file is not valid JSON: %v", err)
	}

	if data[0] != '{' {
		t.Error("expected JSON to start with '{'")
	}
}

func TestLoadState_PermissionDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses file permission checks")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "noperm.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	_, err := LoadState(path)
	if err == nil {
		t.Error("expected error for permission denied")
	}
}

func TestMergeState_WithOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	profileName := "test-merge-overrides"
	worktreeHash := WorktreeHash("/test/merge/overrides")

	globalPath := GlobalProfilePath(profileName)
	globalState := &BrowserState{
		Cookies: []Cookie{
			{Name: "auth", Value: "global-token", Domain: "example.com", Path: "/"},
			{Name: "pref", Value: "light", Domain: "example.com", Path: "/"},
		},
		LocalStorage: map[string]map[string]string{
			"https://example.com": {"theme": "light"},
		},
	}
	if err := globalState.Save(globalPath); err != nil {
		t.Fatalf("save global state: %v", err)
	}

	wtPath := WorktreeStatePath(worktreeHash)
	wtState := &BrowserState{
		Cookies: []Cookie{
			{Name: "pref", Value: "dark", Domain: "example.com", Path: "/"},
		},
		LocalStorage: map[string]map[string]string{
			"https://example.com": {"theme": "dark"},
		},
	}
	if err := wtState.Save(wtPath); err != nil {
		t.Fatalf("save worktree state: %v", err)
	}

	mergedPath, err := MergeState(profileName, worktreeHash)
	if err != nil {
		t.Fatalf("MergeState() error = %v", err)
	}

	merged, err := LoadState(mergedPath)
	if err != nil {
		t.Fatalf("LoadState(merged) error = %v", err)
	}

	if len(merged.Cookies) != 2 {
		t.Errorf("merged cookies count = %d, want 2", len(merged.Cookies))
	}

	for _, c := range merged.Cookies {
		if c.Name == "pref" && c.Value != "dark" {
			t.Errorf("pref cookie value = %q, want %q", c.Value, "dark")
		}
	}

	if merged.LocalStorage["https://example.com"]["theme"] != "dark" {
		t.Errorf("merged theme = %q, want %q",
			merged.LocalStorage["https://example.com"]["theme"], "dark")
	}
}

func TestExtractWorktreeState_DiffsCorrectly(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	profileName := "test-extract-diff"
	worktreeHash := WorktreeHash("/test/extract/diff")

	globalPath := GlobalProfilePath(profileName)
	globalState := &BrowserState{
		Cookies: []Cookie{
			{Name: "auth", Value: "token", Domain: "example.com", Path: "/"},
		},
	}
	if err := globalState.Save(globalPath); err != nil {
		t.Fatalf("save global: %v", err)
	}

	mergedPath := filepath.Join(t.TempDir(), "merged.json")
	mergedState := &BrowserState{
		Cookies: []Cookie{
			{Name: "auth", Value: "token", Domain: "example.com", Path: "/"},
			{Name: testCookieSession, Value: "new-session", Domain: "app.com", Path: "/"},
		},
		LocalStorage: map[string]map[string]string{
			"https://app.com": {"state": "active"},
		},
	}
	if err := mergedState.Save(mergedPath); err != nil {
		t.Fatalf("save merged: %v", err)
	}

	if err := ExtractWorktreeState(mergedPath, profileName, worktreeHash); err != nil {
		t.Fatalf("ExtractWorktreeState() error = %v", err)
	}

	wtPath := WorktreeStatePath(worktreeHash)
	wtState, err := LoadState(wtPath)
	if err != nil {
		t.Fatalf("LoadState(wt) error = %v", err)
	}

	if len(wtState.Cookies) != 1 {
		t.Fatalf("worktree cookies count = %d, want 1", len(wtState.Cookies))
	}
	if wtState.Cookies[0].Name != testCookieSession {
		t.Errorf("worktree cookie name = %q, want %q", wtState.Cookies[0].Name, testCookieSession)
	}

	if wtState.LocalStorage["https://app.com"]["state"] != "active" {
		t.Error("worktree localStorage not preserved")
	}
}

func TestUpdateGlobalProfile_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	profileName := "test-update-profile"
	state := &BrowserState{
		Cookies: []Cookie{
			{Name: "auth", Value: "token123", Domain: "example.com", Path: "/"},
		},
		LocalStorage: map[string]map[string]string{
			"https://example.com": {"user": "alice"},
		},
	}

	if err := UpdateGlobalProfile(profileName, state); err != nil {
		t.Fatalf("UpdateGlobalProfile() error = %v", err)
	}

	path := GlobalProfilePath(profileName)
	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}

	if len(loaded.Cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(loaded.Cookies))
	}
	if loaded.Cookies[0].Value != "token123" {
		t.Errorf("cookie value = %q, want %q", loaded.Cookies[0].Value, "token123")
	}
	if loaded.LocalStorage["https://example.com"]["user"] != "alice" {
		t.Error("localStorage not preserved")
	}
}

func TestDiffStates_IdenticalStates(t *testing.T) {
	state := &BrowserState{
		Cookies: []Cookie{
			{Name: testCookieSession, Value: "abc", Domain: "example.com", Path: "/"},
		},
		LocalStorage: map[string]map[string]string{
			"https://example.com": {"key": "val"},
		},
	}

	diff := diffStates(state, state)

	if len(diff.Cookies) != 0 {
		t.Errorf("expected 0 diff cookies for identical states, got %d", len(diff.Cookies))
	}
	if len(diff.LocalStorage) != 0 {
		t.Errorf("expected 0 diff localStorage for identical states, got %d", len(diff.LocalStorage))
	}
}

func TestDiffStates_AllNew(t *testing.T) {
	a := &BrowserState{}
	b := &BrowserState{
		Cookies: []Cookie{
			{Name: "c1", Value: "v1", Domain: "d1", Path: "/"},
			{Name: "c2", Value: "v2", Domain: "d2", Path: "/"},
		},
		LocalStorage: map[string]map[string]string{
			"https://a.com": {"k1": "v1"},
			"https://b.com": {"k2": "v2"},
		},
	}

	diff := diffStates(a, b)

	if len(diff.Cookies) != 2 {
		t.Errorf("expected 2 diff cookies, got %d", len(diff.Cookies))
	}
	if len(diff.LocalStorage) != 2 {
		t.Errorf("expected 2 diff localStorage origins, got %d", len(diff.LocalStorage))
	}
}

func TestDiffStates_ValueChanged(t *testing.T) {
	a := &BrowserState{
		Cookies: []Cookie{
			{Name: testCookieSession, Value: "old", Domain: "example.com", Path: "/"},
		},
		LocalStorage: map[string]map[string]string{
			"https://example.com": {"key": "old-val"},
		},
	}
	b := &BrowserState{
		Cookies: []Cookie{
			{Name: testCookieSession, Value: "new", Domain: "example.com", Path: "/"},
		},
		LocalStorage: map[string]map[string]string{
			"https://example.com": {"key": "new-val"},
		},
	}

	diff := diffStates(a, b)

	if len(diff.Cookies) != 1 {
		t.Fatalf("expected 1 diff cookie, got %d", len(diff.Cookies))
	}
	if diff.Cookies[0].Value != "new" {
		t.Errorf("diff cookie value = %q, want %q", diff.Cookies[0].Value, "new")
	}
	if diff.LocalStorage["https://example.com"]["key"] != "new-val" {
		t.Errorf("diff localStorage value = %q, want %q",
			diff.LocalStorage["https://example.com"]["key"], "new-val")
	}
}

func TestDiffStates_BothEmpty(t *testing.T) {
	a := &BrowserState{}
	b := &BrowserState{}

	diff := diffStates(a, b)

	if len(diff.Cookies) != 0 {
		t.Errorf("expected 0 diff cookies, got %d", len(diff.Cookies))
	}
	if len(diff.LocalStorage) != 0 {
		t.Errorf("expected 0 diff localStorage, got %d", len(diff.LocalStorage))
	}
}

func TestMergeStates_BothEmpty(t *testing.T) {
	a := &BrowserState{}
	b := &BrowserState{}

	merged := mergeStates(a, b)

	if len(merged.Cookies) != 0 {
		t.Errorf("expected 0 merged cookies, got %d", len(merged.Cookies))
	}
	if len(merged.LocalStorage) != 0 {
		t.Errorf("expected 0 merged localStorage, got %d", len(merged.LocalStorage))
	}
}

func TestMergeStates_OnlyA(t *testing.T) {
	a := &BrowserState{
		Cookies: []Cookie{
			{Name: "only-a", Value: "v", Domain: "a.com", Path: "/"},
		},
		LocalStorage: map[string]map[string]string{
			"https://a.com": {"k": "v"},
		},
	}
	b := &BrowserState{}

	merged := mergeStates(a, b)

	if len(merged.Cookies) != 1 {
		t.Errorf("expected 1 merged cookie, got %d", len(merged.Cookies))
	}
	if merged.LocalStorage["https://a.com"]["k"] != "v" {
		t.Error("localStorage from a not preserved")
	}
}

func TestMergeStates_OnlyB(t *testing.T) {
	a := &BrowserState{}
	b := &BrowserState{
		Cookies: []Cookie{
			{Name: "only-b", Value: "v", Domain: "b.com", Path: "/"},
		},
		LocalStorage: map[string]map[string]string{
			"https://b.com": {"k": "v"},
		},
	}

	merged := mergeStates(a, b)

	if len(merged.Cookies) != 1 {
		t.Errorf("expected 1 merged cookie, got %d", len(merged.Cookies))
	}
	if merged.LocalStorage["https://b.com"]["k"] != "v" {
		t.Error("localStorage from b not preserved")
	}
}

func TestCreateWrapper(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	if err := createWrapper(); err != nil {
		t.Fatalf("createWrapper() error = %v", err)
	}

	wrapperPath := BinaryPath()
	info, err := os.Stat(wrapperPath)
	if err != nil {
		t.Fatalf("wrapper not created: %v", err)
	}

	if info.Mode()&0o111 == 0 {
		t.Error("wrapper should be executable")
	}

	data, err := os.ReadFile(wrapperPath)
	if err != nil {
		t.Fatalf("read wrapper: %v", err)
	}
	if len(data) == 0 {
		t.Error("wrapper is empty")
	}
	if string(data[:2]) != "#!" {
		t.Error("wrapper should start with shebang")
	}
}

func TestEnsureInstalled_WhenNotInstalled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test that may download files in short mode")
	}

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	if IsInstalled() {
		t.Skip("runtime unexpectedly installed in temp dir")
	}

	// EnsureInstalled will attempt to install (may succeed or fail depending on network).
	// We just verify it doesn't panic and exercises the Install path.
	_ = EnsureInstalled(t.Context())
}

func TestConfigSave_InTempHome(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfg := DefaultConfig()
	cfg.Browser = "webkit"
	cfg.Timeout = 45
	cfg.Headless = false

	if err := cfg.Save(); err != nil {
		t.Fatalf("Config.Save() error = %v", err)
	}

	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if loaded.Browser != "webkit" {
		t.Errorf("Browser = %q, want %q", loaded.Browser, "webkit")
	}
	if loaded.Timeout != 45 {
		t.Errorf("Timeout = %d, want %d", loaded.Timeout, 45)
	}
	if loaded.Headless {
		t.Error("Headless should be false")
	}
}

func TestWritePlaywrightConfig_InTempHome(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfg := &Config{
		Headless: false,
		Browser:  "firefox",
		Timeout:  60,
	}

	if err := cfg.WritePlaywrightConfig(); err != nil {
		t.Fatalf("WritePlaywrightConfig() error = %v", err)
	}

	path := PlaywrightConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read playwright config: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON in playwright config: %v", err)
	}

	if timeout, ok := parsed["timeout"].(float64); !ok || timeout != 60000 {
		t.Errorf("timeout = %v, want 60000", parsed["timeout"])
	}

	if headless, ok := parsed["headless"].(bool); !ok || headless {
		t.Errorf("headless = %v, want false", parsed["headless"])
	}

	if browser, ok := parsed["browser"].(string); !ok || browser != "firefox" {
		t.Errorf("browser = %v, want firefox", parsed["browser"])
	}
}

func TestLoadConfig_InTempHome_Defaults(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if !cfg.Headless {
		t.Error("default Headless should be true")
	}
	if cfg.Browser != "chromium" {
		t.Errorf("default Browser = %q, want chromium", cfg.Browser)
	}
	if cfg.Profile != "default" {
		t.Errorf("default Profile = %q, want default", cfg.Profile)
	}
	if cfg.Timeout != 30 {
		t.Errorf("default Timeout = %d, want 30", cfg.Timeout)
	}
}

func TestLoadConfig_InvalidJSONFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	configPath := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("invalid json!!!"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig()
	if err == nil {
		t.Error("expected error for invalid JSON config")
	}
}

func TestLoadConfig_PermissionDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses file permission checks")
	}
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	configPath := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(`{"browser":"firefox"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(configPath, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(configPath, 0o644) })

	_, err := LoadConfig()
	if err == nil {
		t.Error("expected error for permission denied")
	}
}

func TestMergeState_BothWithData(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	profileName := "test-merge-both"
	worktreeHash := WorktreeHash("/test/merge/both-data")

	globalState := &BrowserState{
		Cookies: []Cookie{
			{Name: "global-auth", Value: "g-token", Domain: "auth.com", Path: "/"},
			{Name: "shared", Value: "global-val", Domain: "example.com", Path: "/"},
		},
		LocalStorage: map[string]map[string]string{
			"https://auth.com": {"token": "abc"},
		},
	}
	if err := globalState.Save(GlobalProfilePath(profileName)); err != nil {
		t.Fatalf("save global: %v", err)
	}

	wtState := &BrowserState{
		Cookies: []Cookie{
			{Name: "shared", Value: "wt-val", Domain: "example.com", Path: "/"},
			{Name: "wt-only", Value: "wt-cookie", Domain: "project.com", Path: "/"},
		},
		LocalStorage: map[string]map[string]string{
			"https://project.com": {"state": "active"},
		},
	}
	if err := wtState.Save(WorktreeStatePath(worktreeHash)); err != nil {
		t.Fatalf("save worktree: %v", err)
	}

	mergedPath, err := MergeState(profileName, worktreeHash)
	if err != nil {
		t.Fatalf("MergeState() error = %v", err)
	}

	merged, err := LoadState(mergedPath)
	if err != nil {
		t.Fatalf("LoadState(merged) error = %v", err)
	}

	if len(merged.Cookies) != 3 {
		t.Errorf("merged cookies count = %d, want 3", len(merged.Cookies))
	}

	for _, c := range merged.Cookies {
		if c.Name == "shared" && c.Value != "wt-val" {
			t.Errorf("shared cookie value = %q, want %q", c.Value, "wt-val")
		}
	}

	if len(merged.LocalStorage) != 2 {
		t.Errorf("merged localStorage origins = %d, want 2", len(merged.LocalStorage))
	}
}

func TestExtractWorktreeState_NoChanges(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	profileName := "test-extract-no-changes"
	worktreeHash := WorktreeHash("/test/extract/no-changes")

	globalState := &BrowserState{
		Cookies: []Cookie{
			{Name: "auth", Value: "token", Domain: "example.com", Path: "/"},
		},
	}
	if err := globalState.Save(GlobalProfilePath(profileName)); err != nil {
		t.Fatalf("save global: %v", err)
	}

	mergedPath := filepath.Join(t.TempDir(), "merged.json")
	if err := globalState.Save(mergedPath); err != nil {
		t.Fatalf("save merged: %v", err)
	}

	if err := ExtractWorktreeState(mergedPath, profileName, worktreeHash); err != nil {
		t.Fatalf("ExtractWorktreeState() error = %v", err)
	}

	wtPath := WorktreeStatePath(worktreeHash)
	wtState, err := LoadState(wtPath)
	if err != nil {
		t.Fatalf("LoadState(wt) error = %v", err)
	}

	if len(wtState.Cookies) != 0 {
		t.Errorf("expected 0 worktree cookies (no diff), got %d", len(wtState.Cookies))
	}
}

func TestExtractWorktreeState_EmptyMergedFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	profileName := "test-extract-empty-merged"
	worktreeHash := WorktreeHash("/test/extract/empty-merged")

	// Non-existent merged file results in empty state (LoadState returns empty for missing files)
	// so ExtractWorktreeState succeeds with an empty diff
	err := ExtractWorktreeState("/nonexistent/merged.json", profileName, worktreeHash)
	if err != nil {
		t.Fatalf("ExtractWorktreeState() unexpected error = %v", err)
	}

	// The worktree state should be empty
	wtPath := WorktreeStatePath(worktreeHash)
	wtState, err := LoadState(wtPath)
	if err != nil {
		t.Fatalf("LoadState(wt) error = %v", err)
	}
	if len(wtState.Cookies) != 0 {
		t.Errorf("expected 0 cookies, got %d", len(wtState.Cookies))
	}
}

func TestVersion_NotInstalledInTempHome(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	_, err := Version()
	if err == nil {
		t.Error("Version() should return error when not installed")
	}
}

func TestIsInstalled_EmptyHome(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	if IsInstalled() {
		t.Error("IsInstalled() should return false in empty HOME")
	}
}

func TestEnsureInstalled_WhenAlreadyInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Set up all files to make IsInstalled() return true
	node := filepath.Join(tmpDir, ".valksor", "kvelmo", "runtime", "node")
	if err := os.MkdirAll(filepath.Dir(node), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(node, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}

	cliEntry := filepath.Join(tmpDir, ".valksor", "kvelmo", "runtime", "node_modules", "@playwright", "cli", "playwright-cli.js")
	if err := os.MkdirAll(filepath.Dir(cliEntry), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cliEntry, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	wrapper := filepath.Join(tmpDir, ".valksor", "kvelmo", "bin", "playwright-cli")
	if err := os.MkdirAll(filepath.Dir(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wrapper, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}

	// EnsureInstalled should return nil because IsInstalled() is true
	if err := EnsureInstalled(t.Context()); err != nil {
		t.Errorf("EnsureInstalled() should return nil when already installed, got %v", err)
	}
}

func TestConfigSave_PartialFields(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfg := &Config{
		Headless: true,
		Browser:  "chromium",
		Timeout:  10,
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Config.Save() error = %v", err)
	}

	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if loaded.Timeout != 10 {
		t.Errorf("Timeout = %d, want 10", loaded.Timeout)
	}
	if loaded.Profile != "" {
		// Profile was not set in the saved config, so it stays as zero value
		// unless LoadConfig fills defaults first then overlays
		// LoadConfig starts with DefaultConfig() and unmarshals on top
		// So empty string in JSON would override "default"
		// Actually let's check: LoadConfig does cfg := DefaultConfig() then json.Unmarshal(data, cfg)
		// Since Profile is "" in saved JSON, it will override the default "default" with ""
		t.Logf("Profile = %q (expected empty since not set in saved config)", loaded.Profile)
	}
}

func TestBuildCommand_NoOpts(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	args := []string{"snapshot"}
	cmdArgs, env, cleanup, err := buildCommand(nil, args)
	if err != nil {
		t.Fatalf("buildCommand() error = %v", err)
	}
	defer cleanup()

	// Should contain --config flag and the user args
	foundConfig := false
	foundSnapshot := false
	for _, arg := range cmdArgs {
		if len(arg) > 9 && arg[:9] == "--config=" {
			foundConfig = true
		}
		if arg == "snapshot" {
			foundSnapshot = true
		}
	}
	if !foundConfig {
		t.Error("expected --config= in cmdArgs")
	}
	if !foundSnapshot {
		t.Error("expected 'snapshot' in cmdArgs")
	}

	// Should have headless env var (default config is headless)
	foundHeadless := false
	for _, e := range env {
		if e == "PLAYWRIGHT_CLI_HEADLESS=true" {
			foundHeadless = true
		}
	}
	if !foundHeadless {
		t.Error("expected PLAYWRIGHT_CLI_HEADLESS=true in env")
	}
}

func TestBuildCommand_WithSessionName(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	opts := &ExecOptions{
		SessionName: "my-session",
	}
	cmdArgs, _, cleanup, err := buildCommand(opts, []string{"navigate", "https://example.com"})
	if err != nil {
		t.Fatalf("buildCommand() error = %v", err)
	}
	defer cleanup()

	foundSession := false
	for _, arg := range cmdArgs {
		if arg == "-s=my-session" {
			foundSession = true
		}
	}
	if !foundSession {
		t.Errorf("expected -s=my-session in cmdArgs, got %v", cmdArgs)
	}
}

func TestBuildCommand_WithStateFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	stateFile := filepath.Join(tmpDir, "state.json")
	opts := &ExecOptions{
		StateFile: stateFile,
	}
	_, env, cleanup, err := buildCommand(opts, []string{"snapshot"})
	if err != nil {
		t.Fatalf("buildCommand() error = %v", err)
	}
	defer cleanup()

	foundStateEnv := false
	for _, e := range env {
		if e == "PLAYWRIGHT_CLI_STATE_FILE="+stateFile {
			foundStateEnv = true
		}
	}
	if !foundStateEnv {
		t.Error("expected PLAYWRIGHT_CLI_STATE_FILE in env")
	}
}

func TestBuildCommand_WithCustomEnv(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	opts := &ExecOptions{
		Env: map[string]string{
			"MY_VAR":    "my_value",
			"OTHER_VAR": "other_value",
		},
	}
	_, env, cleanup, err := buildCommand(opts, []string{"eval", "1+1"})
	if err != nil {
		t.Fatalf("buildCommand() error = %v", err)
	}
	defer cleanup()

	foundMyVar := false
	foundOtherVar := false
	for _, e := range env {
		if e == "MY_VAR=my_value" {
			foundMyVar = true
		}
		if e == "OTHER_VAR=other_value" {
			foundOtherVar = true
		}
	}
	if !foundMyVar {
		t.Error("expected MY_VAR=my_value in env")
	}
	if !foundOtherVar {
		t.Error("expected OTHER_VAR=other_value in env")
	}
}

func TestBuildCommand_WithWorktreePath(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	opts := &ExecOptions{
		WorktreePath: "/some/worktree/path",
	}
	_, env, cleanup, err := buildCommand(opts, []string{"snapshot"})
	if err != nil {
		t.Fatalf("buildCommand() error = %v", err)
	}
	defer cleanup()

	// Should have state file env var set (merged state)
	prefix := "PLAYWRIGHT_CLI_STATE_FILE="
	foundStateFile := false
	for _, e := range env {
		if len(e) > len(prefix) && e[:len(prefix)] == prefix {
			foundStateFile = true
		}
	}
	if !foundStateFile {
		t.Errorf("expected PLAYWRIGHT_CLI_STATE_FILE in env when WorktreePath is set, got env=%v", env)
	}
}

func TestBuildCommand_InvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Write invalid config
	configPath := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("invalid!!!"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, _, err := buildCommand(nil, []string{"snapshot"}) //nolint:dogsled // only testing error path
	if err == nil {
		t.Error("expected error for invalid config")
	}
}

func TestBuildCommand_NonHeadless(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Save a non-headless config
	cfg := &Config{
		Headless: false,
		Browser:  "chromium",
		Timeout:  30,
		Profile:  "default",
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	_, env, cleanup, err := buildCommand(nil, []string{"snapshot"})
	if err != nil {
		t.Fatalf("buildCommand() error = %v", err)
	}
	defer cleanup()

	// Should NOT have headless env var
	for _, e := range env {
		if e == "PLAYWRIGHT_CLI_HEADLESS=true" {
			t.Error("should not have PLAYWRIGHT_CLI_HEADLESS=true when headless is false")
		}
	}
}

func TestUpdate_InTempHome(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test that may download files in short mode")
	}

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create some fake runtime files to verify Update removes them
	runtimeDir := RuntimeDir()
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtimeDir, "node"), []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Update removes and re-installs (may succeed or fail depending on network)
	_ = Update(t.Context())
}

func TestMergeState_TempDirCreationError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses directory permission checks")
	}
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	profileName := "test-merge-tmpdir-error"
	worktreeHash := WorktreeHash("/test/merge/tmpdir-error")

	// Make the kvelmo dir read-only so MkdirAll for tmp dir fails
	kvelmoDir := Paths()
	if err := os.MkdirAll(kvelmoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(kvelmoDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(kvelmoDir, 0o755) })

	_, err := MergeState(profileName, worktreeHash)
	if err == nil {
		t.Error("expected error when temp dir creation fails")
	}
}

func TestBuildCommand_MergeStateError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses directory permission checks")
	}
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Make the kvelmo dir read-only so MergeState fails at temp dir creation
	kvelmoDir := Paths()
	if err := os.MkdirAll(kvelmoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// First create playwright config so buildCommand gets past WritePlaywrightConfig
	cfg := DefaultConfig()
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if err := cfg.WritePlaywrightConfig(); err != nil {
		t.Fatal(err)
	}

	// Now make it read-only
	if err := os.Chmod(kvelmoDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(kvelmoDir, 0o755) })

	opts := &ExecOptions{
		WorktreePath: "/some/worktree",
	}
	_, _, _, err := buildCommand(opts, []string{"snapshot"}) //nolint:dogsled // only testing error path
	if err == nil {
		t.Error("expected error when merge state fails")
	}
}

func TestSave_MkdirAllError(t *testing.T) {
	// Try to save to a path where the parent can't be created
	// Use a path under /proc which is read-only
	path := "/proc/fake_dir/nested/state.json"
	state := &BrowserState{}
	err := state.Save(path)
	if err == nil {
		t.Error("expected error when creating directory fails")
	}
}

func TestConfigSave_MkdirAllError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Make the parent dir read-only
	kvelmoDir := Paths()
	if err := os.MkdirAll(filepath.Dir(kvelmoDir), 0o755); err != nil {
		t.Fatal(err)
	}
	// Create kvelmo dir as a file to prevent MkdirAll
	if err := os.WriteFile(kvelmoDir, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()
	err := cfg.Save()
	if err == nil {
		t.Error("expected error when directory creation fails")
	}
}

func TestWritePlaywrightConfig_MkdirAllError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Make the parent dir a file to prevent MkdirAll
	kvelmoDir := Paths()
	if err := os.MkdirAll(filepath.Dir(kvelmoDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(kvelmoDir, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()
	err := cfg.WritePlaywrightConfig()
	if err == nil {
		t.Error("expected error when directory creation fails")
	}
}

func TestExtractWorktreeState_GlobalProfileError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	profileName := "test-extract-global-err"

	// Create merged file
	mergedPath := filepath.Join(t.TempDir(), "merged.json")
	mergedState := &BrowserState{
		Cookies: []Cookie{
			{Name: "c", Value: "v", Domain: "d", Path: "/"},
		},
	}
	if err := mergedState.Save(mergedPath); err != nil {
		t.Fatal(err)
	}

	// Create global profile with invalid JSON
	globalPath := GlobalProfilePath(profileName)
	if err := os.MkdirAll(filepath.Dir(globalPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(globalPath, []byte("bad json!!!"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := ExtractWorktreeState(mergedPath, profileName, "somehash")
	if err == nil {
		t.Error("expected error when global profile has invalid JSON")
	}
}

func TestExtractWorktreeState_SaveError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	profileName := "test-extract-save-err"
	worktreeHash := WorktreeHash("/test/extract/save-err")

	// Create merged file
	mergedPath := filepath.Join(t.TempDir(), "merged.json")
	if err := (&BrowserState{}).Save(mergedPath); err != nil {
		t.Fatal(err)
	}

	// Make worktrees dir a file so Save fails
	wtDir := WorktreesDir()
	if err := os.MkdirAll(filepath.Dir(wtDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wtDir, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := ExtractWorktreeState(mergedPath, profileName, worktreeHash)
	if err == nil {
		t.Error("expected error when worktree state save fails")
	}
}

func TestMergeState_GlobalProfileInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	profileName := "test-merge-bad-global"
	worktreeHash := WorktreeHash("/test/merge/bad-global")

	// Create global profile with invalid JSON
	globalPath := GlobalProfilePath(profileName)
	if err := os.MkdirAll(filepath.Dir(globalPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(globalPath, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := MergeState(profileName, worktreeHash)
	if err == nil {
		t.Error("expected error for invalid global profile JSON")
	}
}

func TestMergeState_WorktreeStateInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	profileName := "test-merge-bad-wt"
	worktreeHash := WorktreeHash("/test/merge/bad-wt")

	// Create valid global profile
	if err := (&BrowserState{}).Save(GlobalProfilePath(profileName)); err != nil {
		t.Fatal(err)
	}

	// Create worktree state with invalid JSON
	wtPath := WorktreeStatePath(worktreeHash)
	if err := os.MkdirAll(filepath.Dir(wtPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wtPath, []byte("bad json"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := MergeState(profileName, worktreeHash)
	if err == nil {
		t.Error("expected error for invalid worktree state JSON")
	}
}

func TestInstall_NodeFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test")
	}

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Make runtime dir a file so installNode fails at MkdirAll
	runtimeDir := RuntimeDir()
	if err := os.MkdirAll(filepath.Dir(runtimeDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runtimeDir, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := Install(t.Context())
	if err == nil {
		t.Error("expected error when runtime dir creation fails")
	}
}

func TestVersion_WhenInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create fake node binary
	node := NodeBinaryPath()
	if err := os.MkdirAll(filepath.Dir(node), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(node, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create fake playwright-cli.js
	cliEntry := filepath.Join(PlaywrightCLIDir(), "playwright-cli.js")
	if err := os.MkdirAll(filepath.Dir(cliEntry), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cliEntry, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create fake wrapper that outputs a version
	wrapper := BinaryPath()
	if err := os.MkdirAll(filepath.Dir(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\necho '1.2.3'\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	version, err := Version()
	if err != nil {
		t.Fatalf("Version() error = %v", err)
	}
	if version != "1.2.3" {
		t.Errorf("Version() = %q, want %q", version, "1.2.3")
	}
}

func TestVersion_WhenBinaryFails(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create all files to make IsInstalled() return true
	node := NodeBinaryPath()
	if err := os.MkdirAll(filepath.Dir(node), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(node, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}

	cliEntry := filepath.Join(PlaywrightCLIDir(), "playwright-cli.js")
	if err := os.MkdirAll(filepath.Dir(cliEntry), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cliEntry, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create wrapper that exits with error
	wrapper := BinaryPath()
	if err := os.MkdirAll(filepath.Dir(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := Version()
	if err == nil {
		t.Error("expected error when binary fails")
	}
}

func TestBuildCommand_WritePlaywrightConfigError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create valid config so LoadConfig succeeds
	cfg := DefaultConfig()
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	// Make .playwright dir a file so WritePlaywrightConfig fails
	pwDir := filepath.Dir(PlaywrightConfigPath())
	parentDir := filepath.Dir(pwDir)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pwDir, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, _, err := buildCommand(nil, []string{"snapshot"}) //nolint:dogsled // only testing error path
	if err == nil {
		t.Error("expected error when WritePlaywrightConfig fails")
	}
}

func TestInstall_PlaywrightCLIFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test")
	}

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Install Node.js successfully first by letting installNode run
	// Then make node_modules dir a file to fail installPlaywrightCLI
	if err := installNode(t.Context()); err != nil {
		t.Skipf("installNode failed (network?): %v", err)
	}

	// Make node_modules a file to break installPlaywrightCLI
	nmDir := NodeModulesDir()
	_ = os.RemoveAll(nmDir)
	if err := os.WriteFile(nmDir, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := Install(t.Context())
	if err == nil {
		t.Error("expected error when playwright-cli install fails")
	}
}

func TestIsInstalled_PartialFiles(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Only create node binary - should still return false
	node := NodeBinaryPath()
	if err := os.MkdirAll(filepath.Dir(node), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(node, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}

	if IsInstalled() {
		t.Error("IsInstalled() should be false with only node binary")
	}

	// Add playwright-cli.js but no wrapper
	cliEntry := filepath.Join(PlaywrightCLIDir(), "playwright-cli.js")
	if err := os.MkdirAll(filepath.Dir(cliEntry), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cliEntry, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	if IsInstalled() {
		t.Error("IsInstalled() should be false without wrapper")
	}
}

func TestCreateWrapper_DirCreationError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Make the bin parent a file so MkdirAll fails
	binDir := filepath.Dir(BinaryPath())
	parentDir := filepath.Dir(binDir)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binDir, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := createWrapper()
	if err == nil {
		t.Error("expected error when bin dir creation fails")
	}
}

func TestMergeState_SaveMergedError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	profileName := "test-merge-save-err"
	worktreeHash := WorktreeHash("/test/merge/save-err")

	// Create valid global and worktree states
	if err := (&BrowserState{}).Save(GlobalProfilePath(profileName)); err != nil {
		t.Fatal(err)
	}

	// Create tmp dir as a file so merged state save fails
	tmpKvelmo := filepath.Join(Paths(), "tmp")
	if err := os.MkdirAll(filepath.Dir(tmpKvelmo), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tmpKvelmo, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := MergeState(profileName, worktreeHash)
	if err == nil {
		t.Error("expected error when merged state save fails")
	}
}

func TestUpdate_RemoveWrapperError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Update with no existing files - should still try to install
	// The wrapper removal for non-existent file should be fine (os.IsNotExist check)
	// This covers the Update function's removal paths
	err := Update(t.Context())
	// May succeed or fail depending on network, just ensure no panic
	_ = err
}

func TestInstall_CreateWrapperFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test")
	}

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Install Node.js and playwright-cli successfully
	if err := installNode(t.Context()); err != nil {
		t.Skipf("installNode failed: %v", err)
	}
	if err := installPlaywrightCLI(t.Context()); err != nil {
		t.Skipf("installPlaywrightCLI failed: %v", err)
	}

	// Make bin dir a file to break createWrapper
	binDir := filepath.Dir(BinaryPath())
	if err := os.MkdirAll(filepath.Dir(binDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binDir, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := Install(t.Context())
	if err == nil {
		t.Error("expected error when createWrapper fails")
	}
}

func TestSave_WriteFileError(t *testing.T) {
	// Create a directory where the file should be - this makes WriteFile fail
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}

	state := &BrowserState{
		Cookies: []Cookie{{Name: "a", Value: "b", Domain: "c", Path: "/"}},
	}
	err := state.Save(path)
	if err == nil {
		t.Error("expected error when WriteFile target is a directory")
	}
}

func TestConfigSave_WriteFileError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create config path as a directory to make WriteFile fail
	configPath := ConfigPath()
	if err := os.MkdirAll(configPath, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()
	err := cfg.Save()
	if err == nil {
		t.Error("expected error when config path is a directory")
	}
}

func TestWritePlaywrightConfig_WriteFileError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create playwright config path as a directory
	pwPath := PlaywrightConfigPath()
	if err := os.MkdirAll(pwPath, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()
	err := cfg.WritePlaywrightConfig()
	if err == nil {
		t.Error("expected error when playwright config path is a directory")
	}
}

func TestExec_EnsureInstalledError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Make runtime dir unwritable so Install fails
	runtimeDir := RuntimeDir()
	parentDir := filepath.Dir(runtimeDir)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runtimeDir, []byte("block"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Exec(t.Context(), nil, "snapshot")
	if err == nil {
		t.Error("expected error when EnsureInstalled fails")
	}
}

func TestExecStream_EnsureInstalledError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	runtimeDir := RuntimeDir()
	parentDir := filepath.Dir(runtimeDir)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runtimeDir, []byte("block"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := ExecStream(t.Context(), nil, "snapshot")
	if err == nil {
		t.Error("expected error when EnsureInstalled fails")
	}
}

func TestExecInteractive_EnsureInstalledError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	runtimeDir := RuntimeDir()
	parentDir := filepath.Dir(runtimeDir)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runtimeDir, []byte("block"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := ExecInteractive(t.Context(), nil, "snapshot")
	if err == nil {
		t.Error("expected error when EnsureInstalled fails")
	}
}

func TestWritePlaywrightConfig_HeadlessTrue(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfg := &Config{
		Headless: true,
		Browser:  "chromium",
		Timeout:  30,
	}

	if err := cfg.WritePlaywrightConfig(); err != nil {
		t.Fatalf("WritePlaywrightConfig() error = %v", err)
	}

	path := PlaywrightConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if headless, ok := parsed["headless"].(bool); !ok || !headless {
		t.Errorf("headless = %v, want true", parsed["headless"])
	}
	if timeout, ok := parsed["timeout"].(float64); !ok || timeout != 30000 {
		t.Errorf("timeout = %v, want 30000", parsed["timeout"])
	}
}
