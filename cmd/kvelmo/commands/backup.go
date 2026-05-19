package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/valksor/kvelmo/internal/backup"
	"github.com/valksor/kvelmo/internal/cli"
	"github.com/valksor/kvelmo/internal/socket"
	"github.com/valksor/kvelmo/meta"
)

var backupJSON bool

var BackupCmd = &cobra.Command{
	Use:   "backup [output-path]",
	Short: "Backup kvelmo state to a tar.gz archive",
	Long: `Create a compressed archive of all kvelmo state.

Includes configuration, task data, recordings, and memory.
Excludes transient files (sockets, locks).

Subcommands:
  list    List existing backup archives

Examples:
  kvelmo backup                        # Default: kvelmo-backup-<timestamp>.tar.gz
  kvelmo backup /tmp/my-backup.tar.gz  # Custom output path
  kvelmo backup list                   # List existing backups
  kvelmo backup list --json            # List as JSON`,
	Args: cobra.MaximumNArgs(1),
	RunE: runBackup,
}

var backupListCmd = &cobra.Command{
	Use:   subList,
	Short: "List existing backup archives",
	Long:  "Query the global socket for existing backup archives, showing timestamp, path, and size.",
	RunE:  runBackupList,
}

var backupListJSON bool

func init() {
	BackupCmd.Flags().BoolVar(&backupJSON, "json", false, "Output as JSON")

	backupListCmd.Flags().BoolVar(&backupListJSON, "json", false, "Output as JSON")
	BackupCmd.AddCommand(backupListCmd)
}

func runBackup(_ *cobra.Command, args []string) error {
	globalPath := socket.GlobalSocketPath()
	if !socket.SocketExists(globalPath) {
		return fmt.Errorf("global socket not running\nRun '%s serve' first", meta.Name)
	}

	client, err := socket.NewClient(globalPath, socket.WithTimeout(30*time.Second))
	if err != nil {
		return fmt.Errorf("connect to global socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	params := map[string]any{}
	if len(args) > 0 {
		params["output_path"] = args[0]
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "backup.create", params)
	if err != nil {
		return fmt.Errorf("backup.create: %w", err)
	}

	var result backup.Result
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if backupJSON {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal json: %w", err)
		}
		fmt.Println(string(data))

		return nil
	}

	if !cli.Quiet {
		fmt.Printf("Backup created: %s\n", result.Path)
		fmt.Printf("Size: %s (%d files)\n", backup.FormatBytes(result.Size), result.Files)
	}

	return nil
}

func runBackupList(_ *cobra.Command, _ []string) error {
	globalPath := socket.GlobalSocketPath()

	if !socket.SocketExists(globalPath) {
		return fmt.Errorf("global socket not running\nRun '%s serve' first", meta.Name)
	}

	client, err := socket.NewClient(globalPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("connect to global socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "backup.list", nil)
	if err != nil {
		return fmt.Errorf("backup.list: %w", err)
	}

	var result struct {
		Backups []backup.BackupInfo `json:"backups"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if backupListJSON {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal json: %w", err)
		}
		fmt.Println(string(data))

		return nil
	}

	if len(result.Backups) == 0 {
		fmt.Println("No backups found")

		return nil
	}

	fmt.Printf("%-24s  %-10s  %s\n", "Created", "Size", "Path")
	fmt.Printf("%-24s  %-10s  %s\n",
		strings.Repeat("-", 24), strings.Repeat("-", 10), strings.Repeat("-", 40))
	for _, b := range result.Backups {
		fmt.Printf("%-24s  %-10s  %s\n", b.CreatedAt, backup.FormatBytes(b.Size), b.Path)
	}

	fmt.Printf("\n%d backup(s)\n", len(result.Backups))

	return nil
}
