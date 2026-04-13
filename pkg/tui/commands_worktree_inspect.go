package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/valksor/kvelmo/pkg/socket"
)

// ── Inspection ──────────────────────────────────────────────────────────────

func wtStatus(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
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
}

func wtCheckpoints(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
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
}

func wtCheckpointsGoto(ctx context.Context, client *socket.Client, args string, _ bool) (string, error) {
	if args == "" {
		return "Usage: /checkpoints goto <sha>", nil
	}
	_, err := client.Call(ctx, "checkpoint.goto", json.RawMessage(mustJSON(map[string]string{"sha": args})))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Jumped to checkpoint %s.", args[:min(7, len(args))]), nil
}

func wtRecap(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
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
}

func wtDiff(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
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
}

func wtShowSpec(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	return wtShowContent(ctx, client, "show.spec")
}

func wtShowPlan(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	return wtShowContent(ctx, client, "show.plan")
}

func wtShowContent(ctx context.Context, client *socket.Client, method string) (string, error) {
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
}

func wtListSearch(ctx context.Context, client *socket.Client, args string, _ bool) (string, error) {
	if args == "" {
		return "Usage: /list search <query>", nil
	}
	resp, err := client.Call(ctx, "task.search", json.RawMessage(mustJSON(map[string]string{"query": args})))
	if err != nil {
		return "", err
	}

	return formatTaskList(resp.Result), nil
}

func wtList(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	resp, err := client.Call(ctx, "task.history", nil)
	if err != nil {
		return "", err
	}

	return formatTaskList(resp.Result), nil
}

func wtEventlog(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
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
}

// ── Tags ────────────────────────────────────────────────────────────────────

func wtTagAdd(ctx context.Context, client *socket.Client, args string, _ bool) (string, error) {
	if args == "" {
		return "Usage: /tag add <name>", nil
	}
	_, err := client.Call(ctx, "task.tag", json.RawMessage(mustJSON(map[string]string{"action": "add", "tag": args})))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Tag %q added.", args), nil
}

func wtTagRemove(ctx context.Context, client *socket.Client, args string, _ bool) (string, error) {
	if args == "" {
		return "Usage: /tag remove <name>", nil
	}
	_, err := client.Call(ctx, "task.tag", json.RawMessage(mustJSON(map[string]string{"action": "remove", "tag": args})))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Tag %q removed.", args), nil
}

func wtTags(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
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
}

// ── Queue ───────────────────────────────────────────────────────────────────

func wtQueueAdd(ctx context.Context, client *socket.Client, args string, _ bool) (string, error) {
	if args == "" {
		return "Usage: /queue add <source>", nil
	}
	_, err := client.Call(ctx, "queue.add", json.RawMessage(mustJSON(map[string]string{"source": args})))
	if err != nil {
		return "", err
	}

	return "Queued: " + args, nil
}

func wtQueueRemove(ctx context.Context, client *socket.Client, args string, _ bool) (string, error) {
	if args == "" {
		return "Usage: /queue remove <id>", nil
	}
	_, err := client.Call(ctx, "queue.remove", json.RawMessage(mustJSON(map[string]string{"id": args})))
	if err != nil {
		return "", err
	}

	return "Removed from queue.", nil
}

func wtQueueReorder(ctx context.Context, client *socket.Client, args string, _ bool) (string, error) {
	parts := strings.SplitN(args, " ", 2)
	if len(parts) != 2 {
		return "Usage: /queue reorder <id> <position>", nil
	}
	position, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return "Position must be a number.", nil //nolint:nilerr // user input validation
	}
	_, err = client.Call(ctx, "queue.reorder", json.RawMessage(mustJSON(map[string]any{"id": parts[0], "position": position})))
	if err != nil {
		return "", fmt.Errorf("queue reorder: %w", err)
	}

	return fmt.Sprintf("Moved %s to position %d.", parts[0][:min(8, len(parts[0]))], position), nil
}

func wtQueueList(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
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
}

// ── Forks ───────────────────────────────────────────────────────────────────

