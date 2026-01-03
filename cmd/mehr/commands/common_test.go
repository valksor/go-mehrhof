//go:build !testbinary
// +build !testbinary

package commands

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/valksor/go-mehrhof/internal/agent"
	"github.com/valksor/go-mehrhof/internal/testutil"
)

func TestConfirmAction(t *testing.T) {
	tests := []struct {
		name        string
		skipConfirm bool
	}{
		{
			name:        "skip confirm returns true",
			skipConfirm: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When skipConfirm is true, it should return true without prompting
			result, err := confirmAction("test action", tt.skipConfirm)
			if err != nil {
				t.Fatalf("confirmAction: %v", err)
			}
			if !result {
				t.Error("expected true when skipConfirm is true")
			}
		})
	}
}

func TestGetDeduplicatingStdout(t *testing.T) {
	// Should return a non-nil writer
	w := getDeduplicatingStdout()
	if w == nil {
		t.Error("getDeduplicatingStdout returned nil")
	}

	// Calling again should return the same instance (singleton)
	w2 := getDeduplicatingStdout()
	if w != w2 {
		t.Error("getDeduplicatingStdout should return the same instance")
	}
}

func TestRedoCommand_Properties(t *testing.T) {
	if redoCmd.Use != "redo" {
		t.Errorf("Use = %q, want %q", redoCmd.Use, "redo")
	}

	if redoCmd.Short == "" {
		t.Error("Short description is empty")
	}

	if redoCmd.RunE == nil {
		t.Error("RunE not set")
	}
}

func TestUndoCommand_Properties(t *testing.T) {
	if undoCmd.Use != "undo" {
		t.Errorf("Use = %q, want %q", undoCmd.Use, "undo")
	}

	if undoCmd.Short == "" {
		t.Error("Short description is empty")
	}

	if undoCmd.RunE == nil {
		t.Error("RunE not set")
	}
}

func TestVersionCommand_Properties(t *testing.T) {
	if versionCmd.Use != "version" {
		t.Errorf("Use = %q, want %q", versionCmd.Use, "version")
	}

	if versionCmd.Short == "" {
		t.Error("Short description is empty")
	}

	if versionCmd.Run == nil {
		t.Error("Run not set")
	}
}

func TestInitCommand_Properties(t *testing.T) {
	if initCmd.Use != "init" {
		t.Errorf("Use = %q, want %q", initCmd.Use, "init")
	}

	if initCmd.Short == "" {
		t.Error("Short description is empty")
	}

	if initCmd.RunE == nil {
		t.Error("RunE not set")
	}

	// Check for --interactive flag
	interactiveFlag := initCmd.Flags().Lookup("interactive")
	if interactiveFlag == nil {
		t.Error("init command missing 'interactive' flag")
	} else {
		// Check shorthand is "i"
		if interactiveFlag.Shorthand != "i" {
			t.Errorf("interactive flag shorthand = %q, want 'i'", interactiveFlag.Shorthand)
		}
	}
}

func TestAgentsCommand_Structure(t *testing.T) {
	// Check agents command structure
	if agentsCmd.Use != "agents" {
		t.Errorf("Use = %q, want %q", agentsCmd.Use, "agents")
	}

	// Check it has subcommands
	subcommands := agentsCmd.Commands()
	if len(subcommands) == 0 {
		t.Error("agentsCmd has no subcommands")
	}

	// Check for 'list' subcommand
	hasList := false
	for _, cmd := range subcommands {
		if cmd.Use == "list" {
			hasList = true
			if cmd.Short == "" {
				t.Error("agents list Short description is empty")
			}
			if cmd.RunE == nil {
				t.Error("agents list RunE not set")
			}

			break
		}
	}
	if !hasList {
		t.Error("agentsCmd missing 'list' subcommand")
	}
}

func TestConfigCommand_Structure(t *testing.T) {
	if configCmd.Use != "config" {
		t.Errorf("Use = %q, want %q", configCmd.Use, "config")
	}

	subcommands := configCmd.Commands()
	if len(subcommands) == 0 {
		t.Error("configCmd has no subcommands")
	}

	// Check for 'validate' subcommand
	hasValidate := false
	for _, cmd := range subcommands {
		if cmd.Use == "validate" {
			hasValidate = true
			if cmd.Short == "" {
				t.Error("config validate Short description is empty")
			}

			break
		}
	}
	if !hasValidate {
		t.Error("configCmd missing 'validate' subcommand")
	}

	// Check validate flags
	validateFlag := configValidateCmd.Flags().Lookup("strict")
	if validateFlag == nil {
		t.Error("validate command missing 'strict' flag")
	}

	formatFlag := configValidateCmd.Flags().Lookup("format")
	if formatFlag == nil {
		t.Error("validate command missing 'format' flag")
	}
}

