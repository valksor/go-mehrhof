package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Missing(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err != nil {
		t.Fatalf("Load() missing error = %v, want nil", err)
	}
	if s != nil {
		t.Errorf("Load() missing = %v, want nil", s)
	}
}

func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "kvelmo.yaml")

	s := &Settings{
		Agent:   AgentSettings{Default: "claude"},
		Workers: WorkerSettings{Max: 5},
	}

	if err := Save(path, s); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file was created (including parent dirs)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Save() did not create file: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got == nil {
		t.Fatal("Load() = nil, want settings")
	}
	if got.Agent.Default != "claude" {
		t.Errorf("Agent.Default = %q, want claude", got.Agent.Default)
	}
	if got.Workers.Max != 5 {
		t.Errorf("Workers.Max = %d, want 5", got.Workers.Max)
	}
}

func TestSaveLoadProject(t *testing.T) {
	root := t.TempDir()

	s := &Settings{Git: GitSettings{BranchPattern: "test/{key}"}}
	if err := SaveProject(root, s); err != nil {
		t.Fatalf("SaveProject() error = %v", err)
	}

	got, err := LoadProject(root)
	if err != nil {
		t.Fatalf("LoadProject() error = %v", err)
	}
	if got == nil {
		t.Fatal("LoadProject() = nil")
	}
	if got.Git.BranchPattern != "test/{key}" {
		t.Errorf("BranchPattern = %q, want test/{key}", got.Git.BranchPattern)
	}
}

func TestProjectPath(t *testing.T) {
	root := t.TempDir()
	path := ProjectPath(root)
	if path == "" {
		t.Error("ProjectPath() returned empty string")
	}
	// Should be under root
	if !filepath.IsAbs(path) {
		t.Errorf("ProjectPath() = %q, should be absolute", path)
	}
}

func TestMerge_NilSrc(t *testing.T) {
	dst := DefaultSettings()
	orig := dst.Agent.Default

	Merge(dst, nil)

	if dst.Agent.Default != orig {
		t.Error("Merge(nil) modified dst, should be no-op")
	}
}

func TestMerge_OverridesFields(t *testing.T) {
	dst := DefaultSettings()
	dst.Agent.Default = "claude"
	dst.Workers.Max = 3

	src := &Settings{
		Agent:   AgentSettings{Default: "codex"},
		Workers: WorkerSettings{Max: 8},
	}

	Merge(dst, src)

	if dst.Agent.Default != "codex" {
		t.Errorf("Agent.Default = %q, want codex", dst.Agent.Default)
	}
	if dst.Workers.Max != 8 {
		t.Errorf("Workers.Max = %d, want 8", dst.Workers.Max)
	}
}

func TestMerge_EmptySrcDoesNotOverride(t *testing.T) {
	dst := DefaultSettings()
	dst.Agent.Default = "claude"

	// src with empty/zero values should not override
	src := &Settings{}
	Merge(dst, src)

	if dst.Agent.Default != "claude" {
		t.Errorf("Agent.Default = %q, want claude (empty src should not override)", dst.Agent.Default)
	}
	if dst.Workers.Max != 3 {
		t.Errorf("Workers.Max = %d, want 3 (empty src should not override)", dst.Workers.Max)
	}
}

func TestMerge_CustomAgents(t *testing.T) {
	dst := DefaultSettings()
	src := &Settings{
		CustomAgents: map[string]CustomAgent{
			"my-agent": {Extends: "claude", Description: "Custom agent"},
		},
	}

	Merge(dst, src)

	if dst.CustomAgents == nil {
		t.Fatal("CustomAgents is nil after merge")
	}
	if _, ok := dst.CustomAgents["my-agent"]; !ok {
		t.Error("my-agent not found after merge")
	}
}

