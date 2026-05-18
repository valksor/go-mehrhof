package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/internal/mcp"
	"github.com/valksor/kvelmo/internal/socket"
)

// MCPCmd implements the `kvelmo mcp` subcommand, an MCP server over stdio
// that bridges an interactive Claude Code session (launched by the
// agent/claudemcp adapter) to a per-worktree kvelmo socket. Claude calls
// MCP tools; the server translates them into kvelmo RPC calls.
//
// This subcommand is intended to be spawned by `claude --mcp-config <file>`
// — never run directly by a human. It reads its environment from:
//
//	KVELMO_TASK_ID         current task ID
//	KVELMO_WORKTREE        worktree absolute path (used for diagnostics)
//	KVELMO_WORKTREE_SOCKET path to the per-worktree kvelmo socket
//	KVELMO_ADAPTER_SOCKET  (optional) Unix socket the adapter listens on for
//	                       rendezvous events (signal_complete, signal_failure)
//	KVELMO_PARENT_PID      (optional) PID of the spawning `claude` process;
//	                       if set, the server exits when that PID disappears
var MCPCmd = &cobra.Command{
	Use:    "mcp",
	Short:  "MCP server over stdio (spawned by claude --mcp-config; do not run manually)",
	Hidden: true,
	RunE:   runMCP,
}

const mcpUsageHint = "this subcommand is spawned by the claude-mcp adapter (agent/claudemcp) via claude's --mcp-config; do not run it directly"

func runMCP(cmd *cobra.Command, _ []string) error {
	taskID := os.Getenv("KVELMO_TASK_ID")
	worktree := os.Getenv("KVELMO_WORKTREE")
	socketPath := os.Getenv("KVELMO_WORKTREE_SOCKET")
	adapterSocket := os.Getenv("KVELMO_ADAPTER_SOCKET")
	parentPIDStr := os.Getenv("KVELMO_PARENT_PID")

	if taskID == "" {
		return fmt.Errorf("KVELMO_TASK_ID is not set — %s", mcpUsageHint)
	}
	if socketPath == "" {
		return fmt.Errorf("KVELMO_WORKTREE_SOCKET is not set — %s", mcpUsageHint)
	}

	ctx, cancel := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	rpc, err := socket.NewClient(socketPath, socket.WithTimeout(60*time.Second))
	if err != nil {
		return fmt.Errorf("connect to worktree socket %s: %w", socketPath, err)
	}
	defer func() { _ = rpc.Close() }()

	client := mcp.NewClient(rpc, taskID, worktree, socketPath)
	reg := mcp.NewToolRegistry()
	mcp.RegisterTools(reg, client)

	// Wire rendezvous notifications back to the adapter (if configured).
	if adapterSocket != "" {
		dialer := func(evt mcp.RendezvousEvent) {
			emitRendezvous(adapterSocket, evt)
		}
		mcp.SetRendezvousSink(dialer)
	}

	// Watchdog: exit if the parent claude process disappears.
	if parentPIDStr != "" {
		if pid, err := strconv.Atoi(parentPIDStr); err == nil && pid > 0 {
			go watchParent(ctx, pid, cancel)
		} else {
			slog.Warn("kvelmo mcp: invalid KVELMO_PARENT_PID", "value", parentPIDStr)
		}
	}

	srv := mcp.NewServer(reg, mcp.WithServerInfo(mcp.ServerInfo{
		Name:    "kvelmo-mcp",
		Version: "1.0.0",
	}))

	slog.Info("kvelmo mcp starting",
		"task_id", taskID,
		"worktree", worktree,
		"socket", socketPath,
		"adapter_socket", adapterSocket != "",
		"parent_pid", parentPIDStr)

	if err := srv.Serve(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("mcp serve: %w", err)
	}

	return nil
}

// watchParent polls the given PID every 5s and cancels the context when it
// disappears. Prevents the MCP subprocess from outliving the claude process
// that spawned it.
func watchParent(ctx context.Context, pid int, cancel context.CancelFunc) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := syscall.Kill(pid, 0); err != nil {
				slog.Info("kvelmo mcp: parent process gone, shutting down", "pid", pid)
				cancel()

				return
			}
		}
	}
}

// emitRendezvous sends a single JSON-encoded RendezvousEvent to the adapter's
// listening Unix socket, then closes the connection. A short dial timeout
// keeps the MCP tool call snappy if the adapter has gone away.
func emitRendezvous(socketPath string, evt mcp.RendezvousEvent) {
	dialCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(dialCtx, "unix", socketPath)
	if err != nil {
		slog.Warn("kvelmo mcp: rendezvous dial failed", "socket", socketPath, "error", err)

		return
	}
	defer func() { _ = conn.Close() }()

	data, err := json.Marshal(evt)
	if err != nil {
		slog.Warn("kvelmo mcp: rendezvous marshal failed", "error", err)

		return
	}
	_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write(append(data, '\n')); err != nil {
		slog.Warn("kvelmo mcp: rendezvous write failed", "error", err)
	}
}