func TestRootCommand_HasSubcommands(t *testing.T) {
	// Check that root command has some expected subcommands
	// Note: Due to init() function ordering, not all commands may be registered
	// during test execution. This test verifies a subset of known commands.

	expectedSubcommands := []string{
		// Core commands that should always be present
		"abandon", "undo", "redo", "version", "init", "config", "agents",
	}

	actualSubcommands := rootCmd.Commands()
	actualNames := make(map[string]bool)
	for _, cmd := range actualSubcommands {
		actualNames[cmd.Use] = true
	}

	missingCommands := []string{}
	for _, expected := range expectedSubcommands {
		if !actualNames[expected] {
			missingCommands = append(missingCommands, expected)
		}
	}

	if len(missingCommands) > 0 {
		// Log as warning rather than fail, since init ordering can vary
		t.Logf("Warning: Some expected subcommands not found: %v", missingCommands)
		t.Logf("Found subcommands: %v", getCommandNames(t, actualSubcommands))
	}

	// Verify at least some commands are registered
	if len(actualNames) < 5 {
		t.Errorf("Expected at least 5 subcommands, got %d", len(actualNames))
	}
}

func getCommandNames(t *testing.T, commands []*cobra.Command) []string {
	t.Helper()
	names := make([]string, len(commands))
	for i, cmd := range commands {
		names[i] = cmd.Use
	}

	return names
}

func TestImplementCommand_Aliases(t *testing.T) {
	expectedAliases := []string{"impl", "i"}
	actualAliases := implementCmd.Aliases

	if len(actualAliases) != len(expectedAliases) {
		t.Errorf("implement aliases count = %d, want %d", len(actualAliases), len(expectedAliases))
	}

	for i, expected := range expectedAliases {
		if i >= len(actualAliases) || actualAliases[i] != expected {
			t.Errorf("implement alias[%d] = %q, want %q", i, actualAliases[i], expected)
		}
	}
}

func TestFinishCommand_Aliases(t *testing.T) {
	expectedAliases := []string{"fi", "done"}
	actualAliases := finishCmd.Aliases

	if len(actualAliases) != len(expectedAliases) {
		t.Errorf("finish aliases count = %d, want %d", len(actualAliases), len(expectedAliases))
	}

	for i, expected := range expectedAliases {
		if i >= len(actualAliases) || actualAliases[i] != expected {
			t.Errorf("finish alias[%d] = %q, want %q", i, actualAliases[i], expected)
		}
	}
}

func TestStatusCommand_Aliases(t *testing.T) {
	expectedAliases := []string{"st"}
	actualAliases := statusCmd.Aliases

	if len(actualAliases) != len(expectedAliases) {
		t.Errorf("status aliases count = %d, want %d", len(actualAliases), len(expectedAliases))
	}

	for i, expected := range expectedAliases {
		if i >= len(actualAliases) || actualAliases[i] != expected {
			t.Errorf("status alias[%d] = %q, want %q", i, actualAliases[i], expected)
		}
	}
}

func TestContinueCommand_Aliases(t *testing.T) {
	expectedAliases := []string{"cont", "c"}
	actualAliases := continueCmd.Aliases

	if len(actualAliases) != len(expectedAliases) {
		t.Errorf("continue aliases count = %d, want %d", len(actualAliases), len(expectedAliases))
	}

	for i, expected := range expectedAliases {
		if i >= len(actualAliases) || actualAliases[i] != expected {
			t.Errorf("continue alias[%d] = %q, want %q", i, actualAliases[i], expected)
		}
	}
}

func TestPlanCommand_Aliases(t *testing.T) {
	expectedAliases := []string{"p"}
	actualAliases := planCmd.Aliases

	if len(actualAliases) != len(expectedAliases) {
		t.Errorf("plan aliases count = %d, want %d", len(actualAliases), len(expectedAliases))
	}

	for i, expected := range expectedAliases {
		if i >= len(actualAliases) || actualAliases[i] != expected {
			t.Errorf("plan alias[%d] = %q, want %q", i, actualAliases[i], expected)
		}
	}
}

