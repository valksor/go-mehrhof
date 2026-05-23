package conductor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/valksor/kvelmo/internal/provider"
)

// fakeSubmitProvider is a minimal provider.SubmitProvider for testing the
// Submit network path without contacting a real forge.
type fakeSubmitProvider struct {
	name        string
	createCalls int
	commentLog  []string
}

func (f *fakeSubmitProvider) Name() string { return f.name }
func (f *fakeSubmitProvider) FetchTask(context.Context, string) (*provider.Task, error) {
	return &provider.Task{}, nil
}
func (f *fakeSubmitProvider) UpdateStatus(context.Context, string, string) error { return nil }

func (f *fakeSubmitProvider) CreatePR(_ context.Context, _ provider.PROptions) (*provider.PRResult, error) {
	f.createCalls++

	return &provider.PRResult{
		ID:     "fake/repo#1",
		Number: 1,
		URL:    "https://fake.example/pr/1",
		State:  "open",
	}, nil
}

func (f *fakeSubmitProvider) AddComment(_ context.Context, _ string, comment string) error {
	f.commentLog = append(f.commentLog, comment)

	return nil
}

// initSubmitRepo creates a working repo with a bare remote named origin and a
// feature branch, returning a conductor wired to it plus the fake provider.
func setupSubmitConductor(t *testing.T) (*Conductor, *fakeSubmitProvider) {
	t.Helper()

	ctx := context.Background()

	// Bare remote.
	remote := t.TempDir()
	gitCmd(ctx, t, "init", "--bare", remote)

	// Working clone.
	c, dir := setupExecConductor(t)
	gitCmd(ctx, t, "-C", dir, "remote", "add", "origin", remote)

	// Determine the default branch and push it so origin/<base> exists.
	base, err := c.git.DefaultBranch(ctx)
	if err != nil {
		t.Fatalf("DefaultBranch: %v", err)
	}
	gitCmd(ctx, t, "-C", dir, "push", "-u", "origin", base)

	// Create a feature branch with a commit.
	gitCmd(ctx, t, "-C", dir, "checkout", "-b", "feature/submit")
	if err := os.WriteFile(filepath.Join(dir, "submit.go"), []byte("package main\n\nvar S = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(ctx, t, "-C", dir, "add", ".")
	gitCmd(ctx, t, "-C", dir, "commit", "-m", "feature work")

	s := c.GetEffectiveSettings()
	s.Git.BaseBranch = base
	c.cachedSettings.Store(s)

	fp := &fakeSubmitProvider{name: "fake"}
	c.providers.Register(fp)

	c.workUnit.Branch = "feature/submit"
	c.workUnit.Source = &Source{Provider: "fake", Reference: "fake/repo#1", URL: "https://fake.example/task/1"}
	passed := true
	c.workUnit.QualityGatePassed = &passed
	c.machine.ForceState(StateReviewing)

	return c, fp
}

func TestSubmit_FullFlowCreatesPR(t *testing.T) {
	c, fp := setupSubmitConductor(t)

	if err := c.Submit(context.Background(), false); err != nil {
		t.Fatalf("Submit error = %v", err)
	}

	if c.State() != StateSubmitted {
		t.Errorf("state after submit = %s, want submitted", c.State())
	}
	if fp.createCalls != 1 {
		t.Errorf("CreatePR called %d times, want 1", fp.createCalls)
	}
	wu := c.GetWorkUnit()
	if wu.PRID != "fake/repo#1" {
		t.Errorf("PRID = %q, want fake/repo#1", wu.PRID)
	}
}

func TestSubmit_ReSubmitUsesExistingPR(t *testing.T) {
	c, fp := setupSubmitConductor(t)
	// Simulate an already-created PR.
	c.workUnit.PRID = "fake/repo#1"

	if err := c.Submit(context.Background(), false); err != nil {
		t.Fatalf("Submit error = %v", err)
	}
	// Re-submit pushes updates rather than creating a duplicate PR.
	if fp.createCalls != 0 {
		t.Errorf("CreatePR should not be called on re-submit, got %d", fp.createCalls)
	}
	if c.State() != StateSubmitted {
		t.Errorf("state = %s, want submitted", c.State())
	}
}
