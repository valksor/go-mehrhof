package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withTempHome points KVELMO_HOME at a temp directory for the duration of the test.
// All settings code that consults paths.BaseDir() will resolve to this dir.
func withTempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("KVELMO_HOME", home)

	return home
}

func TestSaveGlobal_LoadGlobalRoundTrip(t *testing.T) {
	home := withTempHome(t)

	s := &Settings{
		Agent:   AgentSettings{Default: "codex"},
		Workers: WorkerSettings{Max: 9},
	}
	if err := SaveGlobal(s); err != nil {
		t.Fatalf("SaveGlobal: %v", err)
	}

	// File should be written into KVELMO_HOME.
	expectedPath := filepath.Join(home, "kvelmo.yaml")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Fatalf("SaveGlobal did not create %s: %v", expectedPath, err)
	}

	loaded, err := LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadGlobal returned nil after SaveGlobal")
	}
	if loaded.Agent.Default != "codex" {
		t.Errorf("Agent.Default = %q, want codex", loaded.Agent.Default)
	}
	if loaded.Workers.Max != 9 {
		t.Errorf("Workers.Max = %d, want 9", loaded.Workers.Max)
	}
}

func TestCascadeResolver_InvalidateCacheNoop(t *testing.T) {
	// Documented as a no-op for API compatibility; just verify the call is harmless.
	r := NewCascadeResolver()
	r.InvalidateCache()
	r.InvalidateCache()
}

func TestCascadeResolver_GetAllResolved(t *testing.T) {
	r := NewCascadeResolver()

	project := &Settings{Workers: WorkerSettings{Max: 11}}
	global := &Settings{Agent: AgentSettings{Default: "codex"}}

	all := r.GetAllResolved(project, global)
	if len(all) != len(DefaultDefinitions) {
		t.Errorf("GetAllResolved len = %d, want %d", len(all), len(DefaultDefinitions))
	}

	// Verify per-key source ordering: project overrides global which overrides default.
	bySource := map[string]string{}
	for _, rs := range all {
		bySource[rs.Key] = rs.Source
	}
	if bySource[KeyWorkersMax] != string(ScopeProject) {
		t.Errorf("WorkersMax source = %q, want project", bySource[KeyWorkersMax])
	}
	if bySource[KeyAgentDefault] != string(ScopeGlobal) {
		t.Errorf("AgentDefault source = %q, want global", bySource[KeyAgentDefault])
	}
	// A key not set in either falls back to default.
	if bySource[KeyGitBranchPattern] != "default" {
		t.Errorf("GitBranchPattern source = %q, want default", bySource[KeyGitBranchPattern])
	}
}

