package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/internal/socket"
	"github.com/valksor/kvelmo/meta"
)

var FilesCmd = &cobra.Command{
	Use:   "files",
	Short: "Search and list files",
	Long:  `Search and list files in the current project.`,
}

var (
	filesSearchMax  int
	filesSearchJSON bool
	filesListExt    []string
	filesListDepth  int
	filesListJSON   bool
)

var filesSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search files by name or path",
	Args:  cobra.ExactArgs(1),
	RunE:  runFilesSearch,
}

var filesListCmd = &cobra.Command{
	Use:   "list [path]",
	Short: "List files under a path",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runFilesList,
}

func init() {
	filesSearchCmd.Flags().IntVar(&filesSearchMax, "max", 10, "Maximum number of results")
	filesSearchCmd.Flags().BoolVar(&filesSearchJSON, "json", false, "Output raw JSON response")
	filesListCmd.Flags().StringSliceVar(&filesListExt, "ext", nil, "Filter by file extensions (e.g. --ext go,ts)")
	filesListCmd.Flags().IntVar(&filesListDepth, "depth", 0, "Maximum directory depth (0 = unlimited)")
	filesListCmd.Flags().BoolVar(&filesListJSON, "json", false, "Output raw JSON response")
	FilesCmd.AddCommand(filesSearchCmd)
	FilesCmd.AddCommand(filesListCmd)
}

func runFilesSearch(cmd *cobra.Command, args []string) error {
	query := args[0]

	client, ctx, cancel, err := globalSocketClient()
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	defer cancel()

	params := map[string]any{
		"query":       query,
		"max_results": filesSearchMax,
	}

	resp, err := client.Call(ctx, "files.search", params)
	if err != nil {
		return fmt.Errorf("files.search call: %w", err)
	}

	if filesSearchJSON {
		return outputJSON(resp.Result)
	}

	var result struct {
		Files []string `json:"files"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if len(result.Files) == 0 {
		fmt.Println("No files found")

		return nil
	}

	for _, f := range result.Files {
		fmt.Println(f)
	}

	return nil
}

func runFilesList(cmd *cobra.Command, args []string) error {
	gPath := socket.GlobalSocketPath()
	if !socket.SocketExists(gPath) {
		return errors.New(meta.Name + " server not running\nRun '" + meta.Name + " serve' first")
	}

	client, err := socket.NewClient(gPath, socket.WithTimeout(30*time.Second))
	if err != nil {
		return fmt.Errorf("connect to server: %w", err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	params := map[string]any{}
	if len(args) > 0 {
		params["path"] = args[0]
	}
	if len(filesListExt) > 0 {
		params["extensions"] = filesListExt
	}
	if filesListDepth > 0 {
		params["max_depth"] = filesListDepth
	}

	resp, err := client.Call(ctx, "files.list", params)
	if err != nil {
		return fmt.Errorf("files.list call: %w", err)
	}

	if filesListJSON {
		return outputJSON(resp.Result)
	}

	var result struct {
		Files []string `json:"files"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if len(result.Files) == 0 {
		fmt.Println("No files found")

		return nil
	}

	for _, f := range result.Files {
		fmt.Println(f)
	}

	return nil
}
