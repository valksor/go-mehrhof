package help

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestGetHelpContext_Caching(t *testing.T) {
	// Reset any cached context first
	ResetContext()

	// First call should load context
	ctx1 := GetHelpContext()
	if ctx1 == nil {
		t.Fatal("GetHelpContext should never return nil")
	}

	// Second call should return the same cached context
	ctx2 := GetHelpContext()
	if ctx1 != ctx2 {
		t.Error("GetHelpContext should return cached context on subsequent calls")
	}
}

func TestGetHelpContext_NonNil(t *testing.T) {
	// Ensure GetHelpContext never returns nil, even with no workspace
	ResetContext()
	ctx := GetHelpContext()
	if ctx == nil {
		t.Fatal("GetHelpContext should never return nil")
	}
	// An empty context is valid
	if ctx.HasActiveTask {
		t.Error("expected HasActiveTask to be false in isolated test")
	}
}

func TestResetContext(t *testing.T) {
	// Load context
	_ = GetHelpContext()

	// Reset context
	ResetContext()

	// Get fresh context - should be a new instance
	ctx1 := GetHelpContext()
	ctx2 := GetHelpContext()

	if ctx1 != ctx2 {
		t.Error("After ResetContext, GetHelpContext should still cache, but with a fresh instance")
	}
}

func TestFilterAvailable(t *testing.T) {
	tests := []struct {
		name     string
		ctx      *HelpContext
		commands []*cobra.Command
		wantLen  int
	}{
		{
			name: "always available commands",
			ctx:  &HelpContext{},
			commands: []*cobra.Command{
				{Use: "start", Run: func(_ *cobra.Command, _ []string) {}},
				{Use: "auto", Run: func(_ *cobra.Command, _ []string) {}},
				{Use: "help", Run: func(_ *cobra.Command, _ []string) {}},
			},
			wantLen: 3,
		},
		{
			name: "needs active task - none available",
			ctx:  &HelpContext{HasActiveTask: false},
			commands: []*cobra.Command{
				{Use: "status", Run: func(_ *cobra.Command, _ []string) {}},
				{Use: "guide", Run: func(_ *cobra.Command, _ []string) {}},
			},
			wantLen: 0,
		},
		{
			name: "needs active task - all available",
			ctx:  &HelpContext{HasActiveTask: true},
			commands: []*cobra.Command{
				{Use: "status", Run: func(_ *cobra.Command, _ []string) {}},
				{Use: "guide", Run: func(_ *cobra.Command, _ []string) {}},
			},
			wantLen: 2,
		},
		{
			name: "needs specifications - not available",
			ctx:  &HelpContext{HasActiveTask: true, HasSpecifications: false},
			commands: []*cobra.Command{
				{Use: "implement", Run: func(_ *cobra.Command, _ []string) {}},
				{Use: "review", Run: func(_ *cobra.Command, _ []string) {}},
			},
			wantLen: 0,
		},
		{
			name: "needs specifications - available",
			ctx:  &HelpContext{HasActiveTask: true, HasSpecifications: true},
			commands: []*cobra.Command{
				{Use: "implement", Run: func(_ *cobra.Command, _ []string) {}},
				{Use: "review", Run: func(_ *cobra.Command, _ []string) {}},
			},
			wantLen: 2,
		},
		{
			name: "needs git - not available",
			ctx:  &HelpContext{HasActiveTask: true, UseGit: false},
			commands: []*cobra.Command{
				{Use: "undo", Run: func(_ *cobra.Command, _ []string) {}},
				{Use: "redo", Run: func(_ *cobra.Command, _ []string) {}},
			},
			wantLen: 0,
		},
		{
			name: "needs git - available",
			ctx:  &HelpContext{HasActiveTask: true, UseGit: true},
			commands: []*cobra.Command{
				{Use: "undo", Run: func(_ *cobra.Command, _ []string) {}},
				{Use: "redo", Run: func(_ *cobra.Command, _ []string) {}},
			},
			wantLen: 2,
		},
		{
			name: "mixed availability",
			ctx:  &HelpContext{HasActiveTask: true},
			commands: []*cobra.Command{
				{Use: "status", Run: func(_ *cobra.Command, _ []string) {}},    // needs active task - available
				{Use: "implement", Run: func(_ *cobra.Command, _ []string) {}}, // needs specs - not available
				{Use: "help", Run: func(_ *cobra.Command, _ []string) {}},      // always available
			},
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterAvailable(tt.commands, tt.ctx)
			if len(result) != tt.wantLen {
				t.Errorf("FilterAvailable() returned %d commands, want %d", len(result), tt.wantLen)
			}
		})
	}
}