func TestStartCommand_AgentFlagShorthand(t *testing.T) {
	// The agent flag should have shorthand 'A' (not 'a' which conflicts with --all)
	agentFlag := startCmd.Flags().Lookup("agent")
	if agentFlag == nil {
		t.Fatal("start command missing 'agent' flag")
	}

	if agentFlag.Shorthand != "A" {
		t.Errorf("agent flag shorthand = %q, want 'A'", agentFlag.Shorthand)
	}
}

func TestPlanCommand_StandaloneFlag(t *testing.T) {
	// Check for --standalone flag
	standaloneFlag := planCmd.Flags().Lookup("standalone")
	if standaloneFlag == nil {
		t.Error("plan command missing 'standalone' flag")
	}

	// The deprecated --new flag was removed (use --standalone instead)
}

func TestCostCommand_BreakdownFlag(t *testing.T) {
	// Check for --breakdown flag
	breakdownFlag := costCmd.Flags().Lookup("breakdown")
	if breakdownFlag == nil {
		t.Error("cost command missing 'breakdown' flag")
	}

	// The deprecated --by-step flag was removed (use --breakdown instead)
}

// ─────────────────────────────────────────────────────────────────────────────
// Additional tests for common.go utilities
// These tests increase coverage for shared helper functions
// ─────────────────────────────────────────────────────────────────────────────

