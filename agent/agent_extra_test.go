package agent

import (
	"testing"

	"github.com/valksor/kvelmo/agent/permission"
)

func TestKvelmoPermissionHandler_SafeReadTools(t *testing.T) {
	safeReadTools := []string{"Read", "read", "Glob", "glob", "Grep", "grep", "LS", "ls", "search", "Search", "list_dir", "ListDir", "read_file", "ReadFile"}
	for _, tool := range safeReadTools {
		req := PermissionRequest{Tool: tool}
		if !KvelmoPermissionHandler(req) {
			t.Errorf("KvelmoPermissionHandler(%q) = false, want true", tool)
		}
	}
}

func TestKvelmoPermissionHandler_WriteAllowed(t *testing.T) {
	req := PermissionRequest{Tool: "Write"}
	if !KvelmoPermissionHandler(req) {
		t.Error("KvelmoPermissionHandler(Write) = false, want true")
	}
}

func TestKvelmoPermissionHandler_WriteVariants(t *testing.T) {
	for _, tool := range []string{"Write", "write", "WRITE"} {
		req := PermissionRequest{Tool: tool}
		if !KvelmoPermissionHandler(req) {
			t.Errorf("KvelmoPermissionHandler(%q) = false, want true", tool)
		}
	}
}

func TestKvelmoPermissionHandler_EditAllowed(t *testing.T) {
	req := PermissionRequest{Tool: "Edit"}
	if !KvelmoPermissionHandler(req) {
		t.Error("KvelmoPermissionHandler(Edit) = false, want true")
	}
}

func TestKvelmoPermissionHandler_EditVariants(t *testing.T) {
	for _, tool := range []string{"Edit", "edit", "EDIT"} {
		req := PermissionRequest{Tool: tool}
		if !KvelmoPermissionHandler(req) {
			t.Errorf("KvelmoPermissionHandler(%q) = false, want true", tool)
		}
	}
}

func TestKvelmoPermissionHandler_BashSafeCommand(t *testing.T) {
	req := PermissionRequest{
		Tool:  "Bash",
		Input: map[string]any{"command": "ls -la"},
	}
	// Bash with a safe command: danger is Safe, Bash is in allowed list
	if !KvelmoPermissionHandler(req) {
		t.Error("KvelmoPermissionHandler(Bash, ls -la) = false, want true")
	}
}

func TestKvelmoPermissionHandler_BashDangerousCommand(t *testing.T) {
	req := PermissionRequest{
		Tool:  "Bash",
		Input: map[string]any{"command": "rm -rf /"},
	}
	// Dangerous operations must always be denied
	if KvelmoPermissionHandler(req) {
		t.Error("KvelmoPermissionHandler(Bash, rm -rf /) = true, want false (dangerous)")
	}
}

func TestKvelmoPermissionHandler_UnknownToolDenied(t *testing.T) {
	unknownTools := []string{"unknown_dangerous_tool", "SomeRandomTool", "exec", "spawn"}
	for _, tool := range unknownTools {
		req := PermissionRequest{Tool: tool}
		if KvelmoPermissionHandler(req) {
			t.Errorf("KvelmoPermissionHandler(%q) = true, want false (not in allowed list)", tool)
		}
	}
}

func TestKvelmoPermissionHandler_DangerousOperationDenied(t *testing.T) {
	// Even if the tool is Bash (normally allowed), dangerous commands are blocked
	tests := []struct {
		name       string
		tool       string
		input      map[string]any
		wantDanger permission.DangerLevel
	}{
		{
			name:       "rm -rf /",
			tool:       "Bash",
			input:      map[string]any{"command": "rm -rf /"},
			wantDanger: permission.Dangerous,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := PermissionRequest{Tool: tt.tool, Input: tt.input}
			danger := permission.DetectDanger(req.Tool, req.Input)
			if danger.Level != tt.wantDanger {
				t.Errorf("danger level = %v, want %v", danger.Level, tt.wantDanger)
			}
			if KvelmoPermissionHandler(req) {
				t.Error("KvelmoPermissionHandler() = true, want false for dangerous operation")
			}
		})
	}
}

func TestKvelmoPermissionHandler_BashVariants(t *testing.T) {
	// bash in any case should be allowed for safe commands
	for _, tool := range []string{"Bash", "bash", "BASH"} {
		req := PermissionRequest{
			Tool:  tool,
			Input: map[string]any{"command": "echo hello"},
		}
		if !KvelmoPermissionHandler(req) {
			t.Errorf("KvelmoPermissionHandler(%q, echo hello) = false, want true", tool)
		}
	}
}
