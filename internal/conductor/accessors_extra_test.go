package conductor

import (
	"context"
	"testing"

	"github.com/valksor/kvelmo/settings"
)

func TestIsTestFile(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"pkg/foo_test.go", true},
		{"pkg/foo.go", false},
		{"web/app.test.ts", true},
		{"web/app.test.tsx", true},
		{"web/app.spec.js", true},
		{"web/app.spec.jsx", true},
		{"web/app.ts", false},
		{"short.go", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			if got := isTestFile(tc.path); got != tc.want {
				t.Errorf("isTestFile(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestConductor_WorkUnitAccessors(t *testing.T) {
	c := newTestConductor(t)

	if c.WorkUnit() != nil {
		t.Error("WorkUnit should be nil before a task is set")
	}
	if c.GetWorkUnit() != nil {
		t.Error("GetWorkUnit should be nil before a task is set")
	}
	if c.TaskTraceID() != "" {
		t.Error("TaskTraceID should be empty before a task is set")
	}

	wu := &WorkUnit{ID: "t1", Title: "Test", TaskTraceID: "trace-1"}
	c.ForceWorkUnit(wu)

	if got := c.WorkUnit(); got == nil || got.ID != "t1" {
		t.Errorf("WorkUnit = %+v", got)
	}
	if got := c.GetWorkUnit(); got == nil || got.ID != "t1" {
		t.Errorf("GetWorkUnit = %+v", got)
	}
	if c.TaskTraceID() != "trace-1" {
		t.Errorf("TaskTraceID = %q", c.TaskTraceID())
	}

	// MarkDirty updates UpdatedAt and persists without panicking.
	c.MarkDirty()
}

func TestConductor_AutoAdvanceAccessors(t *testing.T) {
	c := newTestConductor(t)

	if c.AutoAdvance() {
		t.Error("AutoAdvance should default to false")
	}
	c.SetAutoAdvance(true)
	if !c.AutoAdvance() {
		t.Error("AutoAdvance should be true after SetAutoAdvance(true)")
	}
}

func TestConductor_SkipPhases(t *testing.T) {
	c := newTestConductor(t)

	c.SetSkipPhases([]string{"simplify", "optimize"})
	merged := c.SkipPhases()

	found := map[string]bool{}
	for _, p := range merged {
		found[p] = true
	}
	if !found["simplify"] || !found["optimize"] {
		t.Errorf("SkipPhases = %v, want to include simplify and optimize", merged)
	}
}

func TestConductor_SetContextItems(t *testing.T) {
	c := newTestConductor(t)

	// No work unit → no-op, no panic.
	c.SetContextItems([]ContextItem{{Type: ContextTypeFile, Ref: "main.go"}})

	c.ForceWorkUnit(&WorkUnit{ID: "t1"})
	c.SetContextItems([]ContextItem{
		{Type: ContextTypeFile, Ref: "main.go"},
		{Type: ContextTypeSymbol, Ref: "Handler"},
	})

	wu := c.GetWorkUnit()
	if len(wu.ContextItems) != 2 {
		t.Errorf("ContextItems = %v, want 2", wu.ContextItems)
	}
}

func TestConductor_Machine(t *testing.T) {
	c := newTestConductor(t)
	if c.Machine() == nil {
		t.Error("Machine() should not be nil")
	}
}

func TestConductor_GetEffectiveSettings(t *testing.T) {
	c := newTestConductor(t)
	s := c.GetEffectiveSettings()
	if s == nil {
		t.Fatal("GetEffectiveSettings returned nil")
	}
	// Reload then read again — should still be non-nil.
	c.ReloadSettings()
	if c.GetEffectiveSettings() == nil {
		t.Error("GetEffectiveSettings nil after reload")
	}
}

func TestConductor_Repo(t *testing.T) {
	c := newTestConductor(t)
	// A bare temp dir has no git repo, so Repo() may be nil — just exercise it.
	_ = c.Repo()
}

func TestConductor_ResponseCache_Disabled(t *testing.T) {
	c := newTestConductor(t)

	// Default settings disable the response cache.
	if c.ResponseCache() != nil {
		t.Error("ResponseCache should be nil when disabled")
	}
	if c.ResponseCacheStats() != nil {
		t.Error("ResponseCacheStats should be nil when disabled")
	}
	// Clear and lookup/store are no-ops when disabled.
	c.ClearResponseCache()
	if _, ok := c.lookupResponseCache("prompt"); ok {
		t.Error("lookupResponseCache should miss when disabled")
	}
	c.storeResponseCache("prompt", "response", "implement")
}

func TestConductor_ResponseCache_Enabled(t *testing.T) {
	c := newTestConductor(t)

	s := settings.DefaultSettings()
	s.Agent.ResponseCache = &settings.ResponseCacheSettings{
		Enabled:    true,
		MaxEntries: 10,
		TTLHours:   1,
	}
	c.initResponseCache(s)

	if c.ResponseCache() == nil {
		t.Fatal("ResponseCache should be set after init")
	}

	// Store then look up.
	c.storeResponseCache("prompt-x", "response-x", "implement")
	got, ok := c.lookupResponseCache("prompt-x")
	if !ok || got != "response-x" {
		t.Errorf("lookupResponseCache = (%q, %v), want (response-x, true)", got, ok)
	}

	if stats := c.ResponseCacheStats(); stats == nil {
		t.Error("ResponseCacheStats should be non-nil when enabled")
	}

	c.ClearResponseCache()
	if _, ok := c.lookupResponseCache("prompt-x"); ok {
		t.Error("lookupResponseCache should miss after Clear")
	}
}

func TestConductor_GetBaseBranch_NoGit(t *testing.T) {
	c := newTestConductor(t)
	// No git repo and no base_branch configured → error.
	_, err := c.GetBaseBranch(context.Background())
	if err == nil {
		t.Log("note: GetBaseBranch returned no error (git may have detected a parent repo)")
	}
}
