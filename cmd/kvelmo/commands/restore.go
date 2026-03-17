package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/valksor/kvelmo/pkg/backup"
	"github.com/valksor/kvelmo/pkg/cli"
	"github.com/valksor/kvelmo/pkg/meta"
	"github.com/valksor/kvelmo/pkg/socket"
)

var restoreJSON bool

var RestoreCmd = &cobra.Command{
	Use:   "restore <archive-path>",
	Short: "Restore kvelmo state from a backup archive",
	Long: `Restore kvelmo state from a previously created backup.

The archive is extracted to ~/.valksor/kvelmo/ by default.

Examples:
  kvelmo restore backup.tar.gz            # Restore to default location`,
	Args: cobra.ExactArgs(1),
	RunE: runRestore,
}

func init() {
	RestoreCmd.Flags().BoolVar(&restoreJSON, "json", false, "Output as JSON")
}

func runRestore(_ *cobra.Command, args []string) error {
	archivePath := args[0]

	globalPath := socket.GlobalSocketPath()
	if !socket.SocketExists(globalPath) {
		return fmt.Errorf("global socket not running\nRun '%s serve' first", meta.Name)
	}

	client, err := socket.NewClient(globalPath, socket.WithTimeout(60*time.Second))
	if err != nil {
		return fmt.Errorf("connect to global socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "backup.restore", map[string]any{
		"archive_path": archivePath,
	})
	if err != nil {
		return fmt.Errorf("backup.restore: %w", err)
	}

	var result backup.RestoreResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if restoreJSON {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal json: %w", err)
		}
		fmt.Println(string(data))

		return nil
	}

	if !cli.Quiet {
		fmt.Printf("Restored to: %s\n", result.Target)
		fmt.Printf("Files: %d, Directories: %d", result.Files, result.Dirs)
		if result.Skipped > 0 {
			fmt.Printf(", Skipped: %d", result.Skipped)
		}
		fmt.Println()
	}

	return nil
}
