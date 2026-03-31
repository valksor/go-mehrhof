package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/valksor/kvelmo/pkg/socket"
)

// commandResultMsg carries the text output from a slash command execution.
type commandResultMsg struct {
	output string
}

// tuiCommand defines a slash command available in the TUI chat input.
type tuiCommand struct {
	name        string
	description string
	worktree    bool // true = worktree socket, false = global socket
}

// commands is the list of all slash commands the TUI supports.
// IMPORTANT: Within each group, longer names MUST appear before shorter prefixes
// (e.g., "/checkpoints goto" before "/checkpoints", "/changelog full" before "/changelog")
// because parseSlashCommand does a linear scan and returns the first prefix match.
var commands = []tuiCommand{
	// Workflow
	{name: "/quick", description: "Quick fix: load, implement, submit", worktree: true},
	{name: "/plan!", description: "Re-run planning", worktree: true},
	{name: "/plan", description: "Run planning phase", worktree: true},
	{name: "/implement!", description: "Re-run implementation", worktree: true},
	{name: "/implement", description: "Run implementation phase", worktree: true},
	{name: "/simplify", description: "Run simplification pass", worktree: true},
	{name: "/optimize", description: "Run optimization pass", worktree: true},
	{name: "/review fix", description: "Review with automatic fixes", worktree: true},
	{name: "/review", description: "Review implementation", worktree: true},

	// Control
	{name: "/undo", description: "Undo to previous checkpoint", worktree: true},
	{name: "/redo", description: "Redo to next checkpoint", worktree: true},
	{name: "/stop", description: "Stop current operation", worktree: true},
	{name: "/abort", description: "Abort current operation", worktree: true},
	{name: "/reset", description: "Reset task to initial state", worktree: true},
	{name: "/update", description: "Update task from source", worktree: true},

	// Inspection
	{name: "/status", description: "Show task state", worktree: true},
	{name: "/checkpoints goto", description: "Jump to checkpoint", worktree: true},
	{name: "/checkpoints", description: "List checkpoints", worktree: true},
	{name: "/recap", description: "Summarize task state", worktree: true},
	{name: "/diff", description: "Show file changes", worktree: true},
	{name: "/show spec", description: "Show specification", worktree: true},
	{name: "/show plan", description: "Show plan", worktree: true},
	{name: "/list search", description: "Search task history", worktree: true},
	{name: "/list", description: "List task history", worktree: true},
	{name: "/eventlog", description: "View lifecycle events", worktree: true},
	{name: "/jobs", description: "List job queue", worktree: false},
	{name: "/stats", description: "Show metrics", worktree: false},

	// Organization
	{name: "/tag add", description: "Add tag", worktree: true},
	{name: "/tag remove", description: "Remove tag", worktree: true},
	{name: "/tags", description: "List tags", worktree: true},
	{name: "/queue add", description: "Queue a task", worktree: true},
	{name: "/queue remove", description: "Remove from queue", worktree: true},
	{name: "/queue list", description: "List queue", worktree: true},
	{name: "/queue", description: "List queue", worktree: true},
	{name: "/fork create", description: "Create fork", worktree: true},
	{name: "/fork list", description: "List forks", worktree: true},
	{name: "/fork compare", description: "Compare forks", worktree: true},
	{name: "/fork select", description: "Select fork", worktree: true},
	{name: "/group create", description: "Create task group", worktree: false},
	{name: "/group add", description: "Add task to group", worktree: false},
	{name: "/group list", description: "List task groups", worktree: false},
	{name: "/group status", description: "Show group status", worktree: false},
	{name: "/group submit", description: "Submit grouped tasks", worktree: false},
	{name: "/group remove", description: "Delete a task group", worktree: false},
	{name: "/batch", description: "Run action across projects", worktree: false},
	{name: "/activity", description: "View activity log", worktree: false},
	{name: "/audit", description: "View audit trail", worktree: true},
	{name: "/report", description: "Generate report", worktree: false},
	{name: "/backup", description: "Create backup", worktree: false},
	{name: "/access", description: "List access tokens", worktree: false},

	// Governance
	{name: "/approve", description: "Approve transition", worktree: true},
	{name: "/quality", description: "Run quality gates", worktree: true},
	{name: "/retry", description: "Re-run failed phase", worktree: true},
	{name: "/checklist check", description: "Check item", worktree: true},
	{name: "/checklist uncheck", description: "Uncheck item", worktree: true},
	{name: "/checklist", description: "Show checklist", worktree: true},
	{name: "/ci", description: "CI status", worktree: true},
	{name: "/policy", description: "Check policy", worktree: true},

	// Files & Code
	{name: "/files search", description: "Search files", worktree: true},
	{name: "/files", description: "List files", worktree: true},
	{name: "/git status", description: "Git status", worktree: true},
	{name: "/git log", description: "Git log", worktree: true},
	{name: "/codegraph search", description: "Search symbols", worktree: true},

	// Memory & Cache
	{name: "/memory search", description: "Search memory", worktree: false},
	{name: "/memory stats", description: "Memory stats", worktree: false},
	{name: "/cache stats", description: "Cache stats", worktree: true},
	{name: "/cache clear", description: "Clear cache", worktree: true},

	// Infrastructure
	{name: "/changelog full", description: "Changelog with descriptions", worktree: true},
	{name: "/changelog", description: "Generate changelog", worktree: true},
	{name: "/workers", description: "List workers", worktree: false},
	{name: "/discover", description: "Scan project commands", worktree: true},
	{name: "/diagnose", description: "System diagnostics", worktree: false},
	{name: "/security scan", description: "Security scan", worktree: false},
	{name: "/remote approve", description: "Approve PR", worktree: true},
	{name: "/remote merge", description: "Merge PR", worktree: true},
	{name: "/onboarding reset", description: "Reset onboarding guide", worktree: false},
	{name: "/config check", description: "Check config drift", worktree: false},

	// Modal-equivalent (execute directly in TUI)
	{name: "/submit", description: "Submit PR", worktree: true},
	{name: "/finish", description: "Finish task", worktree: true},
	{name: "/abandon", description: "Abandon task", worktree: true},
	{name: "/delete", description: "Delete task", worktree: true},

	// Chat-based
	{name: "/explain", description: "Explain last action", worktree: true},
}