func TestCascadeResolver_GetResolved_AllConfigKeys(t *testing.T) {
	r := NewCascadeResolver()
	trueVal := true
	falseVal := false

	project := &Settings{
		Agent: AgentSettings{
			Default:  "codex",
			Strategy: "tree",
		},
		Git: GitSettings{
			BranchPattern: "feat/{key}",
			CommitPrefix:  "[fix]",
			AutoCommit:    &falseVal,
			CreateBranch:  &trueVal,
		},
		Workers: WorkerSettings{Max: 7},
		Workflow: WorkflowSettings{
			AutoAdvance:          &trueVal,
			UseWorktreeIsolation: &falseVal,
			HoldTheLine:          &falseVal,
		},
		Storage: StorageSettings{SaveInProject: &trueVal},
		Notify: NotifySettings{
			Enabled:  &trueVal,
			Terminal: &falseVal,
		},
	}

	tests := []struct {
		key      string
		wantSrc  string
		wantBool *bool  // for bool keys
		wantStr  string // for string keys
		wantInt  int    // for int keys
	}{
		{key: KeyAgentDefault, wantSrc: string(ScopeProject), wantStr: "codex"},
		{key: "agent.strategy", wantSrc: string(ScopeProject), wantStr: "tree"},
		{key: KeyGitBranchPattern, wantSrc: string(ScopeProject), wantStr: "feat/{key}"},
		{key: KeyGitCommitPrefix, wantSrc: string(ScopeProject), wantStr: "[fix]"},
		{key: KeyGitAutoCommit, wantSrc: string(ScopeProject), wantBool: &falseVal},
		{key: KeyGitCreateBranch, wantSrc: string(ScopeProject), wantBool: &trueVal},
		{key: KeyWorkersMax, wantSrc: string(ScopeProject), wantInt: 7},
		{key: "workflow.auto_advance", wantSrc: string(ScopeProject), wantBool: &trueVal},
		{key: KeyWorkflowUseWorktreeIsolation, wantSrc: string(ScopeProject), wantBool: &falseVal},
		{key: "workflow.hold_the_line", wantSrc: string(ScopeProject), wantBool: &falseVal},
		{key: KeyStorageSaveInProject, wantSrc: string(ScopeProject), wantBool: &trueVal},
		{key: "notify.enabled", wantSrc: string(ScopeProject), wantBool: &trueVal},
		{key: "notify.terminal", wantSrc: string(ScopeProject), wantBool: &falseVal},
	}

	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			got := r.GetResolved(tc.key, project, nil)
			if got.Source != tc.wantSrc {
				t.Errorf("source = %q, want %q", got.Source, tc.wantSrc)
			}
			switch {
			case tc.wantBool != nil:
				b, ok := got.Value.(bool)
				if !ok || b != *tc.wantBool {
					t.Errorf("bool value = %v, want %v", got.Value, *tc.wantBool)
				}
			case tc.wantStr != "":
				if got.Value != tc.wantStr {
					t.Errorf("string value = %v, want %v", got.Value, tc.wantStr)
				}
			case tc.wantInt != 0:
				if got.Value != tc.wantInt {
					t.Errorf("int value = %v, want %v", got.Value, tc.wantInt)
				}
			}
		})
	}
}

func TestCascadeResolver_GetResolved_UnknownKey(t *testing.T) {
	r := NewCascadeResolver()
	got := r.GetResolved("definitely.not.a.real.key", nil, nil)
	if got.Key != "" || got.Source != "" {
		t.Errorf("unknown key should return zero ResolvedSetting, got %+v", got)
	}
}

func TestSaveEnvVar_GlobalScope(t *testing.T) {
	home := withTempHome(t)

	if err := SaveEnvVar(ScopeGlobal, "", "GH_TOKEN", "ghp_test"); err != nil {
		t.Fatalf("SaveEnvVar(global): %v", err)
	}

	expectedPath := filepath.Join(home, ".env")
	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("read global .env: %v", err)
	}
	if !strings.Contains(string(data), "GH_TOKEN=ghp_test") {
		t.Errorf("global .env = %q, want contains GH_TOKEN=ghp_test", string(data))
	}
}

func TestSaveEnvVar_GlobalUpdate(t *testing.T) {
	withTempHome(t)

	if err := SaveEnvVar(ScopeGlobal, "", "TOK", "v1"); err != nil {
		t.Fatal(err)
	}
	if err := SaveEnvVar(ScopeGlobal, "", "TOK", "v2"); err != nil {
		t.Fatal(err)
	}

	path, _ := GlobalEnvPath()
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "TOK=v2") || strings.Contains(string(data), "TOK=v1") {
		t.Errorf("update failed; .env = %q", string(data))
	}
}

func TestLoadEnvMap_GlobalAndProjectMerge(t *testing.T) {
	home := withTempHome(t)

	// Global .env
	if err := os.WriteFile(filepath.Join(home, ".env"), []byte("SHARED=global_val\nGLOBAL_ONLY=g\n"), 0o600); err != nil {
		t.Fatalf("write global .env: %v", err)
	}

	root := t.TempDir()
	projDir := ProjectDirPath(root)
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	// Project .env overrides SHARED and adds PROJECT_ONLY.
	if err := os.WriteFile(filepath.Join(projDir, ".env"), []byte("SHARED=project_val\nPROJECT_ONLY=p\n"), 0o600); err != nil {
		t.Fatalf("write project .env: %v", err)
	}

	env, err := LoadEnvMap(root)
	if err != nil {
		t.Fatalf("LoadEnvMap: %v", err)
	}
	if env.Get("SHARED") != "project_val" {
		t.Errorf("SHARED = %q, want project override", env.Get("SHARED"))
	}
	if env.Get("GLOBAL_ONLY") != "g" {
		t.Error("GLOBAL_ONLY missing")
	}
	if env.Get("PROJECT_ONLY") != "p" {
		t.Error("PROJECT_ONLY missing")
	}
}

