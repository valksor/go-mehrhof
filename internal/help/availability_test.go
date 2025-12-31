package help

import "testing"

func TestIsAvailable_AlwaysAvailable(t *testing.T) {
	ctx := &HelpContext{} // Empty context - no task, no workspace

	alwaysAvailable := []string{
		"start", "auto", "list", "init", "config",
		"templates", "providers", "agents", "plugins",
		"workflow", "version", "update", "completion",
		"plan", "help",
	}

	for _, cmd := range alwaysAvailable {
		if !IsAvailable(cmd, ctx) {
			t.Errorf("expected %q to be available with empty context", cmd)
		}
	}
}

func TestIsAvailable_NeedsActiveTask(t *testing.T) {
	tests := []struct {
		name string
		ctx  *HelpContext
		cmd  string
		want bool
	}{
		{
			name: "status without task",
			ctx:  &HelpContext{HasActiveTask: false},
			cmd:  "status",
			want: false,
		},
		{
			name: "status with task",
			ctx:  &HelpContext{HasActiveTask: true},
			cmd:  "status",
			want: true,
		},
		{
			name: "guide without task",
			ctx:  &HelpContext{HasActiveTask: false},
			cmd:  "guide",
			want: false,
		},
		{
			name: "guide with task",
			ctx:  &HelpContext{HasActiveTask: true},
			cmd:  "guide",
			want: true,
		},
		{
			name: "continue without task",
			ctx:  &HelpContext{HasActiveTask: false},
			cmd:  "continue",
			want: false,
		},
		{
			name: "cost without task",
			ctx:  &HelpContext{HasActiveTask: false},
			cmd:  "cost",
			want: false,
		},
		{
			name: "note without task",
			ctx:  &HelpContext{HasActiveTask: false},
			cmd:  "note",
			want: false,
		},
		{
			name: "abandon without task",
			ctx:  &HelpContext{HasActiveTask: false},
			cmd:  "abandon",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsAvailable(tt.cmd, tt.ctx)
			if got != tt.want {
				t.Errorf("IsAvailable(%q) = %v, want %v", tt.cmd, got, tt.want)
			}
		})
	}
}

func TestIsAvailable_NeedsSpecifications(t *testing.T) {
	tests := []struct {
		name string
		ctx  *HelpContext
		cmd  string
		want bool
	}{
		{
			name: "implement without specs",
			ctx:  &HelpContext{HasActiveTask: true, HasSpecifications: false},
			cmd:  "implement",
			want: false,
		},
		{
			name: "implement with specs",
			ctx:  &HelpContext{HasActiveTask: true, HasSpecifications: true},
			cmd:  "implement",
			want: true,
		},
		{
			name: "review without specs",
			ctx:  &HelpContext{HasActiveTask: true, HasSpecifications: false},
			cmd:  "review",
			want: false,
		},
		{
			name: "review with specs",
			ctx:  &HelpContext{HasActiveTask: true, HasSpecifications: true},
			cmd:  "review",
			want: true,
		},
		{
			name: "finish without specs",
			ctx:  &HelpContext{HasActiveTask: true, HasSpecifications: false},
			cmd:  "finish",
			want: false,
		},
		{
			name: "finish with specs",
			ctx:  &HelpContext{HasActiveTask: true, HasSpecifications: true},
			cmd:  "finish",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsAvailable(tt.cmd, tt.ctx)
			if got != tt.want {
				t.Errorf("IsAvailable(%q) = %v, want %v", tt.cmd, got, tt.want)
			}
		})
	}
}

func TestIsAvailable_NeedsGit(t *testing.T) {
	tests := []struct {
		name string
		ctx  *HelpContext
		cmd  string
		want bool
	}{
		{
			name: "undo without git",
			ctx:  &HelpContext{HasActiveTask: true, UseGit: false},
			cmd:  "undo",
			want: false,
		},
		{
			name: "undo with git",
			ctx:  &HelpContext{HasActiveTask: true, UseGit: true},
			cmd:  "undo",
			want: true,
		},
		{
			name: "undo without active task",
			ctx:  &HelpContext{HasActiveTask: false, UseGit: true},
			cmd:  "undo",
			want: false,
		},
		{
			name: "redo without git",
			ctx:  &HelpContext{HasActiveTask: true, UseGit: false},
			cmd:  "redo",
			want: false,
		},
		{
			name: "redo with git",
			ctx:  &HelpContext{HasActiveTask: true, UseGit: true},
			cmd:  "redo",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsAvailable(tt.cmd, tt.ctx)
			if got != tt.want {
				t.Errorf("IsAvailable(%q) = %v, want %v", tt.cmd, got, tt.want)
			}
		})
	}
}

func TestIsAvailable_UnknownCommand(t *testing.T) {
	ctx := &HelpContext{} // Empty context

	// Unknown commands should be treated as always available
	if !IsAvailable("unknown-command", ctx) {
		t.Error("expected unknown command to be available")
	}
}

func TestGetReason(t *testing.T) {
	tests := []struct {
		cmd  string
		want string
	}{
		{"start", ""},
		{"status", "needs active task"},
		{"implement", "needs specifications"},
		{"undo", "needs git task"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			got := GetReason(tt.cmd)
			if got != tt.want {
				t.Errorf("GetReason(%q) = %q, want %q", tt.cmd, got, tt.want)
			}
		})
	}
}
