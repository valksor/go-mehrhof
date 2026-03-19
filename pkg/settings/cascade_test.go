package settings

import (
	"testing"
)

func TestGetResolved_ProjectOverridesGlobal(t *testing.T) {
	r := NewCascadeResolver()

	global := &Settings{}
	global.Agent.Default = "codex"

	project := &Settings{}
	project.Agent.Default = "claude"

	got := r.GetResolved("agent.default", project, global)
	if got.Value != "claude" {
		t.Errorf("expected project value 'claude', got %v", got.Value)
	}
	if got.Source != "project" {
		t.Errorf("expected source 'project', got %q", got.Source)
	}
}

func TestGetResolved_FallsBackToGlobal(t *testing.T) {
	r := NewCascadeResolver()

	global := &Settings{}
	global.Agent.Default = "codex"

	project := &Settings{} // agent.default is empty

	got := r.GetResolved("agent.default", project, global)
	if got.Value != "codex" {
		t.Errorf("expected global value 'codex', got %v", got.Value)
	}
	if got.Source != "global" {
		t.Errorf("expected source 'global', got %q", got.Source)
	}
}

func TestGetResolved_FallsBackToDefault(t *testing.T) {
	r := NewCascadeResolver()

	global := &Settings{}
	project := &Settings{}

	got := r.GetResolved("agent.default", project, global)
	if got.Value != "claude" {
		t.Errorf("expected default value 'claude', got %v", got.Value)
	}
	if got.Source != "default" {
		t.Errorf("expected source 'default', got %q", got.Source)
	}
}

func TestGetResolved_NilProject(t *testing.T) {
	r := NewCascadeResolver()

	global := &Settings{}
	global.Agent.Default = "codex"

	got := r.GetResolved("agent.default", nil, global)
	if got.Value != "codex" {
		t.Errorf("expected global value 'codex', got %v", got.Value)
	}
	if got.Source != "global" {
		t.Errorf("expected source 'global', got %q", got.Source)
	}
}

func TestGetResolved_NilBoth(t *testing.T) {
	r := NewCascadeResolver()

	got := r.GetResolved("agent.default", nil, nil)
	if got.Value != "claude" {
		t.Errorf("expected default 'claude', got %v", got.Value)
	}
	if got.Source != "default" {
		t.Errorf("expected source 'default', got %q", got.Source)
	}
}

func TestGetResolved_UnknownKey(t *testing.T) {
	r := NewCascadeResolver()

	got := r.GetResolved("nonexistent.key", nil, nil)
	if got.Key != "" {
		t.Errorf("expected empty key for unknown setting, got %q", got.Key)
	}
	if got.Value != nil {
		t.Errorf("expected nil value for unknown setting, got %v", got.Value)
	}
	if got.Source != "" {
		t.Errorf("expected empty source for unknown setting, got %q", got.Source)
	}
}

func TestGetResolved_PointerBoolFields(t *testing.T) {
	r := NewCascadeResolver()

	falseVal := false
	trueVal := true

	global := &Settings{}
	global.Git.AutoCommit = &trueVal

	project := &Settings{}
	project.Git.AutoCommit = &falseVal

	got := r.GetResolved("git.auto_commit", project, global)
	if got.Value != false {
		t.Errorf("expected project value false, got %v", got.Value)
	}
	if got.Source != "project" {
		t.Errorf("expected source 'project', got %q", got.Source)
	}
}

func TestGetResolved_IntField(t *testing.T) {
	r := NewCascadeResolver()

	global := &Settings{}
	global.Workers.Max = 5

	got := r.GetResolved("workers.max", nil, global)
	if got.Value != 5 {
		t.Errorf("expected global value 5, got %v", got.Value)
	}
	if got.Source != "global" {
		t.Errorf("expected source 'global', got %q", got.Source)
	}
}

func TestGetAllResolved(t *testing.T) {
	r := NewCascadeResolver()

	global := &Settings{}
	global.Agent.Default = "codex"

	results := r.GetAllResolved(nil, global)
	if len(results) != len(DefaultDefinitions) {
		t.Errorf("expected %d results, got %d", len(DefaultDefinitions), len(results))
	}

	// Check that results are sorted by key.
	for i := 1; i < len(results); i++ {
		if results[i].Key < results[i-1].Key {
			t.Errorf("results not sorted: %q before %q", results[i-1].Key, results[i].Key)
		}
	}

	// Find agent.default and verify it resolved from global.
	found := false
	for _, rs := range results {
		if rs.Key == "agent.default" {
			found = true
			if rs.Value != "codex" {
				t.Errorf("expected agent.default = 'codex', got %v", rs.Value)
			}
			if rs.Source != "global" {
				t.Errorf("expected source 'global', got %q", rs.Source)
			}
		}
	}
	if !found {
		t.Error("agent.default not found in GetAllResolved results")
	}
}

func TestRegisterDefinitions(t *testing.T) {
	r := NewCascadeResolver()

	custom := SettingDefinition{
		Key:          "custom.setting",
		DisplayName:  "Custom Setting",
		DataType:     "string",
		DefaultValue: "hello",
		Scopes:       []string{"global"},
		Category:     "custom",
	}
	r.RegisterDefinitions(custom)

	got := r.GetResolved("custom.setting", nil, nil)
	if got.Value != "hello" {
		t.Errorf("expected default 'hello', got %v", got.Value)
	}
	if got.Source != "default" {
		t.Errorf("expected source 'default', got %q", got.Source)
	}
}

func TestInvalidateCache(t *testing.T) {
	r := NewCascadeResolver()

	global := &Settings{}
	global.Agent.Default = "codex"

	// Populate cache.
	got1 := r.GetResolved("agent.default", nil, global)
	if got1.Value != "codex" {
		t.Fatalf("expected 'codex', got %v", got1.Value)
	}

	// InvalidateCache is a no-op now but should not panic.
	r.InvalidateCache()

	// Re-resolve should still work.
	got2 := r.GetResolved("agent.default", nil, global)
	if got2.Value != "codex" {
		t.Fatalf("expected 'codex' after InvalidateCache, got %v", got2.Value)
	}
}

func TestGetResolved_StrategyField(t *testing.T) {
	r := NewCascadeResolver()

	project := &Settings{}
	project.Agent.Strategy = "iterative"

	got := r.GetResolved("agent.strategy", project, nil)
	if got.Value != "iterative" {
		t.Errorf("expected 'iterative', got %v", got.Value)
	}
	if got.Source != "project" {
		t.Errorf("expected source 'project', got %q", got.Source)
	}
}

func TestGetResolved_WorktreeIsolationPointerBool(t *testing.T) {
	r := NewCascadeResolver()

	falseVal := false
	project := &Settings{}
	project.Workflow.UseWorktreeIsolation = &falseVal

	got := r.GetResolved("workflow.use_worktree_isolation", project, nil)
	if got.Value != false {
		t.Errorf("expected false, got %v", got.Value)
	}
	if got.Source != "project" {
		t.Errorf("expected source 'project', got %q", got.Source)
	}
}

func TestGetResolved_StorageSaveInProject(t *testing.T) {
	r := NewCascadeResolver()

	trueVal := true
	global := &Settings{}
	global.Storage.SaveInProject = &trueVal

	got := r.GetResolved("storage.save_in_project", nil, global)
	if got.Value != true {
		t.Errorf("expected true, got %v", got.Value)
	}
	if got.Source != "global" {
		t.Errorf("expected source 'global', got %q", got.Source)
	}
}