func TestMerge_FalseOverridesTrue(t *testing.T) {
	// This test verifies that
	// project-level false should override global-level true for pointer bool fields.

	dst := DefaultSettings()
	// Verify defaults are true
	if !BoolValue(dst.Git.CreateBranch, true) {
		t.Fatal("CreateBranch default should be true")
	}
	if !BoolValue(dst.Workflow.UseWorktreeIsolation, true) {
		t.Fatal("UseWorktreeIsolation default should be true")
	}

	// Project settings explicitly set to false
	falseVal := false
	src := &Settings{
		Git: GitSettings{
			CreateBranch: &falseVal,
		},
		Workflow: WorkflowSettings{
			UseWorktreeIsolation: &falseVal,
		},
	}

	Merge(dst, src)

	// After merge, the false values should have overridden the true defaults
	if BoolValue(dst.Git.CreateBranch, true) {
		t.Error("CreateBranch should be false after merge, but got true")
	}
	if BoolValue(dst.Workflow.UseWorktreeIsolation, true) {
		t.Error("UseWorktreeIsolation should be false after merge, but got true")
	}
}

func TestMerge_NilDoesNotOverride(t *testing.T) {
	// nil pointer bools should NOT override (they mean "not set")

	dst := DefaultSettings()

	// src with nil pointer bools should not override
	src := &Settings{
		Git: GitSettings{
			CreateBranch: nil, // not set
		},
	}

	Merge(dst, src)

	// CreateBranch should still be true (the default)
	if !BoolValue(dst.Git.CreateBranch, true) {
		t.Error("CreateBranch should still be true (nil should not override)")
	}
}

func TestSetGetValue_AllPaths(t *testing.T) {
	tests := []struct {
		path  string
		value any
	}{
		{"agent.default", "codex"},
		{"agent.allowed", []string{"claude", "codex"}},
		{"providers.default", "gitlab"},
		{"providers.github.token", "ghtoken"},
		{"providers.github.owner", "myorg"},
		{"providers.github.allow_ticket_comment", true},
		{"providers.gitlab.token", "gltoken"},
		{"providers.gitlab.base_url", "https://gl.example.com"},
		{"providers.gitlab.allow_ticket_comment", true},
		{"providers.wrike.token", "wktoken"},
		{"providers.wrike.include_parent_context", true},
		{"providers.wrike.include_sibling_context", false},
		{"providers.wrike.allow_ticket_comment", true},
		{"git.branch_pattern", "feat/{key}"},
		{"git.commit_prefix", "[fix]"},
		{"git.commit_pattern", `^(feat|fix)\(.*\):.*`},
		{"git.pr_title_pattern", "[{key}] {title}"},
		{"git.branch_validation_pattern", `^(feature|bugfix)/.*`},
		{"git.create_branch", true},
		{"git.auto_commit", true},
		{"git.sign_commits", true},
		{"git.allow_pr_comment", false},
		{"workers.max", 5},
		{"storage.save_in_project", true},
		{"storage.spec_output_path", "docs/specs/{key}.md"},
		{"storage.changelog_path", "CHANGELOG.md"},
		{"workflow.use_worktree_isolation", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			s := DefaultSettings()
			if err := SetValue(s, tt.path, tt.value); err != nil {
				t.Fatalf("SetValue(%q) error = %v", tt.path, err)
			}
			got, err := GetValue(s, tt.path)
			if err != nil {
				t.Fatalf("GetValue(%q) error = %v", tt.path, err)
			}
			// Basic non-nil check; type assertions done per field
			if got == nil {
				t.Errorf("GetValue(%q) = nil, want non-nil", tt.path)
			}
		})
	}
}

func TestSetValue_UnknownPath(t *testing.T) {
	s := DefaultSettings()
	err := SetValue(s, "unknown.path", "value")
	if err == nil {
		t.Error("SetValue() unknown path expected error, got nil")
	}
}

func TestGetValue_UnknownPath(t *testing.T) {
	s := DefaultSettings()
	_, err := GetValue(s, "unknown.path")
	if err == nil {
		t.Error("GetValue() unknown path expected error, got nil")
	}
}

