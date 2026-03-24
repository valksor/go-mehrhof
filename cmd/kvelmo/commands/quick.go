package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/pkg/meta"
	"github.com/valksor/kvelmo/pkg/socket"
)

var (
	quickSource string
	quickText   string
	quickSkip   []string
)

var QuickCmd = &cobra.Command{
	Use:   "quick [description]",
	Short: "Quick-fix workflow: load, implement, and submit in one step",
	Long: `Start a task in quick mode, skipping planning and review phases.
Useful for trivial fixes where the full lifecycle is overkill.

  kvelmo quick --from github:owner/repo#123
  kvelmo quick --from file:task.md
  kvelmo quick --text "Fix typo in README"`,
	RunE: runQuick,
}

func init() {
	QuickCmd.Flags().StringVar(&quickSource, "from", "", "Task source")
	QuickCmd.Flags().StringVar(&quickText, "text", "", "Inline task description")
	QuickCmd.Flags().StringSliceVar(&quickSkip, "skip", nil, "Additional phases to skip (e.g., --skip simplify,optimize)")
}

func runQuick(_ *cobra.Command, args []string) error {
	if quickText == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		quickText = strings.TrimSpace(string(data))
	}
	// Accept positional arg as inline text: kvelmo quick "fix the typo"
	if quickText == "" && quickSource == "" && len(args) > 0 {
		quickText = strings.Join(args, " ")
	}
	if quickText != "" && quickSource != "" {
		return errors.New("--text and --from are mutually exclusive")
	}
	if quickText != "" {
		quickSource = "empty:" + quickText
	}
	if quickSource == "" {
		return errors.New("one of --from or --text is required, or pass description as argument")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	wtPath := socket.WorktreeSocketPath(cwd)

	if !socket.SocketExists(wtPath) {
		return fmt.Errorf("no worktree socket running for %s\nRun '"+meta.Name+" start' first", cwd)
	}

	client, err := socket.NewClient(wtPath, socket.WithTimeout(30*time.Second))
	if err != nil {
		return fmt.Errorf("connect to worktree socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	params := map[string]any{
		"source":       quickSource,
		"auto_advance": true,
		"skip_phases":  append([]string{"plan"}, quickSkip...),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.Call(ctx, "start", params)
	if err != nil {
		return fmt.Errorf("start quick task: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err == nil {
		if state, ok := result["state"].(string); ok {
			fmt.Printf("Quick mode: task loaded (state: %s)\n", state)
		} else {
			fmt.Println("Quick mode: task loaded")
		}
	} else {
		fmt.Println("Quick mode: task loaded")
	}

	fmt.Printf("Skipping plan phase, proceeding directly to implementation.\n")
	fmt.Printf("Use '%s watch' to monitor progress.\n", meta.Name)

	return nil
}
