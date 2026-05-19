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
	"github.com/valksor/kvelmo/internal/conductor"
	"github.com/valksor/kvelmo/internal/socket"
	"github.com/valksor/kvelmo/meta"
)

var (
	quickSource        string
	quickText          string
	quickSkip          []string
	quickContextFiles  []string
	quickContextSymbol []string
	quickContextCommit []string
)

var QuickCmd = &cobra.Command{
	Use:   "quick [description]",
	Short: "Quick-fix workflow: load and implement, skipping planning",
	Long: `Start a task in quick mode, skipping the planning phase and auto-advancing
through implement, simplify, optimize, and review.

Use --skip to skip additional phases (e.g., --skip simplify,optimize).

  kvelmo quick "Fix typo in README"
  kvelmo quick --from github:owner/repo#123
  kvelmo quick --from file:task.md
  kvelmo quick --skip simplify,optimize`,
	RunE: runQuick,
}

func init() {
	QuickCmd.Flags().StringVar(&quickSource, "from", "", "Task source")
	QuickCmd.Flags().StringVar(&quickText, "text", "", "Inline task description")
	QuickCmd.Flags().StringSliceVar(&quickSkip, "skip", nil, "Additional phases to skip (e.g., --skip simplify,optimize)")
	QuickCmd.Flags().StringSliceVar(&quickContextFiles, "file", nil, "Attach file context (e.g., --file src/main.go)")
	QuickCmd.Flags().StringSliceVar(&quickContextSymbol, "symbol", nil, "Attach symbol context (e.g., --symbol HandleRequest)")
	QuickCmd.Flags().StringSliceVar(&quickContextCommit, "commit", nil, "Attach commit context (e.g., --commit abc123)")
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
		paramSource:    quickSource,
		"auto_advance": true,
		"skip_phases":  append([]string{phasePlan}, quickSkip...),
	}

	// Build and validate context items
	items := buildQuickContextItems()
	if err := validateContextItems(items); err != nil {
		return err
	}
	if len(items) > 0 {
		params["context_items"] = items
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

	fmt.Printf("Skipping plan phase — auto-advancing through remaining phases.\n")
	fmt.Printf("Use '%s watch' to monitor progress.\n", meta.Name)

	return nil
}

func buildQuickContextItems() []conductor.ContextItem {
	return buildContextItemsFromFlags(quickContextFiles, quickContextSymbol, quickContextCommit)
}