func TestFilterUnavailable(t *testing.T) {
	tests := []struct {
		name     string
		ctx      *HelpContext
		commands []*cobra.Command
		wantLen  int
	}{
		{
			name: "always available - none unavailable",
			ctx:  &HelpContext{},
			commands: []*cobra.Command{
				{Use: "start", Run: func(_ *cobra.Command, _ []string) {}},
				{Use: "help", Run: func(_ *cobra.Command, _ []string) {}},
			},
			wantLen: 0,
		},
		{
			name: "needs active task - all unavailable",
			ctx:  &HelpContext{HasActiveTask: false},
			commands: []*cobra.Command{
				{Use: "status", Run: func(_ *cobra.Command, _ []string) {}},
				{Use: "guide", Run: func(_ *cobra.Command, _ []string) {}},
			},
			wantLen: 2,
		},
		{
			name: "needs active task - none unavailable",
			ctx:  &HelpContext{HasActiveTask: true},
			commands: []*cobra.Command{
				{Use: "status", Run: func(_ *cobra.Command, _ []string) {}},
				{Use: "guide", Run: func(_ *cobra.Command, _ []string) {}},
			},
			wantLen: 0,
		},
		{
			name: "needs specifications - unavailable",
			ctx:  &HelpContext{HasActiveTask: true, HasSpecifications: false},
			commands: []*cobra.Command{
				{Use: "implement", Run: func(_ *cobra.Command, _ []string) {}},
				{Use: "review", Run: func(_ *cobra.Command, _ []string) {}},
			},
			wantLen: 2,
		},
		{
			name: "needs git - unavailable",
			ctx:  &HelpContext{HasActiveTask: true, UseGit: false},
			commands: []*cobra.Command{
				{Use: "undo", Run: func(_ *cobra.Command, _ []string) {}},
			},
			wantLen: 1,
		},
		{
			name: "mixed availability",
			ctx:  &HelpContext{HasActiveTask: false},
			commands: []*cobra.Command{
				{Use: "start", Run: func(_ *cobra.Command, _ []string) {}},     // always available
				{Use: "status", Run: func(_ *cobra.Command, _ []string) {}},    // needs active task - unavailable
				{Use: "implement", Run: func(_ *cobra.Command, _ []string) {}}, // needs specs - also unavailable
			},
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterUnavailable(tt.commands, tt.ctx)
			if len(result) != tt.wantLen {
				t.Errorf("FilterUnavailable() returned %d commands, want %d", len(result), tt.wantLen)
			}
		})
	}
}

func TestUnavailableReason(t *testing.T) {
	tests := []struct {
		cmd  string
		want string
	}{
		{"start", ""},
		{"auto", ""},
		{"status", "needs active task"},
		{"guide", "needs active task"},
		{"continue", "needs active task"},
		{"implement", "needs specifications"},
		{"review", "needs specifications"},
		{"finish", "needs specifications"},
		{"undo", "needs git task"},
		{"redo", "needs git task"},
		{"unknown-command", ""},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			got := UnavailableReason(tt.cmd)
			if got != tt.want {
				t.Errorf("UnavailableReason(%q) = %q, want %q", tt.cmd, got, tt.want)
			}
		})
	}
}

func TestRegisterTemplateFuncs(t *testing.T) {
	// This test ensures RegisterTemplateFuncs doesn't panic
	// We can't easily verify the functions are registered without
	// accessing Cobra's internal template func map
	RegisterTemplateFuncs()
}

func TestSetupContextualHelp(t *testing.T) {
	// Create a test command
	cmd := &cobra.Command{
		Use: "test",
	}

	// Setup contextual help
	SetupContextualHelp(cmd)

	// Verify the usage template was set
	if cmd.UsageTemplate() == "" {
		t.Error("UsageTemplate should be set after SetupContextualHelp")
	}

	// Verify the template is the contextual one
	if cmd.UsageTemplate() != ContextualUsageTemplate {
		t.Errorf("UsageTemplate = %q, want %q", cmd.UsageTemplate(), ContextualUsageTemplate)
	}
}

func TestFilterAvailable_SkipsHiddenCommands(t *testing.T) {
	ctx := &HelpContext{HasActiveTask: true}

	commands := []*cobra.Command{
		{Use: "status", Hidden: false, Run: func(_ *cobra.Command, _ []string) {}},
		{Use: "hidden-cmd", Hidden: true, Run: func(_ *cobra.Command, _ []string) {}},
	}

	result := FilterAvailable(commands, ctx)
	if len(result) != 1 {
		t.Errorf("FilterAvailable() should skip hidden commands, got %d, want 1", len(result))
	}
	if len(result) > 0 && result[0].Name() != "status" {
		t.Errorf("Expected 'status' command, got %q", result[0].Name())
	}
}
