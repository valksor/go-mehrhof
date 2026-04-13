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
	"github.com/valksor/kvelmo/meta"
)

const defaultExplainPrompt = "Explain what you did in the last action, why you made those choices, and any assumptions or constraints you encountered."

var ExplainCmd = &cobra.Command{
	Use:   "explain",
	Short: "Ask the agent to explain its last action",
	Long: `Send a message asking the agent to explain what it did, why,
and any assumptions or constraints it encountered.

Use --prompt to override the default explanation prompt.`,
	RunE: runExplain,
}

func init() {
	ExplainCmd.Flags().StringP("prompt", "p", "", "Custom prompt to override the default explanation request")
	ExplainCmd.Flags().BoolVarP(&explainWait, "wait", "w", false, "Wait for job to complete, streaming output")
}

var explainWait bool

func runExplain(cmd *cobra.Command, args []string) error {
	prompt, _ := cmd.Flags().GetString("prompt")
	if prompt == "" {
		prompt = defaultExplainPrompt
	}

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

	wtPath := socket.WorktreeSocketPath(cwd)
	worktreeID := wtPath

	params := map[string]any{
		"message":     prompt,
		"worktree_id": worktreeID,
		"is_answer":   false,
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

	fmt.Printf("Explain request sent (job: %s)\n", result.JobID)

	if explainWait {
		return waitForJob(wtPath, result.JobID)
	}

	fmt.Println("Use '" + meta.Name + " status' to check progress")

	return nil
}
