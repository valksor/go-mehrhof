package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/agent/recorder"
	"github.com/valksor/kvelmo/internal/socket"
	"github.com/valksor/kvelmo/meta"
)

var recordingsDir string

// RecordingsCmd is the root command for managing agent recordings.
var RecordingsCmd = &cobra.Command{
	Use:   "recordings",
	Short: "Manage agent interaction recordings",
	Long: `View and manage recordings of agent interactions.

Recordings are JSONL files that capture all communication between
kvelmo and AI agents, useful for debugging and auditing.`,
}

var recordingsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all recordings",
	RunE:  runRecordingsList,
}

var recordingsViewCmd = &cobra.Command{
	Use:   "view <file>",
	Short: "View a recording file",
	Args:  cobra.ExactArgs(1),
	RunE:  runRecordingsView,
}

var recordingsReplayCmd = &cobra.Command{
	Use:   "replay <file>",
	Short: "Replay a recording with filtering",
	Args:  cobra.ExactArgs(1),
	RunE:  runRecordingsReplay,
}

var recordingsCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove old recordings",
	RunE:  runRecordingsClean,
}

var (
	recordingsJobFilter   string
	recordingsSinceFilter string
	recordingsTypeFilter  string
	recordingsOlderThan   string
	recordingsOutputJSON  bool
)

func init() {
	// Default recordings directory with robust fallback
	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		homeDir = os.Getenv("HOME")
		if homeDir == "" {
			homeDir = os.TempDir()
		}
	}
	defaultDir := filepath.Join(homeDir, ".valksor", "kvelmo", "recordings")

	RecordingsCmd.PersistentFlags().StringVar(&recordingsDir, "dir", defaultDir, "Recordings directory")

	recordingsListCmd.Flags().StringVar(&recordingsJobFilter, "job", "", "Filter by job ID")
	recordingsListCmd.Flags().StringVar(&recordingsSinceFilter, "since", "", "Show recordings since duration (e.g., 24h, 7d)")
	recordingsListCmd.Flags().BoolVar(&recordingsOutputJSON, "json", false, "Output as JSON")

	recordingsReplayCmd.Flags().StringVar(&recordingsTypeFilter, "filter", "", "Filter by event type (comma-separated)")

	recordingsCleanCmd.Flags().StringVar(&recordingsOlderThan, "older-than", "30d", "Remove recordings older than duration")

	RecordingsCmd.AddCommand(recordingsListCmd)
	RecordingsCmd.AddCommand(recordingsViewCmd)
	RecordingsCmd.AddCommand(recordingsReplayCmd)
	RecordingsCmd.AddCommand(recordingsCleanCmd)
}

