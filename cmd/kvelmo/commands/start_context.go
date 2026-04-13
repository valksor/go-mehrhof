package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/valksor/kvelmo/internal/conductor"
)

// buildContextItems constructs ContextItems from CLI flags.
func buildContextItems() []conductor.ContextItem {
	return buildContextItemsFromFlags(startContextFiles, startContextSymbol, startContextCommit)
}

// buildContextItemsFromFlags constructs ContextItems from file/symbol/commit flag slices.
// Shared by start and quick commands to avoid divergence.
func buildContextItemsFromFlags(files, symbols, commits []string) []conductor.ContextItem {
	items := make([]conductor.ContextItem, 0, len(files)+len(symbols)+len(commits))
	for _, f := range files {
		items = append(items, conductor.ContextItem{Type: conductor.ContextTypeFile, Ref: f})
	}
	for _, s := range symbols {
		items = append(items, conductor.ContextItem{Type: conductor.ContextTypeSymbol, Ref: s})
	}
	for _, c := range commits {
		items = append(items, conductor.ContextItem{Type: conductor.ContextTypeCommit, Ref: c})
	}

	return items
}

// validateContextItems performs fast-fail validation on context items before sending to RPC.
// Checks that file references exist and are not path traversal attempts.
func validateContextItems(items []conductor.ContextItem) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	for _, item := range items {
		if item.Type != conductor.ContextTypeFile {
			continue
		}
		// Parse file:line ref to get just the path (reuse server-side parser for consistency)
		ref, _ := conductor.ParseFileRef(item.Ref)

		// Reject absolute paths
		if filepath.IsAbs(ref) {
			return fmt.Errorf("--file %q: absolute paths not allowed", item.Ref)
		}

		// Check containment
		resolved := filepath.Clean(filepath.Join(cwd, ref))
		cleanCwd := filepath.Clean(cwd)
		if resolved != cleanCwd && !strings.HasPrefix(resolved, cleanCwd+string(filepath.Separator)) {
			return fmt.Errorf("--file %q: path escapes working directory", item.Ref)
		}

		// Check existence
		if _, err := os.Stat(resolved); err != nil {
			return fmt.Errorf("--file %q: %w", item.Ref, err)
		}
	}

	return nil
}

// runProvisionPreview calls the provision.preview RPC to show what would be provisioned.
func runProvisionPreview() error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	resp, err := callWorktree(ctx, "provision.preview", nil)
	if err != nil {
		return fmt.Errorf("provision.preview: %w", err)
	}

	return outputJSON(resp.Result)
}