func wtForkCreate(ctx context.Context, client *socket.Client, args string, _ bool) (string, error) {
	if args == "" {
		return "Usage: /fork create <label>", nil
	}
	_, err := client.Call(ctx, "fork.create", json.RawMessage(mustJSON(map[string]string{"label": args})))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Fork %q created.", args), nil
}

func wtForkList(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
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
}

func wtForkCompare(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	resp, err := client.Call(ctx, "fork.compare", nil)
	if err != nil {
		return "", err
	}

	return string(resp.Result), nil
}

func wtForkSelect(ctx context.Context, client *socket.Client, args string, _ bool) (string, error) {
	if args == "" {
		return "Usage: /fork select <fork-id>", nil
	}
	_, err := client.Call(ctx, "fork.select", json.RawMessage(mustJSON(map[string]string{"fork_id": args})))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Switched to fork %s.", args[:min(8, len(args))]), nil
}

// ── Cache ───────────────────────────────────────────────────────────────────

func wtCacheStats(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	resp, err := client.Call(ctx, "cache.stats", nil)
	if err != nil {
		return "", err
	}

	return string(resp.Result), nil
}

func wtCacheClear(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	_, err := client.Call(ctx, "cache.clear", nil)
	if err != nil {
		return "", err
	}

	return "Cache cleared.", nil
}

// ── Export ──────────────────────────────────────────────────────────────────

func wtExport(ctx context.Context, client *socket.Client, args string, _ bool) (string, error) {
	format := strings.TrimSpace(args)
	if format == "" {
		format = "json"
	}
	resp, err := client.Call(ctx, "task.export", json.RawMessage(mustJSON(map[string]string{"format": format})))
	if err != nil {
		return "", err
	}
	var result struct {
		Data string `json:"data"`
		Path string `json:"path"`
	}
	_ = json.Unmarshal(resp.Result, &result)
	if result.Path != "" {
		return "Exported to " + result.Path, nil
	}
	if result.Data != "" {
		return result.Data, nil
	}

	return "Export complete.", nil
}

// ── Changelog ───────────────────────────────────────────────────────────────

func wtChangelog(ctx context.Context, client *socket.Client, args string, _ bool) (string, error) {
	return changelogImpl(ctx, client, args, false)
}

func wtChangelogFull(ctx context.Context, client *socket.Client, args string, _ bool) (string, error) {
	return changelogImpl(ctx, client, args, true)
}

func changelogImpl(ctx context.Context, client *socket.Client, args string, full bool) (string, error) {
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
	if full {
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
}

// ── Remote ──────────────────────────────────────────────────────────────────

func wtRemoteApprove(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	_, err := client.Call(ctx, "remote.approve", json.RawMessage(mustJSON(map[string]string{"comment": ""})))
	if err != nil {
		return "", err
	}

	return "PR approved.", nil
}

func wtRemoteMerge(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	_, err := client.Call(ctx, "remote.merge", json.RawMessage(mustJSON(map[string]string{"method": "rebase"})))
	if err != nil {
		return "", err
	}

	return "PR merged.", nil
}

// ── Discover ────────────────────────────────────────────────────────────────

func wtDiscover(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	resp, err := client.Call(ctx, "discovery.scan", nil)
	if err != nil {
		return "", err
	}

	var result struct {
		Commands []string `json:"commands"`
		Count    int      `json:"count"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("parse result: %w", err)
	}

	if result.Count == 0 {
		return "No project commands discovered.", nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Discovered commands (%d)\n\n", result.Count)
	for _, cmd := range result.Commands {
		fmt.Fprintf(&sb, "  %s\n", cmd)
	}

	return strings.TrimRight(sb.String(), "\n"), nil
}

// ── Chat-based ──────────────────────────────────────────────────────────────

func wtExplain(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	_, err := client.Call(ctx, "chat.send", json.RawMessage(mustJSON(map[string]string{
		"message": "Explain what you did in the last action, why you made those choices, and any assumptions or constraints you encountered.",
	})))
	if err != nil {
		return "", err
	}

	return "Asking agent to explain...", nil
}
