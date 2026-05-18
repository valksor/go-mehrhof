package claudemcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteMCPConfig(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "mcp-config.json")

	err := writeMCPConfig(
		dest,
		[]string{"kvelmo", "mcp", "--stdio"},
		"task-123",
		"/abs/worktree",
		"/tmp/worktree.sock",
		"/tmp/adapter.sock",
		4242,
	)
	if err != nil {
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

	entry, ok := got.MCPServers["kvelmo"]
	if !ok {
		t.Fatalf("missing 'kvelmo' server entry; got: %+v", got)
	}
	// The bare "kvelmo" command is resolved to an absolute path via
	// os.Executable() so the spawned claude can find it regardless of PATH.
	// In tests this is the test binary; the only invariant we can check is
	// that it is absolute and non-empty.
	if entry.Command == "" || entry.Command == "kvelmo" {
		t.Errorf("command = %q, want absolute resolved path", entry.Command)
	}
	if len(entry.Args) != 2 || entry.Args[0] != "mcp" || entry.Args[1] != "--stdio" {
		t.Errorf("args = %v, want [mcp --stdio]", entry.Args)
	}
	wantEnv := map[string]string{
		"KVELMO_TASK_ID":         "task-123",
		"KVELMO_WORKTREE":        "/abs/worktree",
		"KVELMO_WORKTREE_SOCKET": "/tmp/worktree.sock",
		"KVELMO_ADAPTER_SOCKET":  "/tmp/adapter.sock",
		"KVELMO_PARENT_PID":      "4242",
	}
	for k, v := range wantEnv {
		if entry.Env[k] != v {
			t.Errorf("env[%s] = %q, want %q", k, entry.Env[k], v)
		}
	}

	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("file mode = %o, want 0600", mode)
	}
}
