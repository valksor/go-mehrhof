package conductor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/valksor/kvelmo/agent"
	"github.com/valksor/kvelmo/settings"
)

func TestProvisionWorktree_CopiesFiles(t *testing.T) {
	c, srcDir := setupExecConductor(t)

	// Create a config file in the source that the provisioner should copy.
	if err := os.WriteFile(filepath.Join(srcDir, ".env.local"), []byte("SECRET=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	worktreeDir := t.TempDir()
	s := c.GetEffectiveSettings()
	enabled := true
	s.Git.Provision.Enabled = &enabled
	s.Git.Provision.CopyPatterns = []string{".env.local"}
	c.cachedSettings.Store(s)

	// Should run without panicking; result is best-effort.
	c.provisionWorktree(context.Background(), s, srcDir, worktreeDir)
}

func TestProvisionWorktree_Disabled(t *testing.T) {
	c, srcDir := setupExecConductor(t)
	s := c.GetEffectiveSettings()
	disabled := false
	s.Git.Provision.Enabled = &disabled
	c.cachedSettings.Store(s)
	// Disabled → returns immediately.
	c.provisionWorktree(context.Background(), s, srcDir, t.TempDir())
}

func TestSubmit_WithTemplateAndCustomSectionsAndChangelog(t *testing.T) {
	c, _ := setupSubmitConductor(t)
	dir := c.worktree

	// Add a PR template so detectPRTemplate/fillPRTemplate run during submit.
	ghDir := filepath.Join(dir, ".github")
	if err := os.MkdirAll(ghDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ghDir, "PULL_REQUEST_TEMPLATE.md"), []byte("## Summary\n\n## Testing\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Stage the template so it is part of the branch.
	gitCmd(context.Background(), t, "-C", dir, "add", ".")
	gitCmd(context.Background(), t, "-C", dir, "commit", "-m", "add template")

	// Configure changelog + custom sections + branch deletion.
	s := c.GetEffectiveSettings()
	s.Storage.ChangelogPath = "CHANGELOG.md"
	s.Git.PRCustomSections = []settings.PRSection{{Title: "Extra", Content: "context"}}
	c.cachedSettings.Store(s)
	c.workUnit.ExternalID = "PROJ-1"

	if err := c.Submit(context.Background(), true); err != nil { // deleteBranch=true
		t.Fatalf("Submit error = %v", err)
	}
	if c.State() != StateSubmitted {
		t.Errorf("state = %s, want submitted", c.State())
	}
	// Changelog should have been written.
	if _, err := os.Stat(filepath.Join(dir, "CHANGELOG.md")); err != nil {
		t.Errorf("changelog not written: %v", err)
	}
}

func TestAttemptCIFix_FullFlow(t *testing.T) {
	c, _ := setupSubmitConductor(t)
	dir := c.worktree
	ctx := context.Background()

	// Make a change so the CI fix job produces a commit to push.
	if err := os.WriteFile(filepath.Join(dir, "cifix.go"), []byte("package main\n\nvar CI = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// attemptCIFix submits a fix job, waits for completion, commits, and pushes.
	if err := c.attemptCIFix(ctx, "test failed: TestFoo", 1, c.workUnit.Branch); err != nil {
		t.Fatalf("attemptCIFix error = %v", err)
	}
}

func TestRunQualityAutoFix_Exhausts(t *testing.T) {
	// Agent always errors → every fix attempt's job fails → loop exhausts.
	c, _ := setupExecConductor(t, agent.Event{Type: agent.EventError, Error: "fix failed"})
	c.machine.ForceState(StateReviewing)
	c.workUnit.HasImplemented = true

	s := c.GetEffectiveSettings()
	s.Workflow.AutoFix.MaxAttempts = 2
	c.cachedSettings.Store(s)

	err := c.runQualityAutoFix(context.Background(), errFixNeeded)
	if err == nil {
		t.Fatal("auto-fix should fail after exhausting attempts")
	}
	if !strings.Contains(err.Error(), "exhausted") {
		t.Errorf("error should mention exhaustion, got %v", err)
	}
	if c.GetAutoFixStatus().Active {
		t.Error("auto-fix state should be cleared after exhaustion")
	}
}

func TestSetupStatusSync(t *testing.T) {
	// setupStatusSync registers a listener; New() already calls it. Calling it
	// again should not panic and the listener fires on state change.
	c, err := New(WithWorkDir(t.TempDir()))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.setupStatusSync()
	c.ForceWorkUnit(&WorkUnit{
		ID:     "sync-1",
		Source: &Source{Provider: "github", Reference: "owner/repo#1"},
	})
	// Drive a transition; the sync listener runs in a goroutine (best-effort).
	c.machine.ForceState(StateNone)
	_ = c.machine.Dispatch(context.Background(), EventStart)
}
