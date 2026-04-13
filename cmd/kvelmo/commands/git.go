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

var GitCmd = &cobra.Command{
	Use:   "git",
	Short: "Git operations",
	Long:  `Git operations for the current worktree.`,
}

var gitStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show git status",
	Long:  `Show the current git status including branch and changed files.`,
	RunE:  runGitStatus,
}

var gitDiffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show git diff",
	Long:  `Show the diff of uncommitted changes.`,
	RunE:  runGitDiff,
}

var gitLogCmd = &cobra.Command{
	Use:   "log",
	Short: "Show git log",
	Long:  `Show recent git commits.`,
	RunE:  runGitLog,
}

func init() {
	GitCmd.AddCommand(gitStatusCmd)
	GitCmd.AddCommand(gitDiffCmd)
	GitCmd.AddCommand(gitLogCmd)

	gitDiffCmd.Flags().Bool("cached", false, "Show staged changes only")
	gitLogCmd.Flags().IntP("count", "n", 10, "Number of commits to show")
}

func runGitStatus(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	wtPath := socket.WorktreeSocketPath(cwd)

	if !socket.SocketExists(wtPath) {
		return errors.New("no worktree socket running\nRun '" + meta.Name + " start' first")
	}

	client, err := socket.NewClient(wtPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("connect to worktree socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "git.status", nil)
	if err != nil {
		return fmt.Errorf("git.status call: %w", err)
	}

	var result struct {
		Branch     string   `json:"branch"`
		HasChanges bool     `json:"has_changes"`
		Files      []string `json:"files"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	fmt.Printf("Branch: %s\n", result.Branch)
	if result.HasChanges {
		fmt.Printf("\nChanged files (%d):\n", len(result.Files))
		for _, f := range result.Files {
			fmt.Printf("  %s\n", f)
		}
	} else {
		fmt.Println("No changes")
	}

	return nil
}

func runGitDiff(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	wtPath := socket.WorktreeSocketPath(cwd)

	if !socket.SocketExists(wtPath) {
		return errors.New("no worktree socket running\nRun '" + meta.Name + " start' first")
	}

	client, err := socket.NewClient(wtPath, socket.WithTimeout(10*time.Second))
	if err != nil {
		return fmt.Errorf("connect to worktree socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	cached, _ := cmd.Flags().GetBool("cached")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "git.diff", map[string]any{"cached": cached})
	if err != nil {
		return fmt.Errorf("git.diff call: %w", err)
	}

	var result struct {
		Diff string `json:"diff"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if result.Diff == "" {
		fmt.Println("No changes")
	} else {
		fmt.Print(result.Diff)
	}

	return nil
}

func runGitLog(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	wtPath := socket.WorktreeSocketPath(cwd)

	if !socket.SocketExists(wtPath) {
		return errors.New("no worktree socket running\nRun '" + meta.Name + " start' first")
	}

	client, err := socket.NewClient(wtPath, socket.WithTimeout(5*time.Second))
	if err != nil {
		return fmt.Errorf("connect to worktree socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	count, _ := cmd.Flags().GetInt("count")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "git.log", map[string]any{"count": count})
	if err != nil {
		return fmt.Errorf("git.log call: %w", err)
	}

	var result struct {
		Entries []struct {
			SHA     string `json:"sha"`
			Message string `json:"message"`
			Author  string `json:"author"`
			Date    string `json:"date"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if len(result.Entries) == 0 {
		fmt.Println("No commits")

		return nil
	}

	for _, entry := range result.Entries {
		fmt.Printf("%s %s\n", entry.SHA[:8], entry.Message)
	}

	return nil
}
