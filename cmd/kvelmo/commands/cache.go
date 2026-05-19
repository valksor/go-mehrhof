package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

// CacheCmd is the top-level command for response cache management.
var CacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Response cache management",
	Long:  "View statistics and manage the semantic response cache used for agent prompt deduplication.",
}

var cacheStatsCmd = &cobra.Command{
	Use:   subStats,
	Short: "Show cache statistics",
	Long:  "Display hit/miss rates, entry count, and tokens saved by the response cache.",
	RunE:  runCacheStats,
}

var cacheClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear the response cache",
	Long:  "Remove all entries from the semantic response cache.",
	RunE:  runCacheClear,
}

func init() {
	CacheCmd.AddCommand(cacheStatsCmd)
	CacheCmd.AddCommand(cacheClearCmd)
}

func runCacheStats(_ *cobra.Command, _ []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	resp, err := callWorktree(ctx, "cache.stats", nil)
	if err != nil {
		return fmt.Errorf("cache.stats: %w", err)
	}

	return outputJSON(resp.Result)
}

func runCacheClear(_ *cobra.Command, _ []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	_, err := callWorktree(ctx, "cache.clear", nil)
	if err != nil {
		return fmt.Errorf("cache.clear: %w", err)
	}

	fmt.Println("Response cache cleared.")

	return nil
}