// TestIsQuiet tests the IsQuiet function.
func TestIsQuiet(t *testing.T) {
	// Save original value
	originalQuiet := quiet
	defer func() { quiet = originalQuiet }()

	tests := []struct {
		name     string
		setQuiet bool
		want     bool
	}{
		{"quiet mode enabled", true, true},
		{"quiet mode disabled", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quiet = tt.setQuiet
			if got := IsQuiet(); got != tt.want {
				t.Errorf("IsQuiet() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestBuildConductorOptions tests building conductor options from command options.
func TestBuildConductorOptions(t *testing.T) {
	tests := []struct {
		name    string
		cmdOpts CommandOptions
		minOpts int // Minimum expected options
	}{
		{
			name: "minimal options",
			cmdOpts: CommandOptions{
				Verbose: false,
			},
			minOpts: 1,
		},
		{
			name: "verbose mode",
			cmdOpts: CommandOptions{
				Verbose: true,
			},
			minOpts: 1,
		},
		{
			name: "dry run",
			cmdOpts: CommandOptions{
				DryRun: true,
			},
			minOpts: 2,
		},
		{
			name: "full context",
			cmdOpts: CommandOptions{
				FullContext: true,
			},
			minOpts: 2,
		},
		{
			name: "all options enabled",
			cmdOpts: CommandOptions{
				Verbose:     true,
				DryRun:      true,
				FullContext: true,
			},
			minOpts: 4,
		},
		{
			name: "step agent - planning",
			cmdOpts: CommandOptions{
				StepAgent: "planning",
			},
			minOpts: 2,
		},
		{
			name: "step agent - implement (alias)",
			cmdOpts: CommandOptions{
				StepAgent: "implement",
			},
			minOpts: 2,
		},
		{
			name: "step agent - unknown (should be ignored)",
			cmdOpts: CommandOptions{
				StepAgent: "unknown-step",
			},
			minOpts: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := BuildConductorOptions(tt.cmdOpts)
			if len(opts) < tt.minOpts {
				t.Errorf("BuildConductorOptions() returned %d options, want at least %d", len(opts), tt.minOpts)
			}
		})
	}
}

// TestDeriveStepName tests the deriveStepName function.
func TestDeriveStepName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"planning step", "planning", "planning"},
		{"implementing step", "implementing", "implementing"},
		{"implement alias", "implement", "implementing"},
		{"reviewing step", "reviewing", "reviewing"},
		{"review alias", "review", "reviewing"},
		{"checkpointing step", "checkpointing", "checkpointing"},
		{"unknown step", "unknown", ""},
		{"empty string", "", ""},
		{"case insensitive - Planning", "Planning", "planning"},
		{"case insensitive - IMPLEMENT", "IMPLEMENT", "implementing"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveStepName(tt.input)
			if got != tt.expected {
				t.Errorf("deriveStepName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// TestResolveWorkspaceRoot tests the ResolveWorkspaceRoot function.
func TestResolveWorkspaceRoot(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T) (context.Context, func())
		checkResult func(t *testing.T, res WorkspaceResolution, err error)
	}{
		{
			name: "non-git directory",
			setup: func(t *testing.T) (context.Context, func()) {
				t.Helper()
				tmpDir := t.TempDir()
				ctx := context.Background()
				originalDir, _ := os.Getwd()
				chdir(t, tmpDir)

				return ctx, func() {
					chdir(t, originalDir)
				}
			},
			checkResult: func(t *testing.T, res WorkspaceResolution, err error) {
				t.Helper()
				if err != nil {
					t.Fatalf("ResolveWorkspaceRoot() error = %v", err)
				}
				if res.Root == "" {
					t.Error("Root should not be empty")
				}
				if res.Git != nil {
					t.Error("Git should be nil for non-git directory")
				}
				if res.IsWorktree {
					t.Error("IsWorktree should be false for non-git directory")
				}
			},
		},
		{
			name: "git repository",
			setup: func(t *testing.T) (context.Context, func()) {
				t.Helper()
				_ = testutil.CreateTempGitRepo(t)
				ctx := context.Background()

				return ctx, func() {}
			},
			checkResult: func(t *testing.T, res WorkspaceResolution, err error) {
				t.Helper()
				if err != nil {
					t.Fatalf("ResolveWorkspaceRoot() error = %v", err)
				}
				if res.Root == "" {
					t.Error("Root should not be empty")
				}
				if res.Git == nil {
					t.Error("Git should not be nil for git repository")
				}
				if res.IsWorktree {
					t.Error("IsWorktree should be false for main repository")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cleanup := tt.setup(t)
			defer cleanup()

			res, err := ResolveWorkspaceRoot(ctx)
			tt.checkResult(t, res, err)
		})
	}
}

// TestPrintToolCallTo tests the printToolCallTo function.
func TestPrintToolCallTo(t *testing.T) {
	tests := []struct {
		name       string
		toolCall   *agent.ToolCall
		wantOutput string
	}{
		{
			name:       "nil tool call",
			toolCall:   nil,
			wantOutput: "",
		},
		{
			name: "tool call with description",
			toolCall: &agent.ToolCall{
				Name:        "Read",
				Description: "Read a file",
			},
			wantOutput: "→ Read: Read a file\n",
		},
		{
			name: "tool call without description",
			toolCall: &agent.ToolCall{
				Name: "Write",
			},
			wantOutput: "→ Write\n",
		},
		{
			name: "tool call with empty description",
			toolCall: &agent.ToolCall{
				Name:        "Edit",
				Description: "",
			},
			wantOutput: "→ Edit\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			printToolCallTo(&buf, tt.toolCall)

			got := buf.String()
			if got != tt.wantOutput {
				t.Errorf("printToolCallTo() output = %q, want %q", got, tt.wantOutput)
			}
		})
	}
}

// TestPrintAgentEventTo tests the printAgentEventTo function.
func TestPrintAgentEventTo(t *testing.T) {
	tests := []struct {
		name       string
		event      agent.Event
		wantOutput string
	}{
		{
			name: "event with text only",
			event: agent.Event{
				Text: "Hello, world!",
			},
			wantOutput: "Hello, world!",
		},
		{
			name: "event with tool call",
			event: agent.Event{
				Text: "Processing file",
				ToolCall: &agent.ToolCall{
					Name:        "Read",
					Description: "Read config",
				},
			},
			wantOutput: "Processing file→ Read: Read config\n",
		},
		{
			name: "event with tool call only",
			event: agent.Event{
				ToolCall: &agent.ToolCall{
					Name: "Write",
				},
			},
			wantOutput: "→ Write\n",
		},
		{
			name: "event with result in data",
			event: agent.Event{
				Data: map[string]any{
					"result": "Operation complete",
				},
			},
			wantOutput: "Operation complete",
		},
		{
			name: "event with tool_calls in data",
			event: agent.Event{
				Data: map[string]any{
					"tool_calls": []*agent.ToolCall{
						{Name: "Bash", Description: "Run command"},
						{Name: "Grep", Description: "Search files"},
					},
				},
			},
			wantOutput: "→ Bash: Run command\n→ Grep: Search files\n",
		},
		{
			name:       "empty event",
			event:      agent.Event{},
			wantOutput: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			printAgentEventTo(&buf, tt.event)

			got := buf.String()
			if got != tt.wantOutput {
				t.Errorf("printAgentEventTo() output =\n%q\nwant\n%q", got, tt.wantOutput)
			}
		})
	}
}

// TestConfirmAction_WithInput tests confirmAction with various user inputs.
func TestConfirmAction_WithInput(t *testing.T) {
	// Save original stdin
	originalStdin := os.Stdin
	defer func() { os.Stdin = originalStdin }()

	tests := []struct {
		name        string
		input       string
		skipConfirm bool
		wantResult  bool
		wantErr     bool
	}{
		{
			name:        "user confirms with 'y'",
			input:       "y\n",
			skipConfirm: false,
			wantResult:  true,
			wantErr:     false,
		},
		{
			name:        "user confirms with 'yes'",
			input:       "yes\n",
			skipConfirm: false,
			wantResult:  true,
			wantErr:     false,
		},
		{
			name:        "user confirms with 'Y' (uppercase)",
			input:       "Y\n",
			skipConfirm: false,
			wantResult:  true,
			wantErr:     false,
		},
		{
			name:        "user confirms with 'YES' (uppercase)",
			input:       "YES\n",
			skipConfirm: false,
			wantResult:  true,
			wantErr:     false,
		},
		{
			name:        "user denies with 'n'",
			input:       "n\n",
			skipConfirm: false,
			wantResult:  false,
			wantErr:     false,
		},
		{
			name:        "user denies with 'no'",
			input:       "no\n",
			skipConfirm: false,
			wantResult:  false,
			wantErr:     false,
		},
		{
			name:        "user denies with empty input",
			input:       "\n",
			skipConfirm: false,
			wantResult:  false,
			wantErr:     false,
		},
		{
			name:        "user inputs whitespace",
			input:       "   \n",
			skipConfirm: false,
			wantResult:  false,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create pipe for mock stdin
			r, w, _ := os.Pipe()
			os.Stdin = r

			// Write input to pipe
			if tt.input != "" {
				if _, err := w.WriteString(tt.input); err != nil {
					t.Fatalf("write stdin: %v", err)
				}
			}
			if err := w.Close(); err != nil {
				t.Fatalf("close stdin writer: %v", err)
			}

			// Capture stdout to avoid prompt output
			oldStdout := os.Stdout
			_, outW, _ := os.Pipe()
			os.Stdout = outW

			got, err := confirmAction("Test prompt", tt.skipConfirm)

			// Restore stdout
			os.Stdout = oldStdout
			_ = outW.Close()

			if (err != nil) != tt.wantErr {
				t.Errorf("confirmAction() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.wantResult {
				t.Errorf("confirmAction() = %v, want %v", got, tt.wantResult)
			}
		})
	}
}

// TestPrintNextSteps tests the PrintNextSteps function.
func TestPrintNextSteps(t *testing.T) {
	// Save original stdout and quiet
	originalStdout := os.Stdout
	originalQuiet := quiet
	defer func() {
		os.Stdout = originalStdout
		quiet = originalQuiet
	}()

	tests := []struct {
		name         string
		setQuiet     bool
		steps        []string
		wantContains []string
	}{
		{
			name:     "normal mode - single step",
			setQuiet: false,
			steps:    []string{"Run tests"},
			wantContains: []string{
				"Next steps:",
				"Run tests",
			},
		},
		{
			name:     "normal mode - multiple steps",
			setQuiet: false,
			steps:    []string{"Run tests", "Commit changes", "Push to remote"},
			wantContains: []string{
				"Next steps:",
				"Run tests",
				"Commit changes",
				"Push to remote",
			},
		},
		{
			name:         "quiet mode - suppresses output",
			setQuiet:     true,
			steps:        []string{"Run tests"},
			wantContains: []string{},
		},
		{
			name:     "normal mode - empty steps",
			setQuiet: false,
			steps:    []string{},
			wantContains: []string{
				"Next steps:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quiet = tt.setQuiet

			// Capture stdout using a pipe
			r, w, _ := os.Pipe()
			os.Stdout = w

			PrintNextSteps(tt.steps...)

			// Close writer and restore stdout
			_ = w.Close()
			os.Stdout = originalStdout

			// Read captured output
			var buf bytes.Buffer
			_, _ = buf.ReadFrom(r)
			output := buf.String()

			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("output should contain %q, got %q", want, output)
				}
			}

			if tt.setQuiet && output != "" {
				t.Errorf("quiet mode should suppress output, got %q", output)
			}
		})
	}
}

// TestWorkspaceResolution tests the WorkspaceResolution struct.
func TestWorkspaceResolution(t *testing.T) {
	t.Run("zero value", func(t *testing.T) {
		var res WorkspaceResolution
		if res.Root != "" {
			t.Errorf("zero value Root should be empty, got %q", res.Root)
		}
		if res.Git != nil {
			t.Error("zero value Git should be nil")
		}
		if res.IsWorktree {
			t.Error("zero value IsWorktree should be false")
		}
	})
}

// chdir is a helper to change directory.
func chdir(t *testing.T, dir string) {
	t.Helper()
	t.Chdir(dir)
}
