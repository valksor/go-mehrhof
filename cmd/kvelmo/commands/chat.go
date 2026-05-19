package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/internal/socket"
	"github.com/valksor/kvelmo/meta"
)

var ChatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Chat with the agent",
}

var chatSendCmd = &cobra.Command{
	Use:   "send [message]",
	Short: "Send a message to the agent",
	Long: `Send a message to the agent working on the current task.
Supports @filename mentions to include file contents.`,
	Args: cobra.ExactArgs(1),
	RunE: runChatSend,
}

var chatStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the current chat/job",
	Long:  `Stop the current chat or job while keeping the worker available.`,
	RunE:  runChatStop,
}

var chatHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "Show chat history for the current task",
	Long:  `Display the chat history for the active task in this project.`,
	RunE:  runChatHistory,
}

var chatClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear chat history for the current task",
	Long:  `Clear all chat messages for the active task. This cannot be undone.`,
	RunE:  runChatClear,
}

func init() {
	ChatCmd.Long = fmt.Sprintf(`Chat commands for interacting with the agent.

Examples:
  %[1]s chat send "add error handling to the login function"
  %[1]s chat send "check @src/main.go for issues"
  %[1]s chat stop`, meta.Name)
	ChatCmd.AddCommand(chatSendCmd)
	ChatCmd.AddCommand(chatStopCmd)
	ChatCmd.AddCommand(chatHistoryCmd)
	ChatCmd.AddCommand(chatClearCmd)

	chatSendCmd.Flags().Bool("answer", false, "Mark message as an answer to agent question")
	chatStopCmd.Flags().String("job", "", "Specific job ID to stop (optional)")
	chatHistoryCmd.Flags().IntP("limit", "n", 0, "Limit number of messages to show (0 = all)")
}

func runChatSend(cmd *cobra.Command, args []string) error {
	message := args[0]

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	globalPath := socket.GlobalSocketPath()

	if !socket.SocketExists(globalPath) {
		return errors.New("no global socket running\nRun '" + meta.Name + " serve' first")
	}

	client, err := socket.NewClient(globalPath, socket.WithTimeout(30*time.Second))
	if err != nil {
		return fmt.Errorf("connect to global socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	isAnswer, _ := cmd.Flags().GetBool("answer")

	wtPath := socket.WorktreeSocketPath(cwd)
	worktreeID := wtPath

	params := map[string]any{
		"message":       message,
		paramWorktreeID: worktreeID,
		"is_answer":     isAnswer,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "chat.send", params)
	if err != nil {
		return fmt.Errorf("chat.send call: %w", err)
	}

	var result struct {
		JobID  string `json:"job_id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	fmt.Printf("Message sent (job: %s)\n", result.JobID)
	fmt.Println("Use '" + meta.Name + " status' to check progress")

	return nil
}

func runChatStop(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	globalPath := socket.GlobalSocketPath()

	if !socket.SocketExists(globalPath) {
		return errors.New("no global socket running\nRun '" + meta.Name + " serve' first")
	}

	client, err := socket.NewClient(globalPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("connect to global socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	wtPath := socket.WorktreeSocketPath(cwd)
	worktreeID := wtPath

	jobID, _ := cmd.Flags().GetString("job")

	params := map[string]any{
		paramWorktreeID: worktreeID,
	}
	if jobID != "" {
		params["job_id"] = jobID
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "chat.stop", params)
	if err != nil {
		return fmt.Errorf("chat.stop call: %w", err)
	}

	var result struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	fmt.Printf("%s\n", result.Message)

	return nil
}

func runChatHistory(cmd *cobra.Command, args []string) error {
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
		paramWorktreeID: worktreeID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "chat.history", params)
	if err != nil {
		return fmt.Errorf("chat.history call: %w", err)
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
		fmt.Println("No chat history for this task")

		return nil
	}

	limit, _ := cmd.Flags().GetInt("limit")
	messages := result.Messages
	if limit > 0 && limit < len(messages) {
		messages = messages[len(messages)-limit:]
	}

	fmt.Printf("Chat history (%d messages)\n", len(result.Messages))
	fmt.Println(strings.Repeat("-", 40))
	for _, msg := range messages {
		roleLabel := "User"
		switch msg.Role {
		case "assistant":
			roleLabel = "Assistant"
		case "system":
			roleLabel = "System"
		}

		timestamp := ""
		if msg.Timestamp != "" {
			if t, err := time.Parse(time.RFC3339, msg.Timestamp); err == nil {
				timestamp = t.Format("15:04:05")
			}
		}

		if timestamp != "" {
			fmt.Printf("[%s] %s:\n", timestamp, roleLabel)
		} else {
			fmt.Printf("%s:\n", roleLabel)
		}

		// Truncate long messages in display
		content := msg.Content
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		fmt.Printf("%s\n\n", content)
	}

	return nil
}

func runChatClear(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	globalPath := socket.GlobalSocketPath()

	if !socket.SocketExists(globalPath) {
		return errors.New("no global socket running\nRun '" + meta.Name + " serve' first")
	}

	client, err := socket.NewClient(globalPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("connect to global socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	wtPath := socket.WorktreeSocketPath(cwd)
	worktreeID := wtPath

	params := map[string]any{
		paramWorktreeID: worktreeID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "chat.clear", params)
	if err != nil {
		return fmt.Errorf("chat.clear call: %w", err)
	}

	var result struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	fmt.Printf("%s\n", result.Message)

	return nil
}
