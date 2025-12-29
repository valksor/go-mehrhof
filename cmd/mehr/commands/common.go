package commands

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/valksor/go-mehrhof/internal/agent/claude"
	"github.com/valksor/go-mehrhof/internal/conductor"
	"github.com/valksor/go-mehrhof/internal/output"
	"github.com/valksor/go-mehrhof/internal/provider/directory"
	"github.com/valksor/go-mehrhof/internal/provider/file"
	"github.com/valksor/go-mehrhof/internal/provider/github"
	"github.com/valksor/go-mehrhof/internal/provider/jira"
	"github.com/valksor/go-mehrhof/internal/provider/linear"
	"github.com/valksor/go-mehrhof/internal/provider/notion"
	"github.com/valksor/go-mehrhof/internal/provider/wrike"
	"github.com/valksor/go-mehrhof/internal/provider/youtrack"
)

// dedupStdout is the shared deduplicating writer for verbose output.
// It's initialized once and reused across commands to maintain dedup state.
var dedupStdout *output.DeduplicatingWriter

// getDeduplicatingStdout returns a deduplicating writer that wraps os.Stdout.
// The writer suppresses consecutive identical lines.
func getDeduplicatingStdout() io.Writer {
	if dedupStdout == nil {
		dedupStdout = output.NewDeduplicatingWriter(os.Stdout)
	}
	return dedupStdout
}

// initializeConductor creates and initializes a conductor with the standard
// providers (file, directory) and agents (claude) registered.
//
// This is the common initialization sequence used by most commands.
// Options should be built by the caller to customize behavior per command.
func initializeConductor(ctx context.Context, opts ...conductor.Option) (*conductor.Conductor, error) {
	// Create conductor with provided options
	cond, err := conductor.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("create conductor: %w", err)
	}

		// Register standard providers
	file.Register(cond.GetProviderRegistry())
	directory.Register(cond.GetProviderRegistry())
	github.Register(cond.GetProviderRegistry())
	wrike.Register(cond.GetProviderRegistry())
	linear.Register(cond.GetProviderRegistry())
	jira.Register(cond.GetProviderRegistry())
	notion.Register(cond.GetProviderRegistry())
	youtrack.Register(cond.GetProviderRegistry())

	// Register standard agents
	if err := claude.Register(cond.GetAgentRegistry()); err != nil {
		return nil, fmt.Errorf("register claude agent: %w", err)
	}

	// Initialize the conductor (loads workspace, detects agent, etc.)
	if err := cond.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("initialize: %w", err)
	}

	return cond, nil
}

// confirmAction prompts the user for confirmation unless skipConfirm is true.
// Returns true if the action should proceed, false if cancelled.
// The prompt parameter should describe what will happen (e.g., "delete this task").
func confirmAction(prompt string, skipConfirm bool) (bool, error) {
	if skipConfirm {
		return true, nil
	}

	fmt.Printf("%s\nAre you sure? [y/N]: ", prompt)

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("read response: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes", nil
}
