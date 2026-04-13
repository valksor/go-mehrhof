package agent

import (
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent/permission"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.PreferWebSocket {
		t.Error("DefaultConfig().PreferWebSocket should be true")
	}
	if cfg.Timeout != 30*time.Minute {
		t.Errorf("DefaultConfig().Timeout = %v, want 30m", cfg.Timeout)
	}
	if cfg.RetryCount != 3 {
		t.Errorf("DefaultConfig().RetryCount = %d, want 3", cfg.RetryCount)
	}
	if cfg.RetryDelay != time.Second {
		t.Errorf("DefaultConfig().RetryDelay = %v, want 1s", cfg.RetryDelay)
	}
	if cfg.Environment == nil {
		t.Error("DefaultConfig().Environment should not be nil")
	}
	if cfg.PermissionHandler == nil {
		t.Error("DefaultConfig().PermissionHandler should not be nil")
	}
}

func TestConfigMerge_Command(t *testing.T) {
	base := DefaultConfig()
	other := Config{Command: []string{"codex"}}
	merged := base.Merge(other)
	if len(merged.Command) != 1 || merged.Command[0] != "codex" {
		t.Errorf("Merge().Command = %v, want [codex]", merged.Command)
	}
}

func TestConfigMerge_Args(t *testing.T) {
	base := Config{Args: []string{"--base"}}
	other := Config{Args: []string{"--extra"}}
	merged := base.Merge(other)
	if len(merged.Args) != 2 {
		t.Errorf("Merge().Args len = %d, want 2", len(merged.Args))
	}
}

func TestConfigMerge_WorkDir(t *testing.T) {
	base := DefaultConfig()
	other := Config{WorkDir: "/tmp/project"}
	merged := base.Merge(other)
	if merged.WorkDir != "/tmp/project" {
		t.Errorf("Merge().WorkDir = %q, want /tmp/project", merged.WorkDir)
	}
}

func TestConfigMerge_Timeout(t *testing.T) {
	base := DefaultConfig()
	other := Config{Timeout: 5 * time.Minute}
	merged := base.Merge(other)
	if merged.Timeout != 5*time.Minute {
		t.Errorf("Merge().Timeout = %v, want 5m", merged.Timeout)
	}
}

func TestConfigMerge_Environment(t *testing.T) {
	base := Config{Environment: map[string]string{"A": "1"}}
	other := Config{Environment: map[string]string{"B": "2"}}
	merged := base.Merge(other)
	if merged.Environment["A"] != "1" || merged.Environment["B"] != "2" {
		t.Errorf("Merge().Environment = %v, want A=1 B=2", merged.Environment)
	}
}

func TestConfigMerge_PermissionHandler(t *testing.T) {
	base := DefaultConfig()
	called := false
	other := Config{PermissionHandler: func(_ PermissionRequest) bool {
		called = true

		return true
	}}
	merged := base.Merge(other)
	merged.PermissionHandler(PermissionRequest{})
	if !called {
		t.Error("Merge().PermissionHandler should use other.PermissionHandler when non-nil")
	}
}

func TestConfigMerge_NilOtherValues(t *testing.T) {
	// other with zero/nil values should not override base
	base := Config{
		Command:     []string{"claude"},
		WorkDir:     "/original",
		Timeout:     10 * time.Minute,
		Environment: map[string]string{"KEY": "val"},
	}
	other := Config{} // all zero
	merged := base.Merge(other)
	if merged.Command[0] != "claude" {
		t.Errorf("Merge() nil Command should keep base: %v", merged.Command)
	}
	if merged.WorkDir != "/original" {
		t.Errorf("Merge() empty WorkDir should keep base: %q", merged.WorkDir)
	}
	if merged.Timeout != 10*time.Minute {
		t.Errorf("Merge() zero Timeout should keep base: %v", merged.Timeout)
	}
	if merged.Environment["KEY"] != "val" {
		t.Errorf("Merge() nil Environment should keep base: %v", merged.Environment)
	}
}

func TestConfigMerge_BaseNilEnv(t *testing.T) {
	// When base.Environment is nil and other.Environment is non-nil
	base := Config{}
	other := Config{Environment: map[string]string{"X": "y"}}
	merged := base.Merge(other)
	if merged.Environment["X"] != "y" {
		t.Errorf("Merge() with nil base env: Environment = %v, want X=y", merged.Environment)
	}
}

func TestEvaluatePermission_DangerousDenied(t *testing.T) {
	req := PermissionRequest{
		Tool:  "Bash",
		Input: map[string]any{"command": "rm -rf /"},
	}
	result := EvaluatePermission(req)
	if result.Approved {
		t.Error("Dangerous operation should be denied")
	}
	if result.DangerLevel != permission.Dangerous {
		t.Errorf("DangerLevel = %v, want Dangerous", result.DangerLevel)
	}
	if result.DangerReason == "" {
		t.Error("DangerReason should be set for dangerous operations")
	}
}

func TestEvaluatePermission_SafeToolApproved(t *testing.T) {
	req := PermissionRequest{
		Tool: "Read",
	}
	result := EvaluatePermission(req)
	if !result.Approved {
		t.Error("Safe tool should be approved")
	}
	if result.DangerLevel != permission.Safe {
		t.Errorf("DangerLevel = %v, want Safe", result.DangerLevel)
	}
}

func TestEvaluatePermission_UnsafeToolDenied(t *testing.T) {
	req := PermissionRequest{
		Tool:  "Bash",
		Input: map[string]any{"command": "ls"},
	}
	result := EvaluatePermission(req)
	if result.Approved {
		t.Error("Bash is not in safe list, should be denied")
	}
	if result.DangerLevel != permission.Safe {
		t.Errorf("ls is safe but Bash not in safe tools: DangerLevel = %v", result.DangerLevel)
	}
}

func TestEvaluatePermission_CautionNotAutoApproved(t *testing.T) {
	req := PermissionRequest{
		Tool:  "Bash",
		Input: map[string]any{"command": "sudo apt update"},
	}
	result := EvaluatePermission(req)
	if result.Approved {
		t.Error("Caution-level Bash should not be auto-approved")
	}
	if result.DangerLevel != permission.Caution {
		t.Errorf("DangerLevel = %v, want Caution", result.DangerLevel)
	}
}

func TestDefaultPermissionHandler_DangerousDenied(t *testing.T) {
	req := PermissionRequest{
		Tool:  "Bash",
		Input: map[string]any{"command": "rm -rf /"},
	}
	if DefaultPermissionHandler(req) {
		t.Error("DefaultPermissionHandler should deny dangerous operations")
	}
}

func TestDefaultPermissionHandler_SafeToolApproved(t *testing.T) {
	tests := []string{"Read", "read_file", "Glob", "glob", "Grep", "grep", "LS", "list_dir", "search"}
	for _, tool := range tests {
		req := PermissionRequest{Tool: tool}
		if !DefaultPermissionHandler(req) {
			t.Errorf("DefaultPermissionHandler should approve %q", tool)
		}
	}
}
