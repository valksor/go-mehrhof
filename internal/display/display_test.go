package display

import (
	"fmt"
	"testing"
	"time"

	"github.com/valksor/go-mehrhof/internal/storage"
	"github.com/valksor/go-mehrhof/internal/workflow"
)

func TestFormatState(t *testing.T) {
	tests := []struct {
		name  string
		state workflow.State
		want  string
	}{
		{"idle", workflow.StateIdle, "Ready"},
		{"planning", workflow.StatePlanning, "Planning"},
		{"implementing", workflow.StateImplementing, "Implementing"},
		{"reviewing", workflow.StateReviewing, "Reviewing"},
		{"done", workflow.StateDone, "Completed"},
		{"failed", workflow.StateFailed, "Failed"},
		{"waiting", workflow.StateWaiting, "Waiting"},
		{"checkpointing", workflow.StateCheckpointing, "Checkpointing"},
		{"reverting", workflow.StateReverting, "Reverting"},
		{"restoring", workflow.StateRestoring, "Restoring"},
		{"unknown state", workflow.State("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatState(tt.state)
			if got != tt.want {
				t.Errorf("FormatState(%q) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestFormatStateString(t *testing.T) {
	tests := []struct {
		name  string
		state string
		want  string
	}{
		{"idle", "idle", "Ready"},
		{"planning", "planning", "Planning"},
		{"implementing", "implementing", "Implementing"},
		{"unknown", "unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatStateString(tt.state)
			if got != tt.want {
				t.Errorf("FormatStateString(%q) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestGetStateDescription(t *testing.T) {
	tests := []struct {
		name  string
		state workflow.State
		want  string
	}{
		{"idle", workflow.StateIdle, "Ready to start"},
		{"planning", workflow.StatePlanning, "AI is creating specifications"},
		{"implementing", workflow.StateImplementing, "AI is generating code"},
		{"reviewing", workflow.StateReviewing, "Code review in progress"},
		{"done", workflow.StateDone, "Task completed successfully"},
		{"failed", workflow.StateFailed, "Task failed with error"},
		{"waiting", workflow.StateWaiting, "Action required: Awaiting your response"},
		{"checkpointing", workflow.StateCheckpointing, "Creating checkpoint"},
		{"reverting", workflow.StateReverting, "Reverting to previous state"},
		{"restoring", workflow.StateRestoring, "Restoring from checkpoint"},
		{"unknown", workflow.State("unknown"), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetStateDescription(tt.state)
			if got != tt.want {
				t.Errorf("GetStateDescription(%q) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestFormatSpecificationStatus(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   string
	}{
		{"draft", storage.SpecificationStatusDraft, "Draft"},
		{"ready", storage.SpecificationStatusReady, "Ready"},
		{"implementing", storage.SpecificationStatusImplementing, "Implementing"},
		{"done", storage.SpecificationStatusDone, "Completed"},
		{"unknown", "unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSpecificationStatus(tt.status)
			if got != tt.want {
				t.Errorf("FormatSpecificationStatus(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestGetSpecificationStatusIcon(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   string
	}{
		{"draft", storage.SpecificationStatusDraft, "○"},
		{"ready", storage.SpecificationStatusReady, "◐"},
		{"implementing", storage.SpecificationStatusImplementing, "◑"},
		{"done", storage.SpecificationStatusDone, "●"},
		{"unknown", "unknown", "?"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetSpecificationStatusIcon(tt.status)
			if got != tt.want {
				t.Errorf("GetSpecificationStatusIcon(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestFormatSpecificationStatusWithIcon(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   string
	}{
		{"draft", storage.SpecificationStatusDraft, "○ Draft"},
		{"ready", storage.SpecificationStatusReady, "◐ Ready"},
		{"implementing", storage.SpecificationStatusImplementing, "◑ Implementing"},
		{"done", storage.SpecificationStatusDone, "● Completed"},
		{"unknown", "unknown", "? unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSpecificationStatusWithIcon(tt.status)
			if got != tt.want {
				t.Errorf("FormatSpecificationStatusWithIcon(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestStateDisplayMapCompleteness(t *testing.T) {
	// Verify all known workflow states have display mappings
	knownStates := []workflow.State{
		workflow.StateIdle,
		workflow.StatePlanning,
		workflow.StateImplementing,
		workflow.StateReviewing,
		workflow.StateDone,
		workflow.StateFailed,
		workflow.StateWaiting,
		workflow.StateCheckpointing,
		workflow.StateReverting,
		workflow.StateRestoring,
	}

	for _, state := range knownStates {
		if _, ok := StateDisplay[state]; !ok {
			t.Errorf("StateDisplay missing mapping for %q", state)
		}
		if _, ok := StateDescription[state]; !ok {
			t.Errorf("StateDescription missing mapping for %q", state)
		}
	}
}

func TestSpecificationStatusMapsCompleteness(t *testing.T) {
	// Verify all known specification statuses have display mappings
	knownStatuses := []string{
		storage.SpecificationStatusDraft,
		storage.SpecificationStatusReady,
		storage.SpecificationStatusImplementing,
		storage.SpecificationStatusDone,
	}

	for _, status := range knownStatuses {
		if _, ok := SpecificationStatusDisplay[status]; !ok {
			t.Errorf("SpecificationStatusDisplay missing mapping for %q", status)
		}
		if _, ok := SpecificationStatusIcon[status]; !ok {
			t.Errorf("SpecificationStatusIcon missing mapping for %q", status)
		}
	}
}

// TestColoredFormatFunctions tests the new color-aware formatting functions.
func TestFormatStateColored(t *testing.T) {
	// Disable colors for predictable test output
	SetColorsEnabled(false)
	defer SetColorsEnabled(true)

	// Format is "[prefix] StateName" for accessibility
	tests := []struct {
		name  string
		state workflow.State
		want  string
	}{
		{"idle", workflow.StateIdle, "[R] Ready"},
		{"planning", workflow.StatePlanning, "[P] Planning"},
		{"done", workflow.StateDone, "[D] Completed"},
		{"failed", workflow.StateFailed, "[F] Failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatStateColored(tt.state)
			if got != tt.want {
				t.Errorf("FormatStateColored(%q) = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

// TestInitColors tests the color initialization function.
func TestInitColors(t *testing.T) {
	// Save original state
	wasEnabled := ColorsEnabled()

	tests := []struct {
		name     string
		noColor  bool
		envColor string
		want     bool
	}{
		{"no color flag", true, "", false},
		{"NO_COLOR env", false, "1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			if tt.envColor != "" {
				t.Setenv("NO_COLOR", tt.envColor)
			} else {
				t.Setenv("NO_COLOR", "")
			}

			// Re-initialize with test settings
			InitColors(tt.noColor)

			got := ColorsEnabled()
			if got != tt.want {
				t.Errorf("ColorsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}

	// Restore original state
	SetColorsEnabled(wasEnabled)
}

// TestSetColorsEnabled tests manual color control.
func TestSetColorsEnabled(t *testing.T) {
	// Save original state
	original := ColorsEnabled()
	defer SetColorsEnabled(original)

	tests := []struct {
		name    string
		enabled bool
	}{
		{"disable", false},
		{"enable", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetColorsEnabled(tt.enabled)
			if got := ColorsEnabled(); got != tt.enabled {
				t.Errorf("ColorsEnabled() = %v, want %v", got, tt.enabled)
			}
		})
	}
}

// TestColorFunctions tests the color formatting functions.
func TestColorFunctions(t *testing.T) {
	// Disable colors for predictable output
	SetColorsEnabled(false)
	defer SetColorsEnabled(true)

	tests := []struct {
		name string
		fn   func(string) string
		text string
	}{
		{"Success", Success, "test"},
		{"Error", Error, "test"},
		{"Warning", Warning, "test"},
		{"Info", Info, "test"},
		{"Muted", Muted, "test"},
		{"Bold", Bold, "test"},
		{"Cyan", Cyan, "test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn(tt.text)
			if got != tt.text {
				t.Errorf("%s(%q) = %q, want %q", tt.name, tt.text, got, tt.text)
			}
		})
	}
}

// TestColorPrefixes tests the prefix functions.
func TestColorPrefixes(t *testing.T) {
	// Disable colors for predictable output
	SetColorsEnabled(false)
	defer SetColorsEnabled(true)

	tests := []struct {
		name string
		fn   func() string
		want string
	}{
		{"SuccessPrefix", SuccessPrefix, "✓"},
		{"ErrorPrefix", ErrorPrefix, "✗"},
		{"WarningPrefix", WarningPrefix, "⚠"},
		{"InfoPrefix", InfoPrefix, "→"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn()
			if got != tt.want {
				t.Errorf("%s() = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

// TestMessageFormatters tests the formatted message functions.
func TestMessageFormatters(t *testing.T) {
	// Disable colors for predictable output
	SetColorsEnabled(false)
	defer SetColorsEnabled(true)

	tests := []struct {
		name     string
		fn       func(string, ...any) string
		format   string
		args     []any
		contains string
	}{
		{"SuccessMsg", SuccessMsg, "done", nil, "✓ done"},
		{"ErrorMsg", ErrorMsg, "failed", nil, "✗ failed"},
		{"WarningMsg", WarningMsg, "careful", nil, "⚠ careful"},
		{"InfoMsg", InfoMsg, "info", nil, "→ info"},
		{"SuccessMsg with args", SuccessMsg, "%s %d", []any{"test", 42}, "✓ test 42"},
		{"ErrorMsg with args", ErrorMsg, "code %d", []any{404}, "✗ code 404"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn(tt.format, tt.args...)
			if got != tt.contains {
				t.Errorf("%s() = %q, want %q", tt.name, got, tt.contains)
			}
		})
	}
}

// TestColorState tests state-based color formatting.
func TestColorState(t *testing.T) {
	// Disable colors for predictable output
	SetColorsEnabled(false)
	defer SetColorsEnabled(true)

	tests := []struct {
		name        string
		state       string
		displayName string
	}{
		{"idle", "idle", "Ready"},
		{"planning", "planning", "Planning"},
		{"implementing", "implementing", "Implementing"},
		{"reviewing", "reviewing", "Reviewing"},
		{"checkpointing", "checkpointing", "Checkpointing"},
		{"done", "done", "Done"},
		{"failed", "failed", "Failed"},
		{"waiting", "waiting", "Waiting"},
		{"unknown", "unknown", "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ColorState(tt.state, tt.displayName)
			if got != tt.displayName {
				t.Errorf("ColorState(%q, %q) = %q, want %q", tt.state, tt.displayName, got, tt.displayName)
			}
		})
	}
}

// TestColorSpecStatus tests specification status color formatting.
func TestColorSpecStatus(t *testing.T) {
	// Disable colors for predictable output
	SetColorsEnabled(false)
	defer SetColorsEnabled(true)

	tests := []struct {
		name        string
		status      string
		displayName string
	}{
		{"draft", "draft", "Draft"},
		{"ready", "ready", "Ready"},
		{"implementing", "implementing", "Implementing"},
		{"done", "done", "Done"},
		{"unknown", "unknown", "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ColorSpecStatus(tt.status, tt.displayName)
			if got != tt.displayName {
				t.Errorf("ColorSpecStatus(%q, %q) = %q, want %q", tt.status, tt.displayName, got, tt.displayName)
			}
		})
	}
}

// TestWorktreeIndicator tests the worktree indicator function.
func TestWorktreeIndicator(t *testing.T) {
	// Disable colors for predictable output
	SetColorsEnabled(false)
	defer SetColorsEnabled(true)

	tests := []struct {
		name       string
		isWorktree bool
		wantEmpty  bool
	}{
		{"worktree", true, false},
		{"main repo", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WorktreeIndicator(tt.isWorktree)
			isEmpty := got == ""
			if isEmpty != tt.wantEmpty {
				t.Errorf("WorktreeIndicator(%v) = %q, isEmpty=%v, want isEmpty=%v", tt.isWorktree, got, isEmpty, tt.wantEmpty)
			}
		})
	}
}

// TestColorsWithEnabled tests that colors are actually applied when enabled.
func TestColorsWithEnabled(t *testing.T) {
	// Save original state
	original := ColorsEnabled()
	defer SetColorsEnabled(original)

	// Enable colors
	SetColorsEnabled(true)

	// Test that colored output contains ANSI codes
	text := "test"
	got := Success(text)
	if got == text {
		t.Error("Success() should return different text when colors enabled")
	}
	// Should contain reset code
	if got == "" || !contains(got, "\033") {
		t.Error("Success() should contain ANSI codes when enabled")
	}
}

// TestErrorWithSuggestions tests error formatting with suggestions.
func TestErrorWithSuggestions(t *testing.T) {
	// Disable colors for predictable output
	SetColorsEnabled(false)
	defer SetColorsEnabled(true)

	tests := []struct {
		name        string
		message     string
		suggestions []Suggestion
		contains    []string
	}{
		{
			name:        "no suggestions",
			message:     "something failed",
			suggestions: []Suggestion{},
			contains:    []string{"✗", "something failed"},
		},
		{
			name:    "with suggestions",
			message: "task not found",
			suggestions: []Suggestion{
				{Command: "mehr list", Description: "View all tasks"},
				{Command: "mehr start", Description: "Start a new task"},
			},
			contains: []string{"✗", "task not found", "Suggested actions", "mehr list", "View all tasks", "mehr start", "Start a new task"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ErrorWithSuggestions(tt.message, tt.suggestions)
			for _, want := range tt.contains {
				if !contains(got, want) {
					t.Errorf("ErrorWithSuggestions() missing %q\nGot: %s", want, got)
				}
			}
		})
	}
}

// TestErrorWithContext tests error formatting with context.
func TestErrorWithContext(t *testing.T) {
	// Disable colors for predictable output
	SetColorsEnabled(false)
	defer SetColorsEnabled(true)

	tests := []struct {
		name        string
		err         error
		context     string
		suggestions []Suggestion
		contains    []string
	}{
		{
			name:        "no error, no suggestions",
			err:         nil,
			context:     "operation failed",
			suggestions: []Suggestion{},
			contains:    []string{"✗", "Error: operation failed"},
		},
		{
			name:    "with error",
			err:     fmt.Errorf("underlying issue"),
			context: "operation failed",
			suggestions: []Suggestion{
				{Command: "mehr status", Description: "Check status"},
			},
			contains: []string{"✗", "Error: operation failed", "Cause: underlying issue", "Suggested actions", "mehr status", "Check status"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ErrorWithContext(tt.err, tt.context, tt.suggestions)
			for _, want := range tt.contains {
				if !contains(got, want) {
					t.Errorf("ErrorWithContext() missing %q\nGot: %s", want, got)
				}
			}
		})
	}
}

// TestValidationError tests validation error formatting.
func TestValidationError(t *testing.T) {
	// Disable colors for predictable output
	SetColorsEnabled(false)
	defer SetColorsEnabled(true)

	tests := []struct {
		name        string
		field       string
		message     string
		suggestions []Suggestion
		contains    []string
	}{
		{
			name:        "no suggestions",
			field:       "config.yaml",
			message:     "invalid format",
			suggestions: []Suggestion{},
			contains:    []string{"✗", "Validation Error: config.yaml", "invalid format"},
		},
		{
			name:    "with suggestions",
			field:   "agent",
			message: "unknown agent",
			suggestions: []Suggestion{
				{Command: "mehr agents list", Description: "List agents"},
			},
			contains: []string{"✗", "Validation Error: agent", "unknown agent", "Fix:", "List agents"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidationError(tt.field, tt.message, tt.suggestions)
			for _, want := range tt.contains {
				if !contains(got, want) {
					t.Errorf("ValidationError() missing %q\nGot: %s", want, got)
				}
			}
		})
	}
}

// TestProviderError tests provider error formatting.
func TestProviderError(t *testing.T) {
	// Disable colors for predictable output
	SetColorsEnabled(false)
	defer SetColorsEnabled(true)

	got := ProviderError("jira", fmt.Errorf("API error"), []Suggestion{
		{Command: "mehr config validate", Description: "Validate config"},
	})

	wantContains := []string{"✗", "Error:", "Failed to load from jira provider", "Cause: API error", "Suggested actions", "mehr config validate", "Validate config"}
	for _, want := range wantContains {
		if !contains(got, want) {
			t.Errorf("ProviderError() missing %q\nGot: %s", want, got)
		}
	}
}

// TestNoActiveTaskError tests the no active task error.
func TestNoActiveTaskError(t *testing.T) {
	// Disable colors for predictable output
	SetColorsEnabled(false)
	defer SetColorsEnabled(true)

	got := NoActiveTaskError()

	wantContains := []string{"✗", "No active task", "Suggested actions", "mehr start", "mehr list"}
	for _, want := range wantContains {
		if !contains(got, want) {
			t.Errorf("NoActiveTaskError() missing %q\nGot: %s", want, got)
		}
	}
}

// TestTaskFailedError tests task failure error formatting.
func TestTaskFailedError(t *testing.T) {
	// Disable colors for predictable output
	SetColorsEnabled(false)
	defer SetColorsEnabled(true)

	got := TaskFailedError("implementation", fmt.Errorf("syntax error"))

	wantContains := []string{"✗", "Error:", "Task failed during implementation", "Cause: syntax error", "Suggested actions", "mehr status", "mehr note", "mehr undo"}
	for _, want := range wantContains {
		if !contains(got, want) {
			t.Errorf("TaskFailedError() missing %q\nGot: %s", want, got)
		}
	}
}

// TestConfigError tests configuration error formatting.
func TestConfigError(t *testing.T) {
	// Disable colors for predictable output
	SetColorsEnabled(false)
	defer SetColorsEnabled(true)

	got := ConfigError(fmt.Errorf("invalid YAML"), ".mehrhof/config.yaml")

	wantContains := []string{"✗", "Error:", "Configuration error in .mehrhof/config.yaml", "Cause: invalid YAML", "Suggested actions", "mehr config validate", "cat .mehrhof/config.yaml"}
	for _, want := range wantContains {
		if !contains(got, want) {
			t.Errorf("ConfigError() missing %q\nGot: %s", want, got)
		}
	}
}

// TestAgentError tests agent error formatting.
func TestAgentError(t *testing.T) {
	// Disable colors for predictable output
	SetColorsEnabled(false)
	defer SetColorsEnabled(true)

	got := AgentError("opus", fmt.Errorf("model not available"))

	wantContains := []string{"✗", "Error:", "Agent error: opus", "Cause: model not available", "Suggested actions", "mehr agents list", "mehr --agent="}
	for _, want := range wantContains {
		if !contains(got, want) {
			t.Errorf("AgentError() missing %q\nGot: %s", want, got)
		}
	}
}

// TestGitError tests git error formatting.
func TestGitError(t *testing.T) {
	// Disable colors for predictable output
	SetColorsEnabled(false)
	defer SetColorsEnabled(true)

	got := GitError("branch creation", fmt.Errorf("detached HEAD"))

	wantContains := []string{"✗", "Error:", "Git branch creation failed", "Cause: detached HEAD", "Suggested actions", "git status", "mehr start --no-branch"}
	for _, want := range wantContains {
		if !contains(got, want) {
			t.Errorf("GitError() missing %q\nGot: %s", want, got)
		}
	}
}

// Helper function to check if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findInString(s, substr)))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestFormatter tests the Formatter type methods.
func TestFormatter(t *testing.T) {
	// Disable colors for predictable output
	SetColorsEnabled(false)
	defer SetColorsEnabled(true)

	t.Run("NewFormatter creates default formatter", func(t *testing.T) {
		f := NewFormatter()
		if f == nil {
			t.Fatal("NewFormatter() returned nil")
		}
		if f.indentLevel != 0 {
			t.Errorf("NewFormatter().indentLevel = %d, want 0", f.indentLevel)
		}
		if f.width != DefaultTableWidth {
			t.Errorf("NewFormatter().width = %d, want %d", f.width, DefaultTableWidth)
		}
	})

	t.Run("SetIndent modifies indent level", func(t *testing.T) {
		f := NewFormatter()
		result := f.SetIndent(2)
		if result != f {
			t.Error("SetIndent() should return the same formatter")
		}
		if f.indentLevel != 2 {
			t.Errorf("SetIndent(2).indentLevel = %d, want 2", f.indentLevel)
		}
	})

	t.Run("SetWidth modifies width", func(t *testing.T) {
		f := NewFormatter()
		result := f.SetWidth(100)
		if result != f {
			t.Error("SetWidth() should return the same formatter")
		}
		if f.width != 100 {
			t.Errorf("SetWidth(100).width = %d, want 100", f.width)
		}
	})

	t.Run("Indent returns correct indentation", func(t *testing.T) {
		f := NewFormatter()
		if got := f.Indent(); got != "" {
			t.Errorf("Indent() with level 0 = %q, want empty", got)
		}
		f.SetIndent(2)
		if got := f.Indent(); got != "    " {
			t.Errorf("Indent() with level 2 = %q, want %q", got, "    ")
		}
	})

	t.Run("Section formats header", func(t *testing.T) {
		f := NewFormatter()
		got := f.Section("Test Title")
		if !contains(got, "Test Title") {
			t.Errorf("Section() should contain title")
		}
	})

	t.Run("Section with empty title", func(t *testing.T) {
		f := NewFormatter()
		got := f.Section("")
		if !contains(got, "─") {
			t.Errorf("Section(\"\") should contain separator")
		}
	})

	t.Run("Subsection formats header", func(t *testing.T) {
		f := NewFormatter()
		got := f.Subsection("Sub")
		if !contains(got, "Sub") {
			t.Errorf("Subsection() should contain title")
		}
	})

	t.Run("KeyValue formats pair", func(t *testing.T) {
		f := NewFormatter()
		got := f.KeyValue("name", "value")
		if !contains(got, "name:") {
			t.Errorf("KeyValue() should contain key")
		}
		if !contains(got, "value") {
			t.Errorf("KeyValue() should contain value")
		}
	})

	t.Run("KeyValues formats multiple pairs", func(t *testing.T) {
		f := NewFormatter()
		pairs := map[string]string{
			"key1":      "value1",
			"longerkey": "value2",
		}
		got := f.KeyValues(pairs)
		if !contains(got, "key1:") {
			t.Errorf("KeyValues() should contain first key")
		}
		if !contains(got, "longerkey:") {
			t.Errorf("KeyValues() should contain second key")
		}
	})

	t.Run("List formats items", func(t *testing.T) {
		f := NewFormatter()
		items := []string{"item1", "item2", "item3"}
		got := f.List(items)
		if !contains(got, "item1") {
			t.Errorf("List() should contain first item")
		}
		// Should have numbered items for first 10
		if !contains(got, "1.") {
			t.Errorf("List() should number items")
		}
	})

	t.Run("List with many items", func(t *testing.T) {
		f := NewFormatter()
		items := make([]string, 15)
		for i := range items {
			items[i] = fmt.Sprintf("item%d", i)
		}
		got := f.List(items)
		if !contains(got, "1.") {
			t.Errorf("List() should number early items")
		}
		// Items after 10 should use bullets
		if !contains(got, "•") {
			t.Errorf("List() should use bullets for items after 10")
		}
	})

	t.Run("DefinitionList formats terms", func(t *testing.T) {
		f := NewFormatter()
		terms := map[string]string{
			"term1": "definition1",
			"term2": "definition2",
		}
		got := f.DefinitionList(terms)
		if !contains(got, "term1") {
			t.Errorf("DefinitionList() should contain first term")
		}
		if !contains(got, "definition1") {
			t.Errorf("DefinitionList() should contain first definition")
		}
	})

	t.Run("CodeBlock formats code", func(t *testing.T) {
		f := NewFormatter()
		code := "func main() {}\nreturn"
		got := f.CodeBlock(code, "go")
		if !contains(got, "```go") {
			t.Errorf("CodeBlock() should contain language specifier")
		}
		if !contains(got, "func main()") {
			t.Errorf("CodeBlock() should contain code")
		}
	})

	t.Run("CodeBlock without language", func(t *testing.T) {
		f := NewFormatter()
		code := "some code"
		got := f.CodeBlock(code, "")
		if !contains(got, "```") {
			t.Errorf("CodeBlock() should contain code fence")
		}
	})

	t.Run("Timestamp formats time", func(t *testing.T) {
		f := NewFormatter()
		now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
		got := f.Timestamp(now)
		if !contains(got, "2025-01-15") {
			t.Errorf("Timestamp() should contain date")
		}
		if !contains(got, "10:30") {
			t.Errorf("Timestamp() should contain time")
		}
	})

	t.Run("ShortTimestamp formats time", func(t *testing.T) {
		f := NewFormatter()
		now := time.Date(2025, 1, 15, 10, 30, 45, 0, time.UTC)
		got := f.ShortTimestamp(now)
		if !contains(got, "10:30:45") {
			t.Errorf("ShortTimestamp() = %q, want time", got)
		}
	})

	t.Run("Truncate short string", func(t *testing.T) {
		f := NewFormatter()
		got := f.Truncate("short", 10)
		if got != "short" {
			t.Errorf("Truncate(\"short\", 10) = %q, want \"short\"", got)
		}
	})

	t.Run("Truncate long string", func(t *testing.T) {
		f := NewFormatter()
		got := f.Truncate("this is a very long string", 10)
		if got != "this is..." {
			t.Errorf("Truncate() = %q, want \"this is...\"", got)
		}
	})

	t.Run("Truncate with max length 3", func(t *testing.T) {
		f := NewFormatter()
		got := f.Truncate("longstring", 3)
		if got != "..." {
			t.Errorf("Truncate(_, 3) = %q, want \"...\"", got)
		}
	})

	t.Run("Table formats headers and rows", func(t *testing.T) {
		f := NewFormatter()
		headers := []string{"Name", "Value"}
		rows := [][]string{
			{"key1", "val1"},
			{"key2", "val2"},
		}
		got := f.Table(headers, rows)
		if !contains(got, "Name") {
			t.Errorf("Table() should contain first header")
		}
		if !contains(got, "Value") {
			t.Errorf("Table() should contain second header")
		}
		if !contains(got, "key1") {
			t.Errorf("Table() should contain first row data")
		}
	})

	t.Run("Table with empty rows", func(t *testing.T) {
		f := NewFormatter()
		headers := []string{"Col1", "Col2"}
		got := f.Table(headers, [][]string{})
		if !contains(got, "Col1") {
			t.Errorf("Table() should contain headers even with no rows")
		}
	})
}

// TestFormatterHelperFunctions tests the global helper functions.
func TestFormatterHelperFunctions(t *testing.T) {
	// Disable colors for predictable output
	SetColorsEnabled(false)
	defer SetColorsEnabled(true)

	t.Run("Section helper function", func(t *testing.T) {
		got := Section("Title")
		if !contains(got, "Title") {
			t.Errorf("Section() should contain title")
		}
	})

	t.Run("KeyValue helper function", func(t *testing.T) {
		got := KeyValue("key", "value")
		if !contains(got, "key:") {
			t.Errorf("KeyValue() should contain key")
		}
	})

	t.Run("List helper function", func(t *testing.T) {
		got := List([]string{"a", "b"})
		if !contains(got, "a") {
			t.Errorf("List() should contain items")
		}
	})

	t.Run("Truncate helper function", func(t *testing.T) {
		got := Truncate("very long text", 8)
		if got != "very ..." {
			t.Errorf("Truncate() = %q, want \"very ...\"", got)
		}
	})
}

// TestRelativeTimestamp tests the relative timestamp formatting.
func TestRelativeTimestamp(t *testing.T) {
	f := NewFormatter()

	t.Run("just now", func(t *testing.T) {
		now := time.Now().Add(-10 * time.Second)
		got := f.RelativeTimestamp(now)
		if got != "just now" {
			t.Errorf("RelativeTimestamp(10s ago) = %q, want \"just now\"", got)
		}
	})

	t.Run("minutes ago", func(t *testing.T) {
		past := time.Now().Add(-5 * time.Minute)
		got := f.RelativeTimestamp(past)
		if !contains(got, "mins ago") {
			t.Errorf("RelativeTimestamp(5m ago) = %q, want minutes", got)
		}
	})

	t.Run("hours ago", func(t *testing.T) {
		past := time.Now().Add(-3 * time.Hour)
		got := f.RelativeTimestamp(past)
		if !contains(got, "hrs ago") {
			t.Errorf("RelativeTimestamp(3h ago) = %q, want hours", got)
		}
	})

	t.Run("days ago", func(t *testing.T) {
		past := time.Now().Add(-5 * 24 * time.Hour)
		got := f.RelativeTimestamp(past)
		if !contains(got, "days ago") {
			t.Errorf("RelativeTimestamp(5d ago) = %q, want days", got)
		}
	})

	t.Run("old dates use absolute format", func(t *testing.T) {
		past := time.Now().Add(-100 * 24 * time.Hour)
		got := f.RelativeTimestamp(past)
		// Should be in YYYY-MM-DD format
		if len(got) < 10 {
			t.Errorf("RelativeTimestamp(very old) = %q, want date format", got)
		}
	})
}
