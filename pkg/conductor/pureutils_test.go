package conductor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/valksor/kvelmo/pkg/settings"
)

// ─── runQualityGate ───────────────────────────────────────────────────────────

// noExternalReviewSettings returns settings with external review disabled, for use in
// quality gate tests that should not block on a user prompt.
func noExternalReviewSettings() *settings.Settings {
	s := settings.DefaultSettings()
	s.Workflow.ExternalReview.Mode = settings.ExternalReviewNever

	return s
}

func TestRunQualityGate_NoProjectFiles(t *testing.T) {
	// Empty temp dir — no go.mod, package.json, setup.py, or pyproject.toml
	c, _ := New(WithWorkDir(t.TempDir()), WithSettings(noExternalReviewSettings()))
	if err := c.runQualityGate(context.Background()); err != nil {
		t.Errorf("runQualityGate() on unknown project type = %v, want nil (should skip)", err)
	}
}

func TestRunQualityGate_NodeProjectNoScripts(t *testing.T) {
	dir := t.TempDir()
	// Create package.json with no lint/typecheck scripts
	pkg := map[string]any{
		"name":    "test",
		"scripts": map[string]string{"start": "node index.js"},
	}
	data, err := json.Marshal(pkg)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	c, _ := New(WithWorkDir(dir), WithSettings(noExternalReviewSettings()))
	// NodeScriptsChecker finds no lint/typecheck scripts → skips without running npm/bun
	if err := c.runQualityGate(context.Background()); err != nil {
		t.Errorf("runQualityGate() with no lint/typecheck scripts = %v, want nil", err)
	}
}

// ─── Submit / SaveSpecification / GenerateDeltaSpecification ─────────────────

func TestSubmit_NoWorkUnit(t *testing.T) {
	c, _ := New()
	err := c.Submit(context.Background(), false)
	if err == nil {
		t.Error("Submit() with no work unit should return error")
	}
}

func TestSaveSpecification_NoWorkUnit(t *testing.T) {
	c, _ := New()
	_, err := c.SaveSpecification("content")
	if err == nil {
		t.Error("SaveSpecification() with no work unit should return error")
	}
}

func TestGenerateDeltaSpecification_NoWorkUnit(t *testing.T) {
	c, _ := New()
	_, err := c.GenerateDeltaSpecification(context.Background(), "old", "new")
	if err == nil {
		t.Error("GenerateDeltaSpecification() with no work unit should return error")
	}
}

// ─── watchJob / saveJobSession ────────────────────────────────────────────────

func TestWatchJob_NoPool(t *testing.T) {
	// New() without WithPool → c.pool is nil → watchJob returns immediately
	c, _ := New()
	ctx := context.Background()
	// Should return without panic or blocking
	done := make(chan struct{})
	go func() {
		c.watchJob(ctx, "job-1", EventPlanDone)
		close(done)
	}()
	select {
	case <-done:
		// expected
	case <-ctx.Done():
		t.Error("watchJob() with nil pool timed out unexpectedly")
	}
}

func TestSaveJobSession_NoStore(t *testing.T) {
	// New() without store → c.store is nil → saveJobSession returns immediately
	c, _ := New()
	c.saveJobSession("job-1", "plan", "claude") // Should not panic
}
