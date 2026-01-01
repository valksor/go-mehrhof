//go:build !testbinary
// +build !testbinary

package commands

import (
	"testing"
)

func TestAgentsCommand_Properties(t *testing.T) {
	if agentsCmd.Use != "agents" {
		t.Errorf("Use = %q, want %q", agentsCmd.Use, "agents")
	}

	if agentsCmd.Short == "" {
		t.Error("Short description is empty")
	}

	if agentsCmd.Long == "" {
		t.Error("Long description is empty")
	}
}

func TestAgentsListCommand_Properties(t *testing.T) {
	if agentsListCmd.Use != "list" {
		t.Errorf("Use = %q, want %q", agentsListCmd.Use, "list")
	}

	if agentsListCmd.Short == "" {
		t.Error("Short description is empty")
	}

	if agentsListCmd.Long == "" {
		t.Error("Long description is empty")
	}

	if agentsListCmd.RunE == nil {
		t.Error("RunE not set")
	}
}

func TestAgentsExplainCommand_Properties(t *testing.T) {
	if agentsExplainCmd.Use != "explain" {
		t.Errorf("Use = %q, want %q", agentsExplainCmd.Use, "explain")
	}

	if agentsExplainCmd.Short == "" {
		t.Error("Short description is empty")
	}

	if agentsExplainCmd.Long == "" {
		t.Error("Long description is empty")
	}

	if agentsExplainCmd.RunE == nil {
		t.Error("RunE not set")
	}
}

func TestAgentsCommand_ShortDescription(t *testing.T) {
	expected := "Manage agents"
	if agentsCmd.Short != expected {
		t.Errorf("Short = %q, want %q", agentsCmd.Short, expected)
	}
}

func TestAgentsCommand_LongDescriptionContains(t *testing.T) {
	contains := []string{
		"AI agents",
		"user-defined aliases",
		"config.yaml",
	}

	for _, substr := range contains {
		if !containsString(agentsCmd.Long, substr) {
			t.Errorf("Long description does not contain %q", substr)
		}
	}
}

func TestAgentsListCommand_LongDescriptionContains(t *testing.T) {
	contains := []string{
		"NAME",
		"TYPE",
		"EXTENDS",
		"DESCRIPTION",
	}

	for _, substr := range contains {
		if !containsString(agentsListCmd.Long, substr) {
			t.Errorf("Long description does not contain %q", substr)
		}
	}
}

func TestAgentsExplainCommand_LongDescriptionContains(t *testing.T) {
	contains := []string{
		"priority",
		"CLI step-specific",
		"Task frontmatter",
		"Workspace config",
		"Auto-detection",
	}

	for _, substr := range contains {
		if !containsString(agentsExplainCmd.Long, substr) {
			t.Errorf("Long description does not contain %q", substr)
		}
	}
}

func TestAgentsCommand_HasSubcommands(t *testing.T) {
	subcommands := agentsCmd.Commands()
	if len(subcommands) < 2 {
		t.Errorf("agents command has %d subcommands, want at least 2", len(subcommands))
	}

	expectedSubcommands := []string{"list", "explain"}
	for _, exp := range expectedSubcommands {
		found := false
		for _, cmd := range subcommands {
			if cmd.Use == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("agents command missing subcommand %q", exp)
		}
	}
}

func TestAgentsCommand_RegisteredInRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "agents" {
			found = true
			break
		}
	}
	if !found {
		t.Error("agents command not registered in root command")
	}
}

func TestAgentsListCommand_Examples(t *testing.T) {
	if !containsString(agentsListCmd.Long, "mehr agents list") {
		t.Error("Long description does not contain example 'mehr agents list'")
	}
}

func TestAgentsExplainCommand_DocumentsPriorityLevels(t *testing.T) {
	// Should document all 7 priority levels
	levels := []string{
		"--agent-plan",
		"--agent",
		"agent_steps",
		"agent.default",
	}

	for _, level := range levels {
		if !containsString(agentsExplainCmd.Long, level) {
			t.Errorf("Long description does not document priority level %q", level)
		}
	}
}
