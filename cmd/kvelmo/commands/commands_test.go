package commands

import (
	"testing"
)

func TestStartCommand(t *testing.T) {
	cmd := StartCmd

	if cmd.Use != "start" {
		t.Errorf("Use = %s, want start", cmd.Use)
	}

	// Check --verbose flag exists
	f := cmd.Flags().Lookup("verbose")
	if f == nil {
		t.Error("--verbose flag should exist")
	}

	// Check --from flag exists
	f = cmd.Flags().Lookup("from")
	if f == nil {
		t.Error("--from flag should exist")
	}
}

func TestStatusCommand(t *testing.T) {
	cmd := StatusCmd

	if cmd.Use != "status" {
		t.Errorf("Use = %s, want status", cmd.Use)
	}

	// Check --timeout flag exists
	f := cmd.Flags().Lookup("timeout")
	if f == nil {
		t.Error("--timeout flag should exist")
	}

	// Check --verbose flag exists
	f = cmd.Flags().Lookup("verbose")
	if f == nil {
		t.Error("--verbose flag should exist")
	}
}

func TestStopCommand(t *testing.T) {
	cmd := StopCmd

	if cmd.Use != "stop" {
		t.Errorf("Use = %s, want stop", cmd.Use)
	}

	// Stop command doesn't have flags - it just stops the current operation
	if cmd.Short == "" {
		t.Error("Short description should exist")
	}
}

func TestShutdownCommand(t *testing.T) {
	cmd := ShutdownCmd

	if cmd.Use != "shutdown" {
		t.Errorf("Use = %s, want shutdown", cmd.Use)
	}

	// Check --timeout flag exists
	f := cmd.Flags().Lookup("timeout")
	if f == nil {
		t.Error("--timeout flag should exist")
	}
}

func TestProjectsCommand(t *testing.T) {
	cmd := ProjectsCmd

	if cmd.Use != "projects" {
		t.Errorf("Use = %s, want projects", cmd.Use)
	}

	// Check --timeout flag exists
	f := cmd.Flags().Lookup("timeout")
	if f == nil {
		t.Error("--timeout flag should exist")
	}

	// Check --verbose flag exists
	f = cmd.Flags().Lookup("verbose")
	if f == nil {
		t.Error("--verbose flag should exist")
	}
}

func TestWorkersCommand(t *testing.T) {
	cmd := WorkersCmd

	if cmd.Use != "workers" {
		t.Errorf("Use = %s, want workers", cmd.Use)
	}
}

func TestPlanCommand(t *testing.T) {
	cmd := PlanCmd

	if cmd.Use != "plan" {
		t.Errorf("Use = %s, want plan", cmd.Use)
	}
}

func TestImplementCommand(t *testing.T) {
	cmd := ImplementCmd

	if cmd.Use != "implement" {
		t.Errorf("Use = %s, want implement", cmd.Use)
	}
}

func TestReviewCommand(t *testing.T) {
	cmd := ReviewCmd

	if cmd.Use != "review" {
		t.Errorf("Use = %s, want review", cmd.Use)
	}
}

func TestSubmitCommand(t *testing.T) {
	cmd := SubmitCmd

	if cmd.Use != "submit" {
		t.Errorf("Use = %s, want submit", cmd.Use)
	}
}

func TestUndoCommand(t *testing.T) {
	cmd := UndoCmd

	if cmd.Use != "undo" {
		t.Errorf("Use = %s, want undo", cmd.Use)
	}
}

func TestRedoCommand(t *testing.T) {
	cmd := RedoCmd

	if cmd.Use != "redo" {
		t.Errorf("Use = %s, want redo", cmd.Use)
	}
}

func TestServeCommand(t *testing.T) {
	cmd := ServeCmd

	if cmd.Use != "serve" {
		t.Errorf("Use = %s, want serve", cmd.Use)
	}

	// Check --port flag exists
	f := cmd.Flags().Lookup("port")
	if f == nil {
		t.Error("--port flag should exist")
	}

	// Check --static flag exists
	f = cmd.Flags().Lookup("static")
	if f == nil {
		t.Error("--static flag should exist")
	}

	// Check --open flag exists
	f = cmd.Flags().Lookup("open")
	if f == nil {
		t.Error("--open flag should exist")
	}
}
