package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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

	switch name {
	// ── Workflow ─────────────────────────────────────────────────────────
	case "/quick":
		if args == "" {
			return "Usage: /quick <source>", nil
		}
		_, err := client.Call(ctx, "start", json.RawMessage(mustJSON(map[string]any{"source": args, "auto_advance": true})))
		if err != nil {
			return "", err
		}

		return "Quick fix started.", nil

	case "/plan", "/plan!":
		params := map[string]any{}
		if dryRun {
			params["dry_run"] = true
		}
		_, err := client.Call(ctx, "plan", json.RawMessage(mustJSON(params)))
		if err != nil {
			return "", err
		}

		return "Planning started.", nil

	case "/implement", "/implement!":
		params := map[string]any{}
		if dryRun {
			params["dry_run"] = true
		}
		_, err := client.Call(ctx, "implement", json.RawMessage(mustJSON(params)))
		if err != nil {
			return "", err
		}

		return "Implementation started.", nil

	case "/simplify":
		_, err := client.Call(ctx, "simplify", optDryRun(dryRun))
		if err != nil {
			return "", err
		}

		return "Simplification started.", nil

	case "/optimize":
		_, err := client.Call(ctx, "optimize", optDryRun(dryRun))
		if err != nil {
			return "", err
		}

		return "Optimization started.", nil

	case "/review":
		_, err := client.Call(ctx, "review", json.RawMessage(mustJSON(map[string]any{"approve": true})))
		if err != nil {
			return "", err
		}

		return "Review started.", nil

	case "/review fix":
		_, err := client.Call(ctx, "review", json.RawMessage(mustJSON(map[string]any{"fix": true})))
		if err != nil {
			return "", err
		}

		return "Review with fixes started.", nil

	case "/submit":
		_, err := client.Call(ctx, "submit", optDryRun(dryRun))
		if err != nil {
			return "", err
		}

		return "Submit started.", nil

	case "/finish":
		_, err := client.Call(ctx, "task.finish", nil)
		if err != nil {
			return "", err
		}

		return "Finish started.", nil

	case "/abandon":
		_, err := client.Call(ctx, "abandon", nil)
		if err != nil {
			return "", err
		}

		return "Task abandoned.", nil

	case "/delete":
		_, err := client.Call(ctx, "delete", nil)
		if err != nil {
			return "", err
		}

		return "Task deleted.", nil

	// ── Control ──────────────────────────────────────────────────────────
	case "/undo":
		_, err := client.Call(ctx, "undo", nil)
		if err != nil {
			return "", err
		}

		return "Undone to previous checkpoint.", nil

	case "/redo":
		_, err := client.Call(ctx, "redo", nil)
		if err != nil {
			return "", err
		}

		return "Redone to next checkpoint.", nil

	case "/stop":
		_, err := client.Call(ctx, "stop", nil)
		if err != nil {
			return "", err
		}

		return "Operation stopped.", nil

	case "/abort":
		_, err := client.Call(ctx, "abort", nil)
		if err != nil {
			return "", err
		}

		return "Operation aborted.", nil

	case "/reset":
		_, err := client.Call(ctx, "reset", nil)
		if err != nil {
			return "", err
		}

		return "Task reset.", nil

	case "/update":
		resp, err := client.Call(ctx, "update", nil)
		if err != nil {
			return "", err
		}
		var result struct {
			Changed bool `json:"changed"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		if result.Changed {
			return "Task updated from source.", nil
		}

		return "Task is already up to date.", nil

	// ── Inspection ───────────────────────────────────────────────────────
	case "/status":
		resp, err := client.Call(ctx, "status", nil)
		if err != nil {
			return "", err
		}
		var result struct {
			State string `json:"state"`
			Title string `json:"title"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		if result.Title != "" {
			return fmt.Sprintf("State: %s — %s", result.State, result.Title), nil
		}

		return "State: " + result.State, nil

	case "/checkpoints":
		resp, err := client.Call(ctx, "checkpoints", nil)
		if err != nil {
			return "", err
		}
		var result struct {
			Checkpoints []struct {
				SHA     string `json:"sha"`
				Message string `json:"message"`
			} `json:"checkpoints"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		if len(result.Checkpoints) == 0 {
			return "No checkpoints.", nil
		}
		var lines []string
		for i, cp := range result.Checkpoints {
			lines = append(lines, fmt.Sprintf("%d. %s — %s", i+1, cp.SHA[:min(7, len(cp.SHA))], cp.Message))
		}

		return strings.Join(lines, "\n"), nil

	case "/checkpoints goto":
		if args == "" {
			return "Usage: /checkpoints goto <sha>", nil
		}
		_, err := client.Call(ctx, "checkpoint.goto", json.RawMessage(mustJSON(map[string]string{"sha": args})))
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("Jumped to checkpoint %s.", args[:min(7, len(args))]), nil

	case "/recap":
		resp, err := client.Call(ctx, "recap", nil)
		if err != nil {
			return "", err
		}
		var result struct {
			Recap string `json:"recap"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		if result.Recap == "" {
			return "No recap available.", nil
		}

		return result.Recap, nil

	case "/diff":
		resp, err := client.Call(ctx, "git.diff", nil)
		if err != nil {
			return "", err
		}
		var result struct {
			Diff string `json:"diff"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		if result.Diff == "" {
			return "No changes.", nil
		}

		return result.Diff, nil

	case "/show spec", "/show plan":
		method := "show.spec"
		if name == "/show plan" {
			method = "show.plan"
		}
		resp, err := client.Call(ctx, method, nil)
		if err != nil {
			return "", err
		}
		var result struct {
			Specifications []struct {
				Content string `json:"content"`
			} `json:"specifications"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		if len(result.Specifications) == 0 {
			return "No specification available.", nil
		}
		var parts []string
		for _, s := range result.Specifications {
			parts = append(parts, s.Content)
		}

		return strings.Join(parts, "\n---\n\n"), nil

	case "/list search":
		if args == "" {
			return "Usage: /list search <query>", nil
		}
		resp, err := client.Call(ctx, "task.search", json.RawMessage(mustJSON(map[string]string{"query": args})))
		if err != nil {
			return "", err
		}

		return formatTaskList(resp.Result), nil

	case "/list":
		resp, err := client.Call(ctx, "task.history", nil)
		if err != nil {
			return "", err
		}

		return formatTaskList(resp.Result), nil

	case "/eventlog":
		resp, err := client.Call(ctx, "eventlog.query", json.RawMessage(mustJSON(map[string]int{"limit": 20})))
		if err != nil {
			return "", err
		}
		var result struct {
			Events []struct {
				Type      string `json:"type"`
				Timestamp string `json:"timestamp"`
				Message   string `json:"message"`
			} `json:"events"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		if len(result.Events) == 0 {
			return "No events.", nil
		}
		var lines []string
		for _, e := range result.Events {
			line := fmt.Sprintf("[%s] %s", e.Timestamp, e.Type)
			if e.Message != "" {
				line += ": " + e.Message
			}
			lines = append(lines, line)
		}

		return strings.Join(lines, "\n"), nil

	// ── Tags ─────────────────────────────────────────────────────────────
	case "/tag add":
		if args == "" {
			return "Usage: /tag add <name>", nil
		}
		_, err := client.Call(ctx, "task.tag", json.RawMessage(mustJSON(map[string]string{"action": "add", "tag": args})))
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("Tag %q added.", args), nil

	case "/tag remove":
		if args == "" {
			return "Usage: /tag remove <name>", nil
		}
		_, err := client.Call(ctx, "task.tag", json.RawMessage(mustJSON(map[string]string{"action": "remove", "tag": args})))
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("Tag %q removed.", args), nil

	case "/tags":
		resp, err := client.Call(ctx, "task.tag", json.RawMessage(mustJSON(map[string]string{"action": "list"})))
		if err != nil {
			return "", err
		}
		var result struct {
			Tags []string `json:"tags"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		if len(result.Tags) == 0 {
			return "No tags.", nil
		}

		return "Tags: " + strings.Join(result.Tags, ", "), nil

	// ── Queue ────────────────────────────────────────────────────────────
	case "/queue add":
		if args == "" {
			return "Usage: /queue add <source>", nil
		}
		_, err := client.Call(ctx, "queue.add", json.RawMessage(mustJSON(map[string]string{"source": args})))
		if err != nil {
			return "", err
		}

		return "Queued: " + args, nil

	case "/queue remove":
		if args == "" {
			return "Usage: /queue remove <id>", nil
		}
		_, err := client.Call(ctx, "queue.remove", json.RawMessage(mustJSON(map[string]string{"id": args})))
		if err != nil {
			return "", err
		}

		return "Removed from queue.", nil

	case "/queue list", "/queue":
		resp, err := client.Call(ctx, "queue.list", nil)
		if err != nil {
			return "", err
		}
		var result struct {
			Queue []struct {
				ID     string `json:"id"`
				Title  string `json:"title"`
				Source string `json:"source"`
			} `json:"queue"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		if len(result.Queue) == 0 {
			return "Queue is empty.", nil
		}
		var lines []string
		for i, t := range result.Queue {
			label := t.Title
			if label == "" {
				label = t.Source
			}
			lines = append(lines, fmt.Sprintf("%d. [%s] %s", i+1, t.ID[:min(8, len(t.ID))], label))
		}

		return strings.Join(lines, "\n"), nil

	// ── Forks ────────────────────────────────────────────────────────────
	case "/fork create":
		if args == "" {
			return "Usage: /fork create <label>", nil
		}
		_, err := client.Call(ctx, "fork.create", json.RawMessage(mustJSON(map[string]string{"label": args})))
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("Fork %q created.", args), nil

	case "/fork list":
		resp, err := client.Call(ctx, "fork.list", nil)
		if err != nil {
			return "", err
		}
		var result struct {
			Forks []struct {
				ID    string `json:"id"`
				Label string `json:"label"`
				State string `json:"state"`
			} `json:"forks"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		if len(result.Forks) == 0 {
			return "No active forks.", nil
		}
		var lines []string
		for _, f := range result.Forks {
			lines = append(lines, fmt.Sprintf("%s — %s [%s]", f.ID[:min(8, len(f.ID))], f.Label, f.State))
		}

		return strings.Join(lines, "\n"), nil

	case "/fork compare":
		resp, err := client.Call(ctx, "fork.compare", nil)
		if err != nil {
			return "", err
		}

		return string(resp.Result), nil

	case "/fork select":
		if args == "" {
			return "Usage: /fork select <fork-id>", nil
		}
		_, err := client.Call(ctx, "fork.select", json.RawMessage(mustJSON(map[string]string{"fork_id": args})))
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("Switched to fork %s.", args[:min(8, len(args))]), nil

	// ── Governance ───────────────────────────────────────────────────────
	case "/approve":
		_, err := client.Call(ctx, "approve", nil)
		if err != nil {
			return "", err
		}

		return "Approved.", nil

	case "/checklist check":
		if args == "" {
			return "Usage: /checklist check <number>", nil
		}
		_, err := client.Call(ctx, "review.checklist.check", json.RawMessage(mustJSON(map[string]string{"index": args})))
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("Checklist item %s checked.", args), nil

	case "/checklist uncheck":
		if args == "" {
			return "Usage: /checklist uncheck <number>", nil
		}
		_, err := client.Call(ctx, "review.checklist.uncheck", json.RawMessage(mustJSON(map[string]string{"index": args})))
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("Checklist item %s unchecked.", args), nil

	case "/checklist":
		resp, err := client.Call(ctx, "review.checklist.get", nil)
		if err != nil {
			return "", err
		}
		var result struct {
			Items []struct {
				Label   string `json:"label"`
				Checked bool   `json:"checked"`
			} `json:"items"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		if len(result.Items) == 0 {
			return "No checklist items.", nil
		}
		var lines []string
		for i, item := range result.Items {
			mark := "☐"
			if item.Checked {
				mark = "✓"
			}
			lines = append(lines, fmt.Sprintf("%s %d. %s", mark, i+1, item.Label))
		}

		return strings.Join(lines, "\n"), nil

	case "/ci":
		resp, err := client.Call(ctx, "ci.status", nil)
		if err != nil {
			return "", err
		}
		var result struct {
			Status string `json:"status"`
			Checks []struct {
				Name   string `json:"name"`
				Status string `json:"status"`
			} `json:"checks"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		out := "CI: " + result.Status
		var checksBuilder strings.Builder
		for _, c := range result.Checks {
			mark := "✗"
			if c.Status == "passed" {
				mark = "✓"
			}
			fmt.Fprintf(&checksBuilder, "\n  %s %s", mark, c.Name)
		}
		out += checksBuilder.String()

		return out, nil

	case "/policy":
		resp, err := client.Call(ctx, "policy.check", nil)
		if err != nil {
			return "", err
		}
		var result struct {
			Compliant  bool `json:"compliant"`
			Violations []struct {
				Rule    string `json:"rule"`
				Message string `json:"message"`
			} `json:"violations"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		if result.Compliant {
			return "Policy: compliant.", nil
		}
		var lines []string
		for _, v := range result.Violations {
			lines = append(lines, fmt.Sprintf("  • %s: %s", v.Rule, v.Message))
		}

		return "Policy violations:\n" + strings.Join(lines, "\n"), nil

	// ── Files & Code ─────────────────────────────────────────────────────
	case "/files search":
		if args == "" {
			return "Usage: /files search <pattern>", nil
		}
		resp, err := client.Call(ctx, "files.search", json.RawMessage(mustJSON(map[string]string{"pattern": args})))
		if err != nil {
			return "", err
		}
		var result struct {
			Files []string `json:"files"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		if len(result.Files) == 0 {
			return "No matching files.", nil
		}

		return strings.Join(result.Files, "\n"), nil

	case "/files":
		path := args
		if path == "" {
			path = "."
		}
		resp, err := client.Call(ctx, "files.list", json.RawMessage(mustJSON(map[string]string{"path": path})))
		if err != nil {
			return "", err
		}
		var result struct {
			Files []string `json:"files"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		if len(result.Files) == 0 {
			return "No files.", nil
		}

		return strings.Join(result.Files, "\n"), nil

	case "/git status":
		resp, err := client.Call(ctx, "git.status", nil)
		if err != nil {
			return "", err
		}
		var result struct {
			Branch     string `json:"branch"`
			HasChanges bool   `json:"has_changes"`
			Summary    string `json:"summary"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		out := "Branch: " + result.Branch
		if result.Summary != "" {
			out += "\n" + result.Summary
		} else if result.HasChanges {
			out += " (has changes)"
		} else {
			out += " (clean)"
		}

		return out, nil

	case "/git log":
		resp, err := client.Call(ctx, "git.log", json.RawMessage(mustJSON(map[string]int{"limit": 10})))
		if err != nil {
			return "", err
		}
		var result struct {
			Entries []struct {
				SHA     string `json:"sha"`
				Message string `json:"message"`
			} `json:"entries"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		if len(result.Entries) == 0 {
			return "No commits.", nil
		}
		var lines []string
		for _, e := range result.Entries {
			lines = append(lines, fmt.Sprintf("%s %s", e.SHA[:min(7, len(e.SHA))], e.Message))
		}

		return strings.Join(lines, "\n"), nil

	case "/codegraph search":
		if args == "" {
			return "Usage: /codegraph search <symbol>", nil
		}
		resp, err := client.Call(ctx, "codegraph.search", json.RawMessage(mustJSON(map[string]string{"name": args})))
		if err != nil {
			return "", err
		}
		var result struct {
			Symbols []struct {
				Name string `json:"name"`
				Kind string `json:"kind"`
				File string `json:"file"`
				Line int    `json:"line"`
			} `json:"symbols"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		if len(result.Symbols) == 0 {
			return "No symbols found.", nil
		}
		var lines []string
		for _, s := range result.Symbols {
			lines = append(lines, fmt.Sprintf("%s %s — %s:%d", s.Kind, s.Name, s.File, s.Line))
		}

		return strings.Join(lines, "\n"), nil

	// ── Cache ────────────────────────────────────────────────────────────
	case "/cache stats":
		resp, err := client.Call(ctx, "cache.stats", nil)
		if err != nil {
			return "", err
		}

		return string(resp.Result), nil

	case "/cache clear":
		_, err := client.Call(ctx, "cache.clear", nil)
		if err != nil {
			return "", err
		}

		return "Cache cleared.", nil

	// ── Changelog ────────────────────────────────────────────────────────
	case "/changelog", "/changelog full":
		if args == "" {
			return "Usage: /changelog <source>..<target> [note]", nil
		}
		// Split off note: "source..target optional note text"
		refPart, note, _ := strings.Cut(args, " ")
		parts := strings.SplitN(refPart, "..", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return "Usage: /changelog <source>..<target> [note]", nil
		}
		params := map[string]any{"source": parts[0], "target": parts[1]}
		if name == "/changelog full" {
			params["full"] = true
		}
		if note = strings.TrimSpace(note); note != "" {
			params["note"] = note
		}
		resp, err := client.Call(ctx, "changelog.generate", json.RawMessage(mustJSON(params)))
		if err != nil {
			return "", err
		}
		var result struct {
			Markdown string `json:"markdown"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		if result.Markdown == "" {
			return fmt.Sprintf("No commits between %s and %s", parts[0], parts[1]), nil
		}

		return result.Markdown, nil

	// ── Remote ───────────────────────────────────────────────────────────
	case "/remote approve":
		_, err := client.Call(ctx, "remote.approve", json.RawMessage(mustJSON(map[string]string{"comment": ""})))
		if err != nil {
			return "", err
		}

		return "PR approved.", nil

	case "/remote merge":
		_, err := client.Call(ctx, "remote.merge", json.RawMessage(mustJSON(map[string]string{"method": "rebase"})))
		if err != nil {
			return "", err
		}

		return "PR merged.", nil

	// ── Explain (via chat) ───────────────────────────────────────────────
	case "/explain":
		_, err := client.Call(ctx, "chat.send", json.RawMessage(mustJSON(map[string]string{
			"message": "Explain what you did in the last action, why you made those choices, and any assumptions or constraints you encountered.",
		})))
		if err != nil {
			return "", err
		}

		return "Asking agent to explain...", nil

	case "/quality":
		resp, err := client.Call(ctx, "quality.respond", json.RawMessage(mustJSON(map[string]string{"action": "run"})))
		if err != nil {
			return "", err
		}
		var result struct {
			Status string `json:"status"`
		}
		_ = json.Unmarshal(resp.Result, &result)

		return "Quality: " + result.Status, nil

	case "/retry":
		// Retry re-runs the failed phase: reset to pre-failure state, then re-execute.
		// Note: web's projectStore.retry() infers the failed phase from last_error
		// before resetting. This simpler version maps reset-state to phase, which
		// works for plan/implement/review but not simplify/optimize (both reset to
		// "implemented" and would retry review instead).
		resetResp, err := client.Call(ctx, "reset", nil)
		if err != nil {
			return "", fmt.Errorf("retry reset: %w", err)
		}
		var resetResult struct {
			State string `json:"state"`
		}
		_ = json.Unmarshal(resetResp.Result, &resetResult)

		// Determine which phase to re-run based on the reset state.
		var phase string
		switch resetResult.State {
		case "loaded":
			phase = "plan"
		case "planned":
			phase = "implement"
		case "implemented":
			phase = "review"
		default:
			return fmt.Sprintf("Task reset to %s — use a phase command to continue.", resetResult.State), nil
		}

		_, err = client.Call(ctx, phase, optDryRun(dryRun))
		if err != nil {
			return fmt.Sprintf("Reset succeeded (state: %s) but %s failed: %v", resetResult.State, phase, err), nil
		}

		return fmt.Sprintf("Retrying: reset to %s, re-running %s.", resetResult.State, phase), nil

	case "/audit":
		resp, err := client.Call(ctx, "task.export", json.RawMessage(mustJSON(map[string]string{"format": "audit"})))
		if err != nil {
			return "", err
		}

		return string(resp.Result), nil

	default:
		return "Unknown command: " + name, nil
	}
}

func executeGlobalCommand(ctx context.Context, name, args string) (string, error) {
	globalPath := socket.GlobalSocketPath()
	client, err := socket.NewClient(globalPath, socket.WithTimeout(15*time.Second))
	if err != nil {
		return "", fmt.Errorf("connect to global socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	switch name {
	case "/jobs":
		resp, err := client.Call(ctx, "jobs.list", nil)
		if err != nil {
			return "", err
		}
		var result struct {
			Jobs []struct {
				ID     string `json:"id"`
				Type   string `json:"type"`
				Status string `json:"status"`
			} `json:"jobs"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		if len(result.Jobs) == 0 {
			return "No jobs.", nil
		}
		var lines []string
		for _, j := range result.Jobs {
			lines = append(lines, fmt.Sprintf("%s [%s] %s", j.ID[:min(8, len(j.ID))], j.Status, j.Type))
		}

		return strings.Join(lines, "\n"), nil

	case "/stats":
		resp, err := client.Call(ctx, "metrics", nil)
		if err != nil {
			return "", err
		}

		return string(resp.Result), nil

	case "/workers":
		resp, err := client.Call(ctx, "workers.list", nil)
		if err != nil {
			return "", err
		}
		var result struct {
			Workers []struct {
				Name  string `json:"name"`
				State string `json:"state"`
			} `json:"workers"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		if len(result.Workers) == 0 {
			return "No workers.", nil
		}
		var lines []string
		for _, w := range result.Workers {
			lines = append(lines, fmt.Sprintf("%s [%s]", w.Name, w.State))
		}

		return strings.Join(lines, "\n"), nil

	case "/memory search":
		if args == "" {
			return "Usage: /memory search <query>", nil
		}
		resp, err := client.Call(ctx, "memory.search", json.RawMessage(mustJSON(map[string]any{"query": args, "limit": 5})))
		if err != nil {
			return "", err
		}
		var result struct {
			Results []struct {
				Content string  `json:"content"`
				Score   float64 `json:"score"`
			} `json:"results"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		if len(result.Results) == 0 {
			return "No results.", nil
		}
		var lines []string
		for i, r := range result.Results {
			lines = append(lines, fmt.Sprintf("%d. (%.2f) %s", i+1, r.Score, r.Content))
		}

		return strings.Join(lines, "\n"), nil

	case "/memory stats":
		resp, err := client.Call(ctx, "memory.stats", nil)
		if err != nil {
			return "", err
		}

		return string(resp.Result), nil

	case "/group create":
		if args == "" {
			return "Usage: /group create <label>", nil
		}
		resp, err := client.Call(ctx, "taskgroup.create", json.RawMessage(mustJSON(map[string]string{"label": args})))
		if err != nil {
			return "", err
		}
		var result struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(resp.Result, &result)

		return "Group created: " + result.ID, nil

	case "/group list":
		resp, err := client.Call(ctx, "taskgroup.list", nil)
		if err != nil {
			return "", err
		}
		var result struct {
			Groups []struct {
				ID    string `json:"id"`
				Label string `json:"label"`
			} `json:"groups"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		if len(result.Groups) == 0 {
			return "No groups.", nil
		}
		var lines []string
		for _, g := range result.Groups {
			lines = append(lines, fmt.Sprintf("%s — %s", g.ID[:min(8, len(g.ID))], g.Label))
		}

		return strings.Join(lines, "\n"), nil

	case "/group status":
		if args == "" {
			return "Usage: /group status <group-id>", nil
		}
		resp, err := client.Call(ctx, "taskgroup.status", json.RawMessage(mustJSON(map[string]string{"id": args})))
		if err != nil {
			return "", err
		}

		return string(resp.Result), nil

	case "/diagnose":
		resp, err := client.Call(ctx, "system.diagnose", nil)
		if err != nil {
			return "", err
		}
		var result struct {
			Checks []struct {
				Name   string `json:"name"`
				Status string `json:"status"`
				Detail string `json:"detail"`
			} `json:"checks"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		if len(result.Checks) == 0 {
			return "Diagnostics: OK", nil
		}
		var lines []string
		for _, c := range result.Checks {
			mark := "✗"
			if c.Status == "passed" {
				mark = "✓"
			}
			line := fmt.Sprintf("%s %s", mark, c.Name)
			if c.Detail != "" {
				line += ": " + c.Detail
			}
			lines = append(lines, line)
		}

		return strings.Join(lines, "\n"), nil

	case "/security scan":
		resp, err := client.Call(ctx, "security.scan", nil)
		if err != nil {
			return "", err
		}
		var result struct {
			Issues []struct {
				Severity string `json:"severity"`
				Message  string `json:"message"`
			} `json:"issues"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		if len(result.Issues) == 0 {
			return "No security issues found.", nil
		}
		var lines []string
		for _, i := range result.Issues {
			lines = append(lines, fmt.Sprintf("[%s] %s", i.Severity, i.Message))
		}

		return strings.Join(lines, "\n"), nil

	case "/group add":
		parts := strings.SplitN(args, " ", 2)
		if len(parts) < 2 {
			return "Usage: /group add <group-id> <task-id>", nil
		}
		_, err := client.Call(ctx, "taskgroup.add", json.RawMessage(mustJSON(map[string]string{"id": parts[0], "task_id": parts[1]})))
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("Task added to group %s.", parts[0][:min(8, len(parts[0]))]), nil

	case "/group submit":
		if args == "" {
			return "Usage: /group submit <group-id>", nil
		}
		_, err := client.Call(ctx, "taskgroup.submit", json.RawMessage(mustJSON(map[string]string{"id": args})))
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("Group %s submitted.", args[:min(8, len(args))]), nil

	case "/group remove":
		if args == "" {
			return "Usage: /group remove <group-id>", nil
		}
		_, err := client.Call(ctx, "taskgroup.remove", json.RawMessage(mustJSON(map[string]string{"id": args})))
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("Group %s removed.", args[:min(8, len(args))]), nil

	case "/batch":
		if args == "" {
			return "Usage: /batch <action> (plan, implement, review, submit, abort, reset, stop)", nil
		}
		resp, err := client.Call(ctx, "tasks.batch", json.RawMessage(mustJSON(map[string]string{"action": args})))
		if err != nil {
			return "", err
		}
		var result struct {
			Total   int `json:"total"`
			Results []struct {
				Success bool `json:"success"`
			} `json:"results"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		succeeded := 0
		for _, r := range result.Results {
			if r.Success {
				succeeded++
			}
		}

		return fmt.Sprintf("Batch %s: %d/%d succeeded.", args, succeeded, result.Total), nil

	case "/activity":
		resp, err := client.Call(ctx, "activity.query", json.RawMessage(mustJSON(map[string]int{"limit": 20})))
		if err != nil {
			return "", err
		}
		var result struct {
			Entries []struct {
				Method    string `json:"method"`
				Timestamp string `json:"timestamp"`
				Duration  int    `json:"duration_ms"`
			} `json:"entries"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		if len(result.Entries) == 0 {
			return "No activity.", nil
		}
		var actLines []string
		for _, e := range result.Entries {
			actLines = append(actLines, fmt.Sprintf("[%s] %s (%dms)", e.Timestamp, e.Method, e.Duration))
		}

		return strings.Join(actLines, "\n"), nil

	case "/report":
		resp, err := client.Call(ctx, "report.generate", nil)
		if err != nil {
			return "", err
		}
		var result struct {
			Report string `json:"report"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		if result.Report == "" {
			return "Report generated.", nil
		}

		return result.Report, nil

	case "/backup":
		resp, err := client.Call(ctx, "backup.create", nil)
		if err != nil {
			return "", err
		}
		var result struct {
			Path string `json:"path"`
		}
		_ = json.Unmarshal(resp.Result, &result)

		return "Backup created: " + result.Path, nil

	case "/access":
		resp, err := client.Call(ctx, "access.token.list", nil)
		if err != nil {
			return "", err
		}
		var result struct {
			Tokens []struct {
				ID      string `json:"id"`
				Name    string `json:"name"`
				Created string `json:"created"`
			} `json:"tokens"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		if len(result.Tokens) == 0 {
			return "No access tokens.", nil
		}
		var tokenLines []string
		for _, t := range result.Tokens {
			tokenLines = append(tokenLines, fmt.Sprintf("%s — %s (%s)", t.ID[:min(8, len(t.ID))], t.Name, t.Created))
		}

		return strings.Join(tokenLines, "\n"), nil

	case "/onboarding reset":
		_, err := client.Call(ctx, "onboarding.reset", nil)
		if err != nil {
			return "", err
		}

		return "Onboarding reset. The guide will show again on next visit.", nil

	case "/config check":
		resp, err := client.Call(ctx, "config.check", nil)
		if err != nil {
			return "", err
		}
		var result struct {
			Drifted bool `json:"drifted"`
			Diffs   []struct {
				Key      string `json:"key"`
				Expected string `json:"expected"`
				Actual   string `json:"actual"`
			} `json:"diffs"`
		}
		_ = json.Unmarshal(resp.Result, &result)
		if !result.Drifted {
			return "Configuration: no drift detected.", nil
		}
		var driftLines []string
		for _, d := range result.Diffs {
			driftLines = append(driftLines, fmt.Sprintf("  %s: expected=%s, actual=%s", d.Key, d.Expected, d.Actual))
		}

		return "Configuration drift:\n" + strings.Join(driftLines, "\n"), nil

	default:
		return "Unknown global command: " + name, nil
	}
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