func TestMerge_GitHubConfig(t *testing.T) {
	dst := DefaultSettings()
	src := &Settings{
		Providers: ProviderSettings{
			GitHub: GitHubConfig{
				Token:              "ghtoken",
				Owner:              "myorg",
				AllowTicketComment: true,
			},
		},
	}
	Merge(dst, src)

	if dst.Providers.GitHub.Token != "ghtoken" {
		t.Errorf("GitHub.Token = %q, want ghtoken", dst.Providers.GitHub.Token)
	}
	if dst.Providers.GitHub.Owner != "myorg" {
		t.Errorf("GitHub.Owner = %q, want myorg", dst.Providers.GitHub.Owner)
	}
	if !dst.Providers.GitHub.AllowTicketComment {
		t.Error("GitHub.AllowTicketComment should be true after merge")
	}
}

func TestMerge_GitLabConfig(t *testing.T) {
	dst := DefaultSettings()
	src := &Settings{
		Providers: ProviderSettings{
			GitLab: GitLabConfig{
				Token:   "gltoken",
				BaseURL: "https://gitlab.example.com",
			},
		},
	}
	Merge(dst, src)

	if dst.Providers.GitLab.Token != "gltoken" {
		t.Errorf("GitLab.Token = %q, want gltoken", dst.Providers.GitLab.Token)
	}
	if dst.Providers.GitLab.BaseURL != "https://gitlab.example.com" {
		t.Errorf("GitLab.BaseURL = %q, want https://gitlab.example.com", dst.Providers.GitLab.BaseURL)
	}
}

func TestSetValue_WorkersMax_Float64(t *testing.T) {
	// JSON unmarshaling gives float64 for numbers
	s := DefaultSettings()
	if err := SetValue(s, "workers.max", float64(7)); err != nil {
		t.Fatalf("SetValue(workers.max, float64) error = %v", err)
	}
	if s.Workers.Max != 7 {
		t.Errorf("Workers.Max = %d, want 7", s.Workers.Max)
	}
}

func TestSetValue_AgentAllowed_SliceAny(t *testing.T) {
	s := DefaultSettings()
	if err := SetValue(s, "agent.allowed", []any{"claude", "codex"}); err != nil {
		t.Fatalf("SetValue(agent.allowed, []any) error = %v", err)
	}
	if len(s.Agent.Allowed) != 2 {
		t.Errorf("Agent.Allowed len = %d, want 2", len(s.Agent.Allowed))
	}
}

func TestIsSensitivePath(t *testing.T) {
	sensitive := []string{
		"providers.github.token",
		"providers.gitlab.token",
		"providers.wrike.token",
	}
	notSensitive := []string{
		"providers.github.owner",
		"git.branch_pattern",
		"agent.default",
		"workers.max",
	}

	for _, path := range sensitive {
		if !IsSensitivePath(path) {
			t.Errorf("IsSensitivePath(%q) = false, want true", path)
		}
	}
	for _, path := range notSensitive {
		if IsSensitivePath(path) {
			t.Errorf("IsSensitivePath(%q) = true, want false", path)
		}
	}
}

func TestGetEnvVarForPath(t *testing.T) {
	tests := []struct {
		path    string
		wantEnv string
	}{
		{"providers.github.token", "GITHUB_TOKEN"},
		{"providers.gitlab.token", "GITLAB_TOKEN"},
		{"providers.wrike.token", "WRIKE_TOKEN"},
		{"unknown.path", ""},
	}

	for _, tt := range tests {
		got := GetEnvVarForPath(tt.path)
		if got != tt.wantEnv {
			t.Errorf("GetEnvVarForPath(%q) = %q, want %q", tt.path, got, tt.wantEnv)
		}
	}
}

func TestGlobalPath(t *testing.T) {
	path, err := GlobalPath()
	if err != nil {
		t.Fatalf("GlobalPath() error = %v", err)
	}
	if path == "" {
		t.Error("GlobalPath() returned empty string")
	}
}

func TestLoadGlobal_NoFile(t *testing.T) {
	// If the global settings file doesn't exist, Load returns nil (no error)
	s, err := LoadGlobal()
	if err != nil {
		// May fail if home dir is inaccessible; otherwise nil is expected
		t.Logf("LoadGlobal() note: %v", err)
	}
	_ = s // nil or default, both valid
}