func TestLoadEnvMap_QuotingAndEscapes(t *testing.T) {
	home := withTempHome(t)

	// Test escape sequences inside double quotes, literal in single quotes,
	// blank lines, comments, and malformed lines.
	content := `# a comment
PLAIN=plain
DBL="with\ttab and\nnewline and\\backslash"
SGL='no\nescaping'
NO_VALUE
=missing_key
`
	if err := os.WriteFile(filepath.Join(home, ".env"), []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	env, err := LoadEnvMap("")
	if err != nil {
		t.Fatalf("LoadEnvMap: %v", err)
	}
	if env.Get("PLAIN") != "plain" {
		t.Errorf("PLAIN = %q", env.Get("PLAIN"))
	}
	if env.Get("DBL") != "with\ttab and\nnewline and\\backslash" {
		t.Errorf("DBL escapes not applied: %q", env.Get("DBL"))
	}
	if env.Get("SGL") != `no\nescaping` {
		t.Errorf("SGL should be literal: %q", env.Get("SGL"))
	}
	if env.Get("NO_VALUE") != "" {
		t.Errorf("NO_VALUE should be absent: %q", env.Get("NO_VALUE"))
	}
}

// ─── SetValue / GetValue sweep for every provider setting ───────────────────

func TestSetGetValue_AllProviderPaths(t *testing.T) {
	tests := []struct {
		path  string
		value any
	}{
		{"providers.github.status_sync", true},
		{"providers.gitlab.status_sync", true},
		{"providers.linear.team", "eng"},
		{"providers.linear.include_parent_context", true},
		{"providers.linear.include_sibling_context", true},
		{"providers.linear.allow_ticket_comment", true},
		{"providers.linear.status_sync", true},
		{"providers.jira.token", "jra"},
		{"providers.jira.email", "u@e.com"},
		{"providers.jira.base_url", "https://e.atlassian.net"},
		{"providers.jira.allow_ticket_comment", true},
		{"providers.jira.status_sync", true},
		{"providers.azuredevops.token", "az"},
		{"providers.azuredevops.base_url", "https://dev.azure.com"},
		{"providers.azuredevops.organization", "myorg"},
		{"providers.azuredevops.project", "myproj"},
		{"providers.azuredevops.repository", "myrepo"},
		{"providers.azuredevops.allow_ticket_comment", true},
		{"providers.azuredevops.status_sync", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			s := DefaultSettings()
			if err := SetValue(s, tt.path, tt.value); err != nil {
				t.Fatalf("SetValue(%q): %v", tt.path, err)
			}
			got, err := GetValue(s, tt.path)
			if err != nil {
				t.Fatalf("GetValue(%q): %v", tt.path, err)
			}
			if got == nil || got == "" {
				t.Errorf("GetValue(%q) = %v", tt.path, got)
			}
		})
	}
}

func TestSetValue_WrongTypes(t *testing.T) {
	s := DefaultSettings()

	// String setter rejects non-strings.
	if err := SetValue(s, "providers.github.token", 123); err == nil {
		t.Error("expected error setting bool path to int")
	}
	// Bool setter rejects strings (setBool accepts only bool/string-with-bool).
	if err := SetValue(s, "providers.github.status_sync", []string{"x"}); err == nil {
		t.Error("expected error setting bool path to []string")
	}
	// Int setter rejects bool.
	if err := SetValue(s, "workers.max", true); err == nil {
		t.Error("expected error setting int path to bool")
	}
}
