package claudemcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// mcpConfigFile is the JSON structure claude reads from --mcp-config.
type mcpConfigFile struct {
	MCPServers map[string]mcpServerEntry `json:"mcpServers"`
}

type mcpServerEntry struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// writeMCPConfig materialises the per-task mcp-config.json file consumed by
// `claude --mcp-config <path>`.
//
// If serverCmd[0] is the bare name "kvelmo", it is rewritten to the absolute
// path of the currently-running binary via os.Executable() so the spawned
// claude can locate it regardless of the PATH it inherits (which often
// excludes ~/.local/bin when claude is launched from a GUI or launchd
// context). Any other value is passed through unchanged so users can
// configure an absolute path or a different launcher explicitly.
//
// taskID, worktree, workSocket, adapterSocket are exported into the spawned
// MCP server's environment so it can scope every RPC to the right worktree
// and emit rendezvous events back to us.
func writeMCPConfig(
	dest string,
	serverCmd []string,
	taskID, worktree, workSocket, adapterSocket string,
	parentPID int,
) error {
	if len(serverCmd) == 0 {
		return errors.New("mcp server command is empty")
	}
	resolved := append([]string(nil), serverCmd...)
	if resolved[0] == "kvelmo" {
		if exe, err := os.Executable(); err == nil {
			resolved[0] = exe
		}
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return fmt.Errorf("create mcp config dir: %w", err)
	}
	env := map[string]string{
		"KVELMO_TASK_ID":         taskID,
		"KVELMO_WORKTREE":        worktree,
		"KVELMO_WORKTREE_SOCKET": workSocket,
		"KVELMO_ADAPTER_SOCKET":  adapterSocket,
	}
	if parentPID > 0 {
		env["KVELMO_PARENT_PID"] = strconv.Itoa(parentPID)
	}
	cfg := mcpConfigFile{
		MCPServers: map[string]mcpServerEntry{
			"kvelmo": {
				Command: resolved[0],
				Args:    append([]string(nil), resolved[1:]...),
				Env:     env,
			},
		},
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal mcp config: %w", err)
	}
	if err := os.WriteFile(dest, data, 0o600); err != nil {
		return fmt.Errorf("write mcp config: %w", err)
	}

	return nil
}
