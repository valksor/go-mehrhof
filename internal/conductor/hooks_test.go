package conductor

import (
	"context"
	"testing"

	"github.com/valksor/kvelmo/settings"
)

func TestRunTransitionHooks_NoHooks(t *testing.T) {
	c, err := New(WithWorkDir(t.TempDir()))
	if err != nil {
		t.Fatalf("New conductor: %v", err)
	}
	c.ForceWorkUnit(&WorkUnit{ID: "test-1"})

	if err := c.RunTransitionHooks(context.Background(), EventSubmit); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunTransitionHooks_RequiredSuccess(t *testing.T) {
	s := settings.DefaultSettings()
	s.Workflow.Hooks = settings.HooksSettings{
		"pre_submit": {
			{Command: "true", Description: "always succeed", Required: true},
		},
	}
	c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
	if err != nil {
		t.Fatal(err)
	}
	c.ForceWorkUnit(&WorkUnit{ID: "test-1"})

	if err := c.RunTransitionHooks(context.Background(), EventSubmit); err != nil {
		t.Fatalf("required passing hook should not error: %v", err)
	}
}

func TestRunTransitionHooks_RequiredFailure(t *testing.T) {
	s := settings.DefaultSettings()
	s.Workflow.Hooks = settings.HooksSettings{
		"pre_submit": {
			{Command: "false", Description: "always fail", Required: true},
		},
	}
	c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
	if err != nil {
		t.Fatal(err)
	}
	c.ForceWorkUnit(&WorkUnit{ID: "test-1"})

	if err := c.RunTransitionHooks(context.Background(), EventSubmit); err == nil {
		t.Fatal("expected error from required failing hook")
	}
}

func TestRunTransitionHooks_OptionalFailure(t *testing.T) {
	s := settings.DefaultSettings()
	s.Workflow.Hooks = settings.HooksSettings{
		"pre_submit": {
			{Command: "false", Description: "optional fail", Required: false},
		},
	}
	c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
	if err != nil {
		t.Fatal(err)
	}
	c.ForceWorkUnit(&WorkUnit{ID: "test-1"})

	if err := c.RunTransitionHooks(context.Background(), EventSubmit); err != nil {
		t.Fatalf("optional hook failure should not return error: %v", err)
	}
}

func TestRunTransitionHooks_MixedRequiredOptional(t *testing.T) {
	s := settings.DefaultSettings()
	s.Workflow.Hooks = settings.HooksSettings{
		"pre_implement": {
			{Command: "true", Description: "passing required", Required: true},
			{Command: "false", Description: "failing optional", Required: false},
		},
	}
	c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
	if err != nil {
		t.Fatal(err)
	}
	c.ForceWorkUnit(&WorkUnit{ID: "test-1"})

	if err := c.RunTransitionHooks(context.Background(), EventImplement); err != nil {
		t.Fatalf("should succeed when required hooks pass and optional hooks fail: %v", err)
	}
}

func TestRunTransitionHooks_NoMatchingEvent(t *testing.T) {
	s := settings.DefaultSettings()
	s.Workflow.Hooks = settings.HooksSettings{
		"pre_submit": {
			{Command: "false", Description: "should not run", Required: true},
		},
	}
	c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
	if err != nil {
		t.Fatal(err)
	}
	c.ForceWorkUnit(&WorkUnit{ID: "test-1"})

	// Hooks for pre_submit should not run when event is plan
	if err := c.RunTransitionHooks(context.Background(), EventPlan); err != nil {
		t.Fatalf("hooks for different event should not run: %v", err)
	}
}

func TestListHooks_Empty(t *testing.T) {
	c, err := New(WithWorkDir(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}

	hooks := c.ListHooks()
	if len(hooks) > 0 {
		t.Fatalf("expected no hooks, got %v", hooks)
	}
}

func TestListHooks_Configured(t *testing.T) {
	s := settings.DefaultSettings()
	s.Workflow.Hooks = settings.HooksSettings{
		"pre_submit": {
			{Command: "echo check", Description: "Run checks", Required: true},
		},
		"pre_implement": {
			{Command: "echo lint", Required: false},
		},
	}
	c, err := New(WithWorkDir(t.TempDir()), WithSettings(s))
	if err != nil {
		t.Fatal(err)
	}

	hooks := c.ListHooks()
	if len(hooks) != 2 {
		t.Fatalf("expected 2 hook groups, got %d", len(hooks))
	}
	if len(hooks["pre_submit"]) != 1 {
		t.Fatalf("expected 1 pre_submit hook, got %d", len(hooks["pre_submit"]))
	}
	if !hooks["pre_submit"][0].Required {
		t.Fatal("expected pre_submit hook to be required")
	}
}
