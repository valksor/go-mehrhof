package commands

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/internal/socket"
	"github.com/valksor/kvelmo/meta"
)

var (
	exportFormat  string
	exportSince   string
	exportInclude string
)

var ExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export task history and metrics",
	Long:  "Export kvelmo data in JSON or CSV format for external analysis.",
	RunE:  runExport,
}

func init() {
	ExportCmd.Flags().StringVar(&exportFormat, "format", "json", "Output format (json, csv)")
	ExportCmd.Flags().StringVar(&exportSince, "since", "", "Time range (e.g., 7d, 30d)")
	ExportCmd.Flags().StringVar(&exportInclude, "include", "tasks,metrics", "Data to include (comma-separated: tasks,metrics,activity)")
}

func runExport(_ *cobra.Command, _ []string) error {
	globalPath := socket.GlobalSocketPath()
	if !socket.SocketExists(globalPath) {
		return errors.New("global socket not running\nRun '" + meta.Name + " serve' first")
	}

	client, err := socket.NewClient(globalPath, socket.WithTimeout(30*time.Second))
	if err != nil {
		return fmt.Errorf("connect to global socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "export", map[string]any{
		"format":  exportFormat,
		"since":   exportSince,
		"include": exportInclude,
	})
	if err != nil {
		return fmt.Errorf("export: %w", err)
	}

	if exportFormat == "csv" {
		return outputCSV(resp.Result)
	}

	out, jsonErr := json.MarshalIndent(resp.Result, "", "  ")
	if jsonErr != nil {
		fmt.Println(string(resp.Result))
	} else {
		fmt.Println(string(out))
	}

	return nil
}

func outputCSV(data json.RawMessage) error {
	var result struct {
		Tasks []struct {
			ID    string `json:"id"`
			Path  string `json:"path"`
			State string `json:"state"`
		} `json:"tasks"`
		Activity []struct {
			Timestamp  string `json:"timestamp"`
			Method     string `json:"method"`
			DurationMs int64  `json:"duration_ms"`
			Error      string `json:"error"`
			UserID     string `json:"user_id"`
			TaskID     string `json:"task_id"`
			AgentModel string `json:"agent_model"`
		} `json:"activity"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("parse export data: %w", err)
	}

	w := csv.NewWriter(os.Stdout)
	defer w.Flush()

	if len(result.Tasks) > 0 {
		_ = w.Write([]string{"# Tasks"})
		_ = w.Write([]string{"id", "path", "state"})
		for _, t := range result.Tasks {
			_ = w.Write([]string{t.ID, t.Path, t.State})
		}
		_ = w.Write([]string{})
	}

	if len(result.Activity) > 0 {
		_ = w.Write([]string{"# Activity"})
		_ = w.Write([]string{"timestamp", "method", "duration_ms", "error", "user_id", "task_id", "agent_model"})
		for _, a := range result.Activity {
			_ = w.Write([]string{
				a.Timestamp,
				a.Method,
				strconv.FormatInt(a.DurationMs, 10),
				a.Error,
				a.UserID,
				a.TaskID,
				a.AgentModel,
			})
		}
	}

	return nil
}