// parseSlashCommand checks if input starts with "/" and returns the matching
// command plus any remaining args. Returns nil if no command matches.
func parseSlashCommand(input string) (*tuiCommand, string) {
	if !strings.HasPrefix(input, "/") {
		return nil, ""
	}

	for i := range commands {
		cmd := &commands[i]
		if input == cmd.name || strings.HasPrefix(input, cmd.name+" ") {
			args := strings.TrimSpace(input[len(cmd.name):])

			return cmd, args
		}
	}

	return nil, ""
}

// executeCommand runs a slash command and returns a tea.Cmd that produces a
// commandResultMsg with the text output.
func (m *Model) executeCommand(cmd *tuiCommand, args string) tea.Cmd {
	ctx := m.ctx

	// Global commands don't require an active worktree.
	if !cmd.worktree {
		return func() tea.Msg {
			output, err := executeGlobalCommand(ctx, cmd.name, args)
			if err != nil {
				return commandResultMsg{output: fmt.Sprintf("Error: %v", err)}
			}

			return commandResultMsg{output: output}
		}
	}

	wt := m.activeWorktree()
	if wt == nil {
		return func() tea.Msg { return commandResultMsg{output: "No active worktree."} }
	}
	dir := wt.Dir
	dryRun := m.dryRun

	return func() tea.Msg {
		output, err := executeWorktreeCommand(ctx, dir, cmd.name, args, dryRun)
		if err != nil {
			return commandResultMsg{output: fmt.Sprintf("Error: %v", err)}
		}

		return commandResultMsg{output: output}
	}
}

func executeWorktreeCommand(ctx context.Context, dir, name, args string, dryRun bool) (string, error) {
	socketPath := socket.WorktreeSocketPath(dir)
	client, err := socket.NewClient(socketPath, socket.WithTimeout(30*time.Second))
	if err != nil {
		return "", fmt.Errorf("connect: %w", err)
	}
	defer func() { _ = client.Close() }()

	if handler, ok := worktreeHandlers[name]; ok {
		return handler(ctx, client, args, dryRun)
	}

	return "Unknown command: " + name, nil
}

func executeGlobalCommand(ctx context.Context, name, args string) (string, error) {
	globalPath := socket.GlobalSocketPath()
	client, err := socket.NewClient(globalPath, socket.WithTimeout(15*time.Second))
	if err != nil {
		return "", fmt.Errorf("connect to global socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	if handler, ok := globalHandlers[name]; ok {
		return handler(ctx, client, args)
	}

	return "Unknown global command: " + name, nil
}

// mustJSON marshals v to json.RawMessage, panicking on error (safe for known types).
func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("mustJSON: %v", err))
	}

	return b
}

// optDryRun returns nil or a JSON object with dry_run set.
func optDryRun(dryRun bool) json.RawMessage {
	if !dryRun {
		return nil
	}

	return json.RawMessage(mustJSON(map[string]any{"dry_run": true}))
}

// formatTaskList formats a task.history or task.search response.
func formatTaskList(data json.RawMessage) string {
	var result struct {
		Tasks []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
			State string `json:"state"`
		} `json:"tasks"`
	}
	_ = json.Unmarshal(data, &result)
	if len(result.Tasks) == 0 {
		return "No tasks."
	}
	var lines []string
	for _, t := range result.Tasks {
		lines = append(lines, fmt.Sprintf("%s [%s] %s", t.ID[:min(8, len(t.ID))], t.State, t.Title))
	}

	return strings.Join(lines, "\n")
}