func runRecordingsList(_ *cobra.Command, _ []string) error {
	globalPath := socket.GlobalSocketPath()
	if !socket.SocketExists(globalPath) {
		return fmt.Errorf("global socket not running\nRun '%s serve' first", meta.Name)
	}

	client, err := socket.NewClient(globalPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("connect to global socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	params := map[string]any{}
	if recordingsJobFilter != "" {
		params["job"] = recordingsJobFilter
	}
	if recordingsSinceFilter != "" {
		params["since"] = recordingsSinceFilter
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "recordings.list", params)
	if err != nil {
		return fmt.Errorf("recordings.list: %w", err)
	}

	var result struct {
		Recordings []recorder.RecordingInfo `json:"recordings"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	// Check for empty results
	if len(result.Recordings) == 0 {
		fmt.Println("No recordings found")

		return nil
	}

	if recordingsOutputJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")

		return enc.Encode(result.Recordings)
	}

	// Table output
	fmt.Printf("%-12s %-8s %-8s %-20s %s\n", "JOB", "AGENT", "LINES", "STARTED", "FILE")
	fmt.Println(strings.Repeat("-", 80))
	for _, info := range result.Recordings {
		// Truncate job ID for display
		jobDisplay := info.JobID
		if len(jobDisplay) > 12 {
			jobDisplay = jobDisplay[:12]
		}
		fmt.Printf("%-12s %-8s %-8d %-20s %s\n",
			jobDisplay,
			info.Agent,
			info.Lines,
			info.StartedAt,
			filepath.Base(info.Path),
		)
	}

	fmt.Printf("\nTotal: %d recording(s)\n", len(result.Recordings))

	return nil
}

func runRecordingsView(_ *cobra.Command, args []string) error {
	globalPath := socket.GlobalSocketPath()
	if !socket.SocketExists(globalPath) {
		return fmt.Errorf("global socket not running\nRun '%s serve' first", meta.Name)
	}

	client, err := socket.NewClient(globalPath, socket.WithTimeout(10*time.Second))
	if err != nil {
		return fmt.Errorf("connect to global socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	file := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "recordings.view", map[string]any{
		"file": file,
	})
	if err != nil {
		return fmt.Errorf("recordings.view: %w", err)
	}

	var result struct {
		Header  *recorder.Header  `json:"header"`
		Records []recorder.Record `json:"records"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	// Print header
	if h := result.Header; h != nil {
		fmt.Printf("Recording: %s\n", filepath.Base(file))
		fmt.Printf("Job: %s | Agent: %s | Model: %s\n", h.JobID, h.Agent, h.Model)
		fmt.Printf("Started: %s\n", h.StartedAt.Format(time.RFC3339))
		fmt.Println(strings.Repeat("-", 60))
	}

	// Print records
	for _, rec := range result.Records {
		direction := "\u2192"
		if rec.Direction == recorder.Inbound {
			direction = "\u2190"
		}

		fmt.Printf("[%s] %s %s: ", rec.Timestamp.Format("15:04:05.000"), direction, rec.Type)

		// Pretty print the event
		var prettyEvent any
		if err := json.Unmarshal(rec.Event, &prettyEvent); err == nil {
			//nolint:errchkjson // Re-marshaling unmarshaled JSON won't fail
			eventJSON, _ := json.MarshalIndent(prettyEvent, "    ", "  ")
			fmt.Printf("\n    %s\n", eventJSON)
		} else {
			fmt.Printf("%s\n", rec.Event)
		}
	}

	return nil
}

func runRecordingsReplay(_ *cobra.Command, args []string) error {
	path := args[0]

	// If not absolute, assume it's in the recordings dir
	if !filepath.IsAbs(path) {
		path = filepath.Join(recordingsDir, path)
	}

	records, err := recorder.ReadAll(path)
	if err != nil {
		return fmt.Errorf("read recording: %w", err)
	}

	// Apply filter
	if recordingsTypeFilter != "" {
		types := strings.Split(recordingsTypeFilter, ",")
		for i := range types {
			types[i] = strings.TrimSpace(types[i])
		}
		records = recorder.FilterRecords(records, recorder.Filter{Types: types})
	}

	// Output filtered records
	for _, rec := range records {
		direction := "OUT"
		if rec.Direction == recorder.Inbound {
			direction = "IN "
		}

		fmt.Printf("%s [%s] %s: ", rec.Timestamp.Format("15:04:05.000"), direction, rec.Type)

		// Compact JSON output
		var compactEvent any
		if err := json.Unmarshal(rec.Event, &compactEvent); err == nil {
			//nolint:errchkjson // Re-marshaling unmarshaled JSON won't fail
			eventJSON, _ := json.Marshal(compactEvent)
			// Truncate long lines (rune-safe to preserve UTF-8)
			line := string(eventJSON)
			runes := []rune(line)
			if len(runes) > 120 {
				line = string(runes[:117]) + "..."
			}
			fmt.Println(line)
		} else {
			fmt.Printf("%s\n", rec.Event)
		}
	}

	fmt.Printf("\nTotal: %d record(s)\n", len(records))

	return nil
}

func runRecordingsClean(_ *cobra.Command, _ []string) error {
	duration, err := parseDuration(recordingsOlderThan)
	if err != nil {
		return fmt.Errorf("invalid --older-than duration: %w", err)
	}

	cutoff := time.Now().Add(-duration).Unix()
	removed, err := recorder.CleanOldRecordings(recordingsDir, cutoff)
	if err != nil {
		return fmt.Errorf("clean recordings: %w", err)
	}

	if removed == 0 {
		fmt.Println("No recordings to clean")
	} else {
		fmt.Printf("Removed %d recording(s)\n", removed)
	}

	return nil
}

// parseDuration parses duration strings like "24h", "7d", "30d".
// Returns an error if the duration is zero or negative.
func parseDuration(s string) (time.Duration, error) {
	var d time.Duration
	var err error

	// Handle day suffix
	if rest, found := strings.CutSuffix(s, "d"); found {
		days, err := strconv.Atoi(rest)
		if err != nil {
			return 0, err
		}
		d = time.Duration(days) * 24 * time.Hour
	} else {
		// Standard duration
		d, err = time.ParseDuration(s)
		if err != nil {
			return 0, err
		}
	}

	if d <= 0 {
		return 0, fmt.Errorf("duration must be positive, got %v", d)
	}

	return d, nil
}
