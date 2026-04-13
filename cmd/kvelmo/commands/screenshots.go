package commands

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/internal/socket"
	"github.com/valksor/kvelmo/meta"
)

var ScreenshotsCmd = &cobra.Command{
	Use:   "screenshots",
	Short: "Manage screenshots",
	Long:  `List and manage screenshots captured during task execution.`,
}

var screenshotsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List screenshots",
	Long:  `List all screenshots for the current task.`,
	RunE:  runScreenshotsList,
}

var screenshotsDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete a screenshot",
	Long:  `Delete a screenshot by its ID.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runScreenshotsDelete,
}

var screenshotsCaptureCmd = &cobra.Command{
	Use:   "capture",
	Short: "Capture a screenshot",
	Long:  `Capture a screenshot from the browser or system.`,
	RunE:  runScreenshotsCapture,
}

var screenshotsGetOutput string

var screenshotsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a screenshot by ID",
	Long:  `Get screenshot metadata by ID. Use --output to save the image to a file.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runScreenshotsGet,
}

func init() {
	screenshotsGetCmd.Flags().StringVarP(&screenshotsGetOutput, "output", "o", "", "Save image to file path")
	screenshotsCaptureCmd.Flags().String("source", "browser", "Screenshot source (browser or system)")
	screenshotsCaptureCmd.Flags().String("step", "", "Label for this capture step")
	ScreenshotsCmd.AddCommand(screenshotsListCmd)
	ScreenshotsCmd.AddCommand(screenshotsDeleteCmd)
	ScreenshotsCmd.AddCommand(screenshotsGetCmd)
	ScreenshotsCmd.AddCommand(screenshotsCaptureCmd)
}

type Screenshot struct {
	ID        string `json:"id"`
	TaskID    string `json:"task_id"`
	Path      string `json:"path"`
	Filename  string `json:"filename"`
	Timestamp string `json:"timestamp"`
	Source    string `json:"source"`
	Step      string `json:"step,omitempty"`
	Agent     string `json:"agent,omitempty"`
	Format    string `json:"format"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	SizeBytes int    `json:"size_bytes"`
}

func runScreenshotsList(cmd *cobra.Command, args []string) error {
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

	resp, err := client.Call(ctx, "screenshots.list", nil)
	if err != nil {
		return fmt.Errorf("screenshots.list call: %w", err)
	}

	var result struct {
		Screenshots []Screenshot `json:"screenshots"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if len(result.Screenshots) == 0 {
		fmt.Println("No screenshots")

		return nil
	}

	fmt.Printf("Screenshots (%d):\n\n", len(result.Screenshots))
	for _, s := range result.Screenshots {
		fmt.Printf("  ID:       %s\n", s.ID)
		fmt.Printf("  File:     %s\n", s.Filename)
		fmt.Printf("  Size:     %dx%d (%d bytes)\n", s.Width, s.Height, s.SizeBytes)
		fmt.Printf("  Source:   %s\n", s.Source)
		if s.Step != "" {
			fmt.Printf("  Step:     %s\n", s.Step)
		}
		fmt.Printf("  Time:     %s\n", s.Timestamp)
		fmt.Println()
	}

	return nil
}

func runScreenshotsDelete(cmd *cobra.Command, args []string) error {
	id := args[0]

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

	resp, err := client.Call(ctx, "screenshots.delete", map[string]any{"screenshot_id": id})
	if err != nil {
		return fmt.Errorf("screenshots.delete call: %w", err)
	}

	var result struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if result.Success {
		fmt.Printf("Screenshot %s deleted\n", id)
	} else {
		fmt.Printf("Failed to delete screenshot %s\n", id)
	}

	return nil
}

func runScreenshotsCapture(cmd *cobra.Command, _ []string) error {
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

	source, _ := cmd.Flags().GetString("source")
	step, _ := cmd.Flags().GetString("step")

	params := map[string]any{"source": source}
	if step != "" {
		params["step"] = step
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "screenshots.capture", params)
	if err != nil {
		return fmt.Errorf("screenshots.capture call: %w", err)
	}

	var result struct {
		Screenshot Screenshot `json:"screenshot"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	s := result.Screenshot
	fmt.Printf("Screenshot captured: %s\n", s.ID)
	fmt.Printf("  File:     %s\n", s.Filename)
	fmt.Printf("  Size:     %dx%d (%d bytes)\n", s.Width, s.Height, s.SizeBytes)
	fmt.Printf("  Source:   %s\n", s.Source)

	return nil
}

func runScreenshotsGet(cmd *cobra.Command, args []string) error {
	id := args[0]

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

	resp, err := client.Call(ctx, "screenshots.get", map[string]any{"screenshot_id": id})
	if err != nil {
		return fmt.Errorf("screenshots.get call: %w", err)
	}

	var result struct {
		Screenshot Screenshot `json:"screenshot"`
		Data       string     `json:"data,omitempty"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	s := result.Screenshot
	fmt.Printf("ID:       %s\n", s.ID)
	fmt.Printf("File:     %s\n", s.Filename)
	fmt.Printf("Size:     %dx%d (%d bytes)\n", s.Width, s.Height, s.SizeBytes)
	fmt.Printf("Format:   %s\n", s.Format)
	fmt.Printf("Source:   %s\n", s.Source)
	if s.Step != "" {
		fmt.Printf("Step:     %s\n", s.Step)
	}
	fmt.Printf("Time:     %s\n", s.Timestamp)

	if screenshotsGetOutput != "" && result.Data != "" {
		decoded, decErr := base64.StdEncoding.DecodeString(result.Data)
		if decErr != nil {
			return fmt.Errorf("decode image data: %w", decErr)
		}
		if writeErr := os.WriteFile(screenshotsGetOutput, decoded, 0o644); writeErr != nil {
			return fmt.Errorf("write output file: %w", writeErr)
		}
		fmt.Printf("Saved to: %s\n", screenshotsGetOutput)
	}

	return nil
}
