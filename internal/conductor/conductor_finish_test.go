package conductor

import (
	"context"
	"strings"
	"testing"
)

// ─── Finish ──────────────────────────────────────────────────────────────────

func TestConductorFinish_NoTask(t *testing.T) {
	c, _ := New()
	_, err := c.Finish(context.Background(), FinishOptions{})
	if err == nil {
		t.Error("Finish() with no task should return error")
	}
	if !strings.Contains(err.Error(), "no task loaded") {
		t.Errorf("Finish() error = %q, want 'no task loaded'", err.Error())
	}
}

func TestConductorFinish_WrongState(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{ID: "f1", Title: "T"})
	c.machine.ForceState(StateLoaded)
	_, err := c.Finish(context.Background(), FinishOptions{})
	if err == nil {
		t.Error("Finish() from loaded state without force should return error")
	}
	if !strings.Contains(err.Error(), "cannot finish") {
		t.Errorf("Finish() error = %q, want 'cannot finish'", err.Error())
	}
}

func TestConductorFinish_NoGit(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{ID: "f2", Title: "T"})
	c.machine.ForceState(StateSubmitted)
	_, err := c.Finish(context.Background(), FinishOptions{})
	if err == nil {
		t.Error("Finish() without git should return error")
	}
	if !strings.Contains(err.Error(), "git not available") {
		t.Errorf("Finish() error = %q, want 'git not available'", err.Error())
	}
}

// ─── ApprovePR ───────────────────────────────────────────────────────────────

func TestConductorApprovePR_NoTask(t *testing.T) {
	c, _ := New()
	err := c.ApprovePR(context.Background(), "lgtm")
	if err == nil {
		t.Error("ApprovePR() with no task should return error")
	}
}

func TestConductorApprovePR_WrongState(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{ID: "a1", Title: "T"})
	c.machine.ForceState(StateLoaded)
	err := c.ApprovePR(context.Background(), "lgtm")
	if err == nil {
		t.Error("ApprovePR() from wrong state should return error")
	}
	if !strings.Contains(err.Error(), "cannot approve") {
		t.Errorf("ApprovePR() error = %q, want 'cannot approve'", err.Error())
	}
}

func TestConductorApprovePR_NoProviders(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{ID: "a2", Title: "T"})
	c.machine.ForceState(StateSubmitted)
	c.providers = nil
	err := c.ApprovePR(context.Background(), "lgtm")
	if err == nil {
		t.Error("ApprovePR() with no providers should return error")
	}
	if !strings.Contains(err.Error(), "no provider configured") {
		t.Errorf("ApprovePR() error = %q, want 'no provider configured'", err.Error())
	}
}

// ─── MergePR ─────────────────────────────────────────────────────────────────

func TestConductorMergePR_NoTask(t *testing.T) {
	c, _ := New()
	err := c.MergePR(context.Background(), "rebase")
	if err == nil {
		t.Error("MergePR() with no task should return error")
	}
}

func TestConductorMergePR_WrongState(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{ID: "m1", Title: "T"})
	c.machine.ForceState(StateLoaded)
	err := c.MergePR(context.Background(), "rebase")
	if err == nil {
		t.Error("MergePR() from wrong state should return error")
	}
	if !strings.Contains(err.Error(), "cannot merge") {
		t.Errorf("MergePR() error = %q, want 'cannot merge'", err.Error())
	}
}

func TestConductorMergePR_NoProviders(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{ID: "m2", Title: "T"})
	c.machine.ForceState(StateSubmitted)
	c.providers = nil
	err := c.MergePR(context.Background(), "squash")
	if err == nil {
		t.Error("MergePR() with no providers should return error")
	}
	if !strings.Contains(err.Error(), "no provider configured") {
		t.Errorf("MergePR() error = %q, want 'no provider configured'", err.Error())
	}
}

// ─── Refresh ─────────────────────────────────────────────────────────────────

func TestConductorRefresh_NoTask(t *testing.T) {
	c, _ := New()
	_, err := c.Refresh(context.Background())
	if err == nil {
		t.Error("Refresh() with no task should return error")
	}
	if !strings.Contains(err.Error(), "no task loaded") {
		t.Errorf("Refresh() error = %q, want 'no task loaded'", err.Error())
	}
}
