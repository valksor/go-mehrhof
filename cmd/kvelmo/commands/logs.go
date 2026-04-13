package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/valksor/kvelmo/internal/conductor"
	"github.com/valksor/kvelmo/internal/socket"
	"github.com/valksor/kvelmo/meta"
)

var LogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show agent activity for the current task",
	Long:  `Display timestamped agent activity log for the active task. This shows the same chat history as 'chat history' but formatted as an activity log with prominent timestamps and truncated messages.`,
	RunE:  runLogs,
}

func init() {
	LogsCmd.Flags().IntP("limit", "n", 50, "Number of messages to show")
	LogsCmd.Flags().Bool("full", false, "Show full message content without truncation")
	LogsCmd.Flags().Bool("json", false, "Output raw JSON")
	LogsCmd.Flags().BoolP("follow", "f", false, "Follow live output after showing history")
}

func runLogs(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	globalPath := socket.GlobalSocketPath()

	if !socket.SocketExists(globalPath) {
		return errors.New("no global socket running\nRun '" + meta.Name + " serve' first")
	}

	client, err := socket.NewClient(globalPath, socket.WithTimeout(10*time.Second))
	if err != nil {
		return fmt.Errorf("connect to global socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	wtPath := socket.WorktreeSocketPath(cwd)
	worktreeID := wtPath

	params := map[string]any{
		"worktree_id": worktreeID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "chat.history", params)
	if err != nil {
		return fmt.Errorf("chat.history call: %w", err)
	}

	jsonFlag, _ := cmd.Flags().GetBool("json")
	if jsonFlag {
		return outputJSON(resp.Result)
	}

	var result struct {
		Messages []struct {
			ID        string   `json:"id"`
			Role      string   `json:"role"`
			Content   string   `json:"content"`
			Mentions  []string `json:"mentions,omitempty"`
			Timestamp string   `json:"timestamp,omitempty"`
			JobID     string   `json:"job_id,omitempty"`
		} `json:"messages"`
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if len(result.Messages) == 0 {
		fmt.Println("No activity logs for this task")

		return nil
	}

	limit, _ := cmd.Flags().GetInt("limit")
	showFull, _ := cmd.Flags().GetBool("full")

	messages := result.Messages
	if limit > 0 && limit < len(messages) {
		messages = messages[len(messages)-limit:]
	}

	fmt.Printf("Activity log (%d messages, showing %d)\n", len(result.Messages), len(messages))
	fmt.Println(strings.Repeat("=", 60))

	for _, msg := range messages {
		roleLabel := "USER"
		switch msg.Role {
		case "assistant":
			roleLabel = "AGENT"
		case "system":
			roleLabel = "SYSTEM"
		}

		timestamp := "          "
		if msg.Timestamp != "" {
			if t, err := time.Parse(time.RFC3339, msg.Timestamp); err == nil {
				timestamp = t.Format("15:04:05")
			}
		}

		fmt.Printf("%s  %-6s  ", timestamp, roleLabel)

		content := strings.TrimSpace(msg.Content)
		if !showFull && len(content) > 200 {
			content = content[:200] + " [truncated, use --full to see all]"
		}

		// Replace newlines with indented continuation for compact display
		content = strings.ReplaceAll(content, "\n", "\n                  ")
		fmt.Printf("%s\n", content)
	}

	follow, _ := cmd.Flags().GetBool("follow")
	if !follow {
		return nil
	}

	return followLogs(cmd, cwd, showFull)
}

// logsStreamEventTypes are the event types displayed when following logs.
var logsStreamEventTypes = []string{
	"stream",
	"assistant",
	"job_output",
	"job_started",
	"job_completed",
	"job_failed",
}

// followLogs connects to the worktree socket's event stream and tails new
// events, displaying them in the same format as the history output.
func followLogs(_ *cobra.Command, cwd string, showFull bool) error {
	wtPath := socket.WorktreeSocketPath(cwd)
	if !socket.SocketExists(wtPath) {
		return fmt.Errorf("no worktree socket running for %s\nRun '%s start' first", cwd, meta.Name)
	}

	var d net.Dialer
	conn, err := d.DialContext(context.Background(), "unix", wtPath)
	if err != nil {
		return fmt.Errorf("connect to worktree socket: %w", err)
	}
	defer func() { _ = conn.Close() }()

	req := socket.Request{
		JSONRPC: "2.0",
		ID:      "logs-follow-1",
		Method:  "stream.subscribe",
		Params:  json.RawMessage(`{"last_seq":0}`),
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	reqBytes = append(reqBytes, '\n')
	if _, err := conn.Write(reqBytes); err != nil {
		return fmt.Errorf("send subscribe: %w", err)
	}

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		if scanErr := scanner.Err(); scanErr != nil {
			return fmt.Errorf("read response: %w", scanErr)
		}

		return errors.New("connection closed before response")
	}

	var resp socket.Response
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("subscribe failed: %s", resp.Error.Message)
	}

	fmt.Println(strings.Repeat("-", 60))
	fmt.Println("Following live output (Ctrl+C to stop)...")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		_ = conn.Close()
	}()

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event conductor.ConductorEvent
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}

		if !slices.Contains(logsStreamEventTypes, event.Type) {
			continue
		}

		timestamp := event.Timestamp.Format("15:04:05")
		label := strings.ToUpper(event.Type)

		content := strings.TrimSpace(event.Message)
		if content == "" {
			continue
		}

		if !showFull && len(content) > 200 {
			content = content[:200] + " [truncated, use --full to see all]"
		}
		content = strings.ReplaceAll(content, "\n", "\n                  ")

		fmt.Printf("%s  %-6s  %s\n", timestamp, label, content)
	}

	if err := scanner.Err(); err != nil {
		if isClosedConnErr(err) {
			return nil
		}

		return fmt.Errorf("read stream: %w", err)
	}

	return nil
}
