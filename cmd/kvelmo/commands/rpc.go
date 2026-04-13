package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/internal/socket"
)

var rpcGlobal bool

var RPCCmd = &cobra.Command{
	Use:   "rpc <method> [params-json]",
	Short: "Send a raw JSON-RPC call to the socket",
	Long: `Send an arbitrary JSON-RPC method call to the worktree or global socket.

This is a power-user tool for scripting and debugging. The response
is printed as raw JSON to stdout.

Examples:
  kvelmo rpc status                          # Call status on worktree socket
  kvelmo rpc plan '{"force":true}'           # Call plan with params
  kvelmo rpc --global workers.list           # Call on global socket
  kvelmo rpc --global metrics                # Get metrics from global socket`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runRPC,
}

func init() {
	RPCCmd.Flags().BoolVar(&rpcGlobal, "global", false, "Send to global socket instead of worktree socket")
}

func runRPC(_ *cobra.Command, args []string) error {
	method := args[0]

	var params json.RawMessage
	if len(args) > 1 {
		if !json.Valid([]byte(args[1])) {
			return errors.New("invalid JSON params: " + args[1])
		}
		params = json.RawMessage(args[1])
	}

	var socketPath string
	if rpcGlobal {
		socketPath = socket.GlobalSocketPath()
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		socketPath = socket.WorktreeSocketPath(cwd)
	}

	if !socket.SocketExists(socketPath) {
		if rpcGlobal {
			return errors.New("global socket not running (run 'kvelmo serve' first)")
		}

		return errors.New("worktree socket not running (run 'kvelmo start' first)")
	}

	client, err := socket.NewClient(socketPath, socket.WithTimeout(30*time.Second))
	if err != nil {
		return fmt.Errorf("connect to socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, method, params)
	if err != nil {
		return fmt.Errorf("rpc call %s: %w", method, err)
	}

	if resp.Error != nil {
		return fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	fmt.Println(string(resp.Result))

	return nil
}
