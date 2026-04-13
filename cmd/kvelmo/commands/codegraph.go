package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/internal/cli"
)

var CodegraphCmd = &cobra.Command{
	Use:   "codegraph",
	Short: "Code symbol graph",
	Long:  "Index, search, and explore code symbols and their relationships.",
}

var codegraphStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show code graph statistics",
	RunE:  runCodegraphStats,
}

var codegraphIndexCmd = &cobra.Command{
	Use:   "index [path]",
	Short: "Index code symbols in a directory",
	Long:  "Parse Go source files and index symbols and relationships into the code graph. Defaults to the project root.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runCodegraphIndex,
}

var codegraphSearchCmd = &cobra.Command{
	Use:   "search <name>",
	Short: "Find symbol definitions",
	Long:  "Search for symbols by exact name, or use --pattern with SQL LIKE wildcards (%).",
	Args:  cobra.ExactArgs(1),
	RunE:  runCodegraphSearch,
}

var codegraphCallersCmd = &cobra.Command{
	Use:   "callers <function>",
	Short: "Find callers of a function",
	Args:  cobra.ExactArgs(1),
	RunE:  runCodegraphCallers,
}

var codegraphDepsCmd = &cobra.Command{
	Use:   "deps <package>",
	Short: "Find package dependencies",
	Args:  cobra.ExactArgs(1),
	RunE:  runCodegraphDeps,
}

var (
	codegraphSearchPattern bool
	codegraphJSON          bool
)

func init() {
	CodegraphCmd.AddCommand(codegraphStatsCmd)
	CodegraphCmd.AddCommand(codegraphIndexCmd)
	CodegraphCmd.AddCommand(codegraphSearchCmd)
	CodegraphCmd.AddCommand(codegraphCallersCmd)
	CodegraphCmd.AddCommand(codegraphDepsCmd)

	codegraphSearchCmd.Flags().BoolVar(&codegraphSearchPattern, "pattern", false, "Use LIKE pattern matching (% for wildcards)")
	CodegraphCmd.PersistentFlags().BoolVar(&codegraphJSON, "json", false, "Output raw JSON response")
}

func runCodegraphStats(_ *cobra.Command, _ []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	resp, err := callWorktree(ctx, "codegraph.stats", nil)
	if err != nil {
		return fmt.Errorf("codegraph.stats: %w", err)
	}

	if codegraphJSON {
		return outputJSON(resp.Result)
	}

	var stats map[string]int
	if err := json.Unmarshal(resp.Result, &stats); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	fmt.Println("Code Graph Statistics")
	// Show top-level stats first
	for _, key := range []string{"files", "symbols", "edges"} {
		if v, ok := stats[key]; ok {
			fmt.Printf("  %-20s %d\n", key, v)
		}
	}

	// Show breakdowns
	keys := slices.Sorted(maps.Keys(stats))
	keys = slices.DeleteFunc(keys, func(k string) bool {
		return k == "files" || k == "symbols" || k == "edges"
	})
	if len(keys) > 0 {
		fmt.Println("  Breakdown:")
		for _, k := range keys {
			fmt.Printf("    %-20s %d\n", k, stats[k])
		}
	}

	return nil
}

func runCodegraphIndex(_ *cobra.Command, args []string) error {
	params := map[string]any{}
	if len(args) > 0 {
		params["path"] = args[0]
	}

	spinner := cli.NewSpinner("Indexing code symbols...")
	spinner.Start()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	resp, err := callWorktree(ctx, "codegraph.index", params)
	if err != nil {
		spinner.Fail("Indexing failed")

		return fmt.Errorf("codegraph.index: %w", err)
	}

	if codegraphJSON {
		spinner.Stop()

		return outputJSON(resp.Result)
	}

	var result struct {
		Files   int `json:"files"`
		Symbols int `json:"symbols"`
		Edges   int `json:"edges"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		spinner.Fail("Invalid response")

		return fmt.Errorf("parse result: %w", err)
	}

	spinner.Success(fmt.Sprintf("Indexed %d files (%d symbols, %d edges)", result.Files, result.Symbols, result.Edges))

	return nil
}

func runCodegraphSearch(_ *cobra.Command, args []string) error {
	params := map[string]any{
		"name":    args[0],
		"pattern": codegraphSearchPattern,
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	resp, err := callWorktree(ctx, "codegraph.search", params)
	if err != nil {
		return fmt.Errorf("codegraph.search: %w", err)
	}

	if codegraphJSON {
		return outputJSON(resp.Result)
	}

	var result struct {
		Symbols []struct {
			Name    string `json:"name"`
			Kind    string `json:"kind"`
			File    string `json:"file"`
			Line    int    `json:"line"`
			Package string `json:"package"`
		} `json:"symbols"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if len(result.Symbols) == 0 {
		fmt.Println("No symbols found.")

		return nil
	}

	fmt.Printf("Found %d symbol(s):\n\n", len(result.Symbols))
	for _, s := range result.Symbols {
		fmt.Printf("  %-12s %-30s %s:%d  (%s)\n", s.Kind, s.Name, s.File, s.Line, s.Package)
	}

	return nil
}

func runCodegraphCallers(_ *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	resp, err := callWorktree(ctx, "codegraph.callers", map[string]any{"name": args[0]})
	if err != nil {
		return fmt.Errorf("codegraph.callers: %w", err)
	}

	if codegraphJSON {
		return outputJSON(resp.Result)
	}

	var result struct {
		Callers []struct {
			Name    string `json:"name"`
			Kind    string `json:"kind"`
			File    string `json:"file"`
			Line    int    `json:"line"`
			Package string `json:"package"`
		} `json:"callers"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if len(result.Callers) == 0 {
		fmt.Printf("No callers found for %q.\n", args[0])

		return nil
	}

	fmt.Printf("Callers of %q (%d):\n\n", args[0], len(result.Callers))
	for _, c := range result.Callers {
		fmt.Printf("  %-30s %s:%d  (%s)\n", c.Name, c.File, c.Line, c.Package)
	}

	return nil
}

func runCodegraphDeps(_ *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	resp, err := callWorktree(ctx, "codegraph.deps", map[string]any{"package": args[0]})
	if err != nil {
		return fmt.Errorf("codegraph.deps: %w", err)
	}

	if codegraphJSON {
		return outputJSON(resp.Result)
	}

	var result struct {
		Dependencies []string `json:"dependencies"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("parse result: %w", err)
	}

	if len(result.Dependencies) == 0 {
		fmt.Printf("No dependencies found for package %q.\n", args[0])

		return nil
	}

	fmt.Printf("Dependencies of %q (%d):\n", args[0], len(result.Dependencies))
	for _, d := range result.Dependencies {
		fmt.Printf("  %s\n", d)
	}

	return nil
}
