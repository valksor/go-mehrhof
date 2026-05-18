package claudemcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestWriteMCPConfigExplicitPath verifies that an explicit (non-"kvelmo")
// serverCmd[0] is passed through unchanged — i.e. user overrides via config
// are honoured.
func TestWriteMCPConfigExplicitPath(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "mcp-config.json")
	custom := "/opt/custom/launcher"

	if err := writeMCPConfig(
		dest,
		[]string{custom, "mcp", "--stdio"},
		"task-123", "/abs/worktree", "/tmp/worktree.sock", "/tmp/adapter.sock", 0,
	); err != nil {
		t.Fatalf("writeMCPConfig: %v", err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var got mcpConfigFile
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.MCPServers["kvelmo"].Command != custom {
		t.Errorf("command = %q, want %q (user override should not be rewritten)",
			got.MCPServers["kvelmo"].Command, custom)
	}
}