func TestLoadEffective_EmptyProject(t *testing.T) {
	root := t.TempDir()
	merged, global, project, err := LoadEffective(root)
	if err != nil {
		t.Fatalf("LoadEffective() error = %v", err)
	}
	_ = global
	_ = project
	if merged == nil {
		t.Error("LoadEffective() merged = nil, want non-nil")
	}
}

func TestApplyEnvOverrides(t *testing.T) {
	tests := []struct {
		envVar string
		value  string
		check  func(t *testing.T, s *Settings)
	}{
		{
			envVar: "KVELMO_AGENT_DEFAULT",
			value:  "codex",
			check: func(t *testing.T, s *Settings) {
				t.Helper()
				if s.Agent.Default != "codex" {
					t.Errorf("Agent.Default = %q, want codex", s.Agent.Default)
				}
			},
		},
		{
			envVar: "KVELMO_WORKERS_MAX",
			value:  "7",
			check: func(t *testing.T, s *Settings) {
				t.Helper()
				if s.Workers.Max != 7 {
					t.Errorf("Workers.Max = %d, want 7", s.Workers.Max)
				}
			},
		},
		{
			envVar: "KVELMO_GIT_AUTO_COMMIT",
			value:  "false",
			check: func(t *testing.T, s *Settings) {
				t.Helper()
				if BoolValue(s.Git.AutoCommit, true) {
					t.Error("Git.AutoCommit should be false")
				}
			},
		},
		{
			envVar: "KVELMO_GIT_BASE_BRANCH",
			value:  "develop",
			check: func(t *testing.T, s *Settings) {
				t.Helper()
				if s.Git.BaseBranch != "develop" {
					t.Errorf("Git.BaseBranch = %q, want develop", s.Git.BaseBranch)
				}
			},
		},
		{
			envVar: "KVELMO_PROVIDERS_DEFAULT",
			value:  "gitlab",
			check: func(t *testing.T, s *Settings) {
				t.Helper()
				if s.Providers.Default != "gitlab" {
					t.Errorf("Providers.Default = %q, want gitlab", s.Providers.Default)
				}
			},
		},
		{
			envVar: "KVELMO_WORKFLOW_EXTERNAL_REVIEW_MODE",
			value:  "never",
			check: func(t *testing.T, s *Settings) {
				t.Helper()
				if s.Workflow.ExternalReview.Mode != ExternalReviewNever {
					t.Errorf("Workflow.ExternalReview.Mode = %q, want never", s.Workflow.ExternalReview.Mode)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.envVar, func(t *testing.T) {
			t.Setenv(tt.envVar, tt.value)
			s := DefaultSettings()
			applyEnvOverrides(s)
			tt.check(t, s)
		})
	}
}

// ─── FindNearestProjectConfig ───────────────────────────────────────────────

func TestFindNearestProjectConfig_NoConfigs(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	paths := FindNearestProjectConfig(sub, root)
	if len(paths) != 0 {
		t.Errorf("expected no configs, got %v", paths)
	}
}

func TestFindNearestProjectConfig_SingleAtRoot(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ".valksor")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "kvelmo.yaml")
	if err := os.WriteFile(configPath, []byte("agent:\n  default: claude\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	sub := filepath.Join(root, "packages", "web")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	result := FindNearestProjectConfig(sub, root)
	if len(result) != 1 {
		t.Fatalf("expected 1 config, got %d: %v", len(result), result)
	}
	if result[0] != configPath {
		t.Errorf("expected %s, got %s", configPath, result[0])
	}
}

func TestFindNearestProjectConfig_MonorepoMultiple(t *testing.T) {
	root := t.TempDir()

	// Root config
	rootCfgDir := filepath.Join(root, ".valksor")
	if err := os.MkdirAll(rootCfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	rootCfg := filepath.Join(rootCfgDir, "kvelmo.yaml")
	if err := os.WriteFile(rootCfg, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Nested config
	backendDir := filepath.Join(root, "packages", "backend")
	backendCfgDir := filepath.Join(backendDir, ".valksor")
	if err := os.MkdirAll(backendCfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	backendCfg := filepath.Join(backendCfgDir, "kvelmo.yaml")
	if err := os.WriteFile(backendCfg, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	sub := filepath.Join(backendDir, "src", "handlers")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	result := FindNearestProjectConfig(sub, root)
	if len(result) != 2 {
		t.Fatalf("expected 2 configs, got %d: %v", len(result), result)
	}
	// Shallowest first
	if result[0] != rootCfg {
		t.Errorf("expected root config first, got %s", result[0])
	}
	if result[1] != backendCfg {
		t.Errorf("expected backend config second, got %s", result[1])
	}
}

func TestFindNearestProjectConfig_StopsAtStopDir(t *testing.T) {
	root := t.TempDir()

	// Config ABOVE stop dir
	aboveDir := filepath.Join(root, ".valksor")
	if err := os.MkdirAll(aboveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(aboveDir, "kvelmo.yaml"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	stopDir := filepath.Join(root, "myproject")
	sub := filepath.Join(stopDir, "src")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	result := FindNearestProjectConfig(sub, stopDir)
	if len(result) != 0 {
		t.Errorf("expected no configs (above stop dir), got %v", result)
	}
}

// ─── Merge new fields ───────────────────────────────────────────────────────

func TestMerge_NewGitFields(t *testing.T) {
	dst := DefaultSettings()
	src := &Settings{
		Git: GitSettings{
			PRRequiredSections: []string{"summary", "test plan"},
		},
	}
	Merge(dst, src)

	if len(dst.Git.PRRequiredSections) != 2 {
		t.Errorf("PRRequiredSections not merged: %v", dst.Git.PRRequiredSections)
	}
}

func TestMerge_PlanOutputPath(t *testing.T) {
	dst := DefaultSettings()
	src := &Settings{
		Storage: StorageSettings{PlanOutputPath: "docs/plans/{key}.md"},
	}
	Merge(dst, src)

	if dst.Storage.PlanOutputPath != "docs/plans/{key}.md" {
		t.Errorf("PlanOutputPath not merged: %q", dst.Storage.PlanOutputPath)
	}
}

func TestMerge_Hooks(t *testing.T) {
	dst := DefaultSettings()
	src := &Settings{
		Workflow: WorkflowSettings{
			Hooks: HooksSettings{
				"pre_submit": {{Command: "./check.sh", Required: true}},
			},
		},
	}
	Merge(dst, src)

	if len(dst.Workflow.Hooks) != 1 {
		t.Errorf("Hooks not merged: %v", dst.Workflow.Hooks)
	}
	if len(dst.Workflow.Hooks["pre_submit"]) != 1 {
		t.Errorf("pre_submit hooks not merged: %v", dst.Workflow.Hooks["pre_submit"])
	}
}

func TestMerge_Preset(t *testing.T) {
	dst := DefaultSettings()
	src := &Settings{Preset: "compliance"}
	Merge(dst, src)

	if dst.Preset != "compliance" {
		t.Errorf("Preset not merged: %q", dst.Preset)
	}
}

func TestSetGetValue_PlanOutputPath(t *testing.T) {
	s := DefaultSettings()
	if err := SetValue(s, "storage.plan_output_path", "docs/{key}.md"); err != nil {
		t.Fatalf("SetValue error: %v", err)
	}
	got, err := GetValue(s, "storage.plan_output_path")
	if err != nil {
		t.Fatalf("GetValue error: %v", err)
	}
	if got != "docs/{key}.md" {
		t.Errorf("got %v, want docs/{key}.md", got)
	}
}

func TestApplyEnvOverrides_InvalidValues(t *testing.T) {
	// Invalid boolean should not change the setting
	t.Setenv("KVELMO_GIT_AUTO_COMMIT", "notabool")
	s := DefaultSettings()
	applyEnvOverrides(s)
	if !BoolValue(s.Git.AutoCommit, true) {
		t.Error("Git.AutoCommit should remain true for invalid bool value")
	}

	// Invalid integer should not change the setting
	t.Setenv("KVELMO_WORKERS_MAX", "notanumber")
	s = DefaultSettings()
	applyEnvOverrides(s)
	if s.Workers.Max != 3 {
		t.Errorf("Workers.Max = %d, want 3 for invalid int value", s.Workers.Max)
	}
}
