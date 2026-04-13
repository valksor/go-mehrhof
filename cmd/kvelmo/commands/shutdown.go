package commands

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/internal/socket"
)

var (
	shutdownTimeout time.Duration
	shutdownForce   bool
)

var ShutdownCmd = &cobra.Command{
	Use:   "shutdown",
	Short: "Shutdown the worktree socket server",
	Long: `Shutdown the worktree socket server for the current directory.

By default, sends a graceful shutdown request and waits for the socket to exit. Use --force to skip the graceful wait and unregister immediately.

Note: This stops the kvelmo server, not the current operation. Use 'kvelmo stop' to stop a running operation.`,
	RunE: runShutdown,
}

func init() {
	ShutdownCmd.Flags().DurationVarP(&shutdownTimeout, "timeout", "t", 2*time.Second, "Graceful shutdown timeout")
	ShutdownCmd.Flags().BoolVarP(&shutdownForce, "force", "f", false, "Skip graceful shutdown and unregister immediately")
}

func runShutdown(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	wtPath := socket.WorktreeSocketPath(cwd)
	if !socket.SocketExists(wtPath) {
		fmt.Println("No worktree socket running")

		return nil
	}

	if !shutdownForce {
		client, err := socket.NewClient(wtPath, socket.WithTimeout(shutdownTimeout))
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
			_, _ = client.Call(ctx, "shutdown", nil)
			cancel()
			_ = client.Close()

			deadline := time.Now().Add(shutdownTimeout)
			for time.Now().Before(deadline) {
				if !socket.SocketExists(wtPath) {
					fmt.Printf("Shutdown worktree: %s\n", cwd)

					return nil
				}
				time.Sleep(100 * time.Millisecond)
			}
			fmt.Println("Graceful shutdown timed out, forcing...")
		}
	}

	fmt.Printf("Shutdown worktree: %s\n", cwd)

	globalPath := socket.GlobalSocketPath()
	if socket.SocketExists(globalPath) {
		client, err := socket.NewClient(globalPath, socket.WithTimeout(time.Second))
		if err == nil {
			defer func() { _ = client.Close() }()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			id := socket.WorktreeIDFromPath(cwd)
			_, _ = client.Call(ctx, "projects.unregister", socket.UnregisterParams{ID: id})
		}
	}

	return nil
}
