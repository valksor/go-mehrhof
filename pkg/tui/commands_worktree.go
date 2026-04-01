package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/valksor/kvelmo/pkg/socket"
)

// worktreeHandler handles a worktree-scoped slash command.
type worktreeHandler func(ctx context.Context, client *socket.Client, args string, dryRun bool) (string, error)

// worktreeHandlers maps command names to their handler functions.
var worktreeHandlers = map[string]worktreeHandler{
	// Workflow
	"/quick":      wtQuick,
	"/plan":       wtPlan,
	"/plan!":      wtPlan,
	"/implement":  wtImplement,
	"/implement!": wtImplement,
	"/simplify":   wtSimplify,
	"/optimize":   wtOptimize,
	"/review":     wtReview,
	"/review fix": wtReviewFix,
	"/submit":     wtSubmit,
	"/finish":     wtFinish,
	"/abandon":    wtAbandon,
	"/delete":     wtDelete,

	// Control
	"/undo":   wtUndo,
	"/redo":   wtRedo,
	"/stop":   wtStop,
	"/abort":  wtAbort,
	"/reset":  wtReset,
	"/update": wtUpdate,

	// Inspection
	"/status":           wtStatus,
	"/checkpoints":      wtCheckpoints,
	"/checkpoints goto": wtCheckpointsGoto,
	"/recap":            wtRecap,
	"/diff":             wtDiff,
	"/show spec":        wtShowSpec,
	"/show plan":        wtShowPlan,
	"/list search":      wtListSearch,
	"/list":             wtList,
	"/eventlog":         wtEventlog,

	// Tags
	"/tag add":    wtTagAdd,
	"/tag remove": wtTagRemove,
	"/tags":       wtTags,

	// Queue
	"/queue add":     wtQueueAdd,
	"/queue remove":  wtQueueRemove,
	"/queue reorder": wtQueueReorder,
	"/queue list":    wtQueueList,
	"/queue":         wtQueueList,

	// Forks
	"/fork create":  wtForkCreate,
	"/fork list":    wtForkList,
	"/fork compare": wtForkCompare,
	"/fork select":  wtForkSelect,

	// Governance
	"/approve":           wtApprove,
	"/checklist check":   wtChecklistCheck,
	"/checklist uncheck": wtChecklistUncheck,
	"/checklist":         wtChecklist,
	"/ci":                wtCI,
	"/policy":            wtPolicy,
	"/quality":           wtQuality,
	"/retry":             wtRetry,
	"/audit":             wtAudit,

	// Files & Code
	"/files search":      wtFilesSearch,
	"/files":             wtFiles,
	"/git status":        wtGitStatus,
	"/git log":           wtGitLog,
	"/codegraph callers": wtCodegraphCallers,
	"/codegraph deps":    wtCodegraphDeps,
	"/codegraph index":   wtCodegraphIndex,
	"/codegraph stats":   wtCodegraphStats,
	"/codegraph search":  wtCodegraphSearch,

	// Cache
	"/cache stats": wtCacheStats,
	"/cache clear": wtCacheClear,

	// Export
	"/export": wtExport,

	// Changelog
	"/changelog":      wtChangelog,
	"/changelog full": wtChangelogFull,

	// Remote
	"/remote approve": wtRemoteApprove,
	"/remote merge":   wtRemoteMerge,

	// Discover
	"/discover": wtDiscover,

	// Chat-based
	"/explain": wtExplain,
}

// ── Workflow ────────────────────────────────────────────────────────────────

func wtQuick(ctx context.Context, client *socket.Client, args string, _ bool) (string, error) {
	if args == "" {
		return "Usage: /quick <source>", nil
	}
	_, err := client.Call(ctx, "start", json.RawMessage(mustJSON(map[string]any{"source": args, "auto_advance": true})))
	if err != nil {
		return "", err
	}

	return "Quick fix started.", nil
}

func wtPlan(ctx context.Context, client *socket.Client, _ string, dryRun bool) (string, error) {
	params := map[string]any{}
	if dryRun {
		params["dry_run"] = true
	}
	_, err := client.Call(ctx, "plan", json.RawMessage(mustJSON(params)))
	if err != nil {
		return "", err
	}

	return "Planning started.", nil
}

func wtImplement(ctx context.Context, client *socket.Client, _ string, dryRun bool) (string, error) {
	params := map[string]any{}
	if dryRun {
		params["dry_run"] = true
	}
	_, err := client.Call(ctx, "implement", json.RawMessage(mustJSON(params)))
	if err != nil {
		return "", err
	}

	return "Implementation started.", nil
}

func wtSimplify(ctx context.Context, client *socket.Client, _ string, dryRun bool) (string, error) {
	_, err := client.Call(ctx, "simplify", optDryRun(dryRun))
	if err != nil {
		return "", err
	}

	return "Simplification started.", nil
}

func wtOptimize(ctx context.Context, client *socket.Client, _ string, dryRun bool) (string, error) {
	_, err := client.Call(ctx, "optimize", optDryRun(dryRun))
	if err != nil {
		return "", err
	}

	return "Optimization started.", nil
}

func wtReview(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	_, err := client.Call(ctx, "review", json.RawMessage(mustJSON(map[string]any{"approve": true})))
	if err != nil {
		return "", err
	}

	return "Review started.", nil
}

func wtReviewFix(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	_, err := client.Call(ctx, "review", json.RawMessage(mustJSON(map[string]any{"fix": true})))
	if err != nil {
		return "", err
	}

	return "Review with fixes started.", nil
}

func wtSubmit(ctx context.Context, client *socket.Client, _ string, dryRun bool) (string, error) {
	_, err := client.Call(ctx, "submit", optDryRun(dryRun))
	if err != nil {
		return "", err
	}

	return "Submit started.", nil
}

func wtFinish(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	_, err := client.Call(ctx, "task.finish", nil)
	if err != nil {
		return "", err
	}

	return "Finish started.", nil
}

func wtAbandon(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	_, err := client.Call(ctx, "abandon", nil)
	if err != nil {
		return "", err
	}

	return "Task abandoned.", nil
}

func wtDelete(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	_, err := client.Call(ctx, "delete", nil)
	if err != nil {
		return "", err
	}

	return "Task deleted.", nil
}

// ── Control ─────────────────────────────────────────────────────────────────

func wtUndo(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	_, err := client.Call(ctx, "undo", nil)
	if err != nil {
		return "", err
	}

	return "Undone to previous checkpoint.", nil
}

func wtRedo(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	_, err := client.Call(ctx, "redo", nil)
	if err != nil {
		return "", err
	}

	return "Redone to next checkpoint.", nil
}

func wtStop(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	_, err := client.Call(ctx, "stop", nil)
	if err != nil {
		return "", err
	}

	return "Operation stopped.", nil
}

func wtAbort(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	_, err := client.Call(ctx, "abort", nil)
	if err != nil {
		return "", err
	}

	return "Operation aborted.", nil
}

func wtReset(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	_, err := client.Call(ctx, "reset", nil)
	if err != nil {
		return "", err
	}

	return "Task reset.", nil
}

func wtUpdate(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
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
}

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

// ── Governance ──────────────────────────────────────────────────────────────

func wtApprove(ctx context.Context, client *socket.Client, args string, _ bool) (string, error) {
	event := strings.TrimSpace(args)
	if event == "" {
		return "Usage: /approve <event> (e.g. submit, implement)", nil
	}
	_, err := client.Call(ctx, "approve", json.RawMessage(mustJSON(map[string]string{"event": event})))
	if err != nil {
		return "", err
	}

	return "Approved: " + event, nil
}

func wtChecklistCheck(ctx context.Context, client *socket.Client, args string, _ bool) (string, error) {
	item := strings.TrimSpace(args)
	if item == "" {
		return "Usage: /checklist check <item-name>", nil
	}
	_, err := client.Call(ctx, "review.checklist.check", json.RawMessage(mustJSON(map[string]string{"item": item})))
	if err != nil {
		return "", err
	}

	return "Checked: " + item, nil
}

func wtChecklistUncheck(ctx context.Context, client *socket.Client, args string, _ bool) (string, error) {
	item := strings.TrimSpace(args)
	if item == "" {
		return "Usage: /checklist uncheck <item-name>", nil
	}
	_, err := client.Call(ctx, "review.checklist.uncheck", json.RawMessage(mustJSON(map[string]string{"item": item})))
	if err != nil {
		return "", err
	}

	return "Unchecked: " + item, nil
}

func wtChecklist(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	resp, err := client.Call(ctx, "review.checklist.get", nil)
	if err != nil {
		return "", err
	}
	var result struct {
		Required []string `json:"required"`
		Checked  []string `json:"checked"`
	}
	_ = json.Unmarshal(resp.Result, &result)
	if len(result.Required) == 0 {
		return "No checklist items.", nil
	}
	checkedSet := make(map[string]bool, len(result.Checked))
	for _, c := range result.Checked {
		checkedSet[c] = true
	}
	var lines []string
	for i, item := range result.Required {
		mark := "☐"
		if checkedSet[item] {
			mark = "✓"
		}
		lines = append(lines, fmt.Sprintf("%s %d. %s", mark, i+1, item))
	}

	return strings.Join(lines, "\n"), nil
}

func wtCI(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
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
}

func wtPolicy(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
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
}

func wtQuality(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	resp, err := client.Call(ctx, "quality.respond", json.RawMessage(mustJSON(map[string]string{"action": "run"})))
	if err != nil {
		return "", err
	}
	var result struct {
		Status string `json:"status"`
	}
	_ = json.Unmarshal(resp.Result, &result)

	return "Quality: " + result.Status, nil
}

func wtRetry(ctx context.Context, client *socket.Client, _ string, dryRun bool) (string, error) {
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
		return "", fmt.Errorf("reset to %s succeeded but %s failed: %w", resetResult.State, phase, err)
	}

	return fmt.Sprintf("Retrying: reset to %s, re-running %s.", resetResult.State, phase), nil
}

func wtAudit(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	resp, err := client.Call(ctx, "task.export", json.RawMessage(mustJSON(map[string]string{"format": "audit"})))
	if err != nil {
		return "", err
	}

	var result struct {
		Entries []struct {
			Action    string `json:"action"`
			Timestamp string `json:"timestamp"`
			Details   string `json:"details"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return string(resp.Result), nil //nolint:nilerr // Fallback to raw JSON on parse failure
	}
	if len(result.Entries) == 0 {
		return "No audit entries.", nil
	}
	var lines []string
	for _, e := range result.Entries {
		line := fmt.Sprintf("[%s] %s", e.Timestamp, e.Action)
		if e.Details != "" {
			line += " — " + e.Details
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n"), nil
}

// ── Files & Code ────────────────────────────────────────────────────────────

func wtFilesSearch(ctx context.Context, client *socket.Client, args string, _ bool) (string, error) {
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
}

func wtFiles(ctx context.Context, client *socket.Client, args string, _ bool) (string, error) {
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
}

func wtGitStatus(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
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
}

func wtGitLog(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
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
}

func wtCodegraphCallers(ctx context.Context, client *socket.Client, args string, _ bool) (string, error) {
	if args == "" {
		return "Usage: /codegraph callers <symbol>", nil
	}
	resp, err := client.Call(ctx, "codegraph.callers", json.RawMessage(mustJSON(map[string]string{"name": args})))
	if err != nil {
		return "", err
	}
	var result struct {
		Callers []struct {
			Name string `json:"name"`
			File string `json:"file"`
			Line int    `json:"line"`
		} `json:"callers"`
	}
	_ = json.Unmarshal(resp.Result, &result)
	if len(result.Callers) == 0 {
		return "No callers found.", nil
	}
	var lines []string
	for _, c := range result.Callers {
		lines = append(lines, fmt.Sprintf("%s — %s:%d", c.Name, c.File, c.Line))
	}

	return strings.Join(lines, "\n"), nil
}

func wtCodegraphDeps(ctx context.Context, client *socket.Client, args string, _ bool) (string, error) {
	if args == "" {
		return "Usage: /codegraph deps <symbol>", nil
	}
	resp, err := client.Call(ctx, "codegraph.deps", json.RawMessage(mustJSON(map[string]string{"name": args})))
	if err != nil {
		return "", err
	}
	var result struct {
		Deps []struct {
			Name string `json:"name"`
			Kind string `json:"kind"`
			File string `json:"file"`
		} `json:"deps"`
	}
	_ = json.Unmarshal(resp.Result, &result)
	if len(result.Deps) == 0 {
		return "No dependencies found.", nil
	}
	var lines []string
	for _, d := range result.Deps {
		lines = append(lines, fmt.Sprintf("%s %s — %s", d.Kind, d.Name, d.File))
	}

	return strings.Join(lines, "\n"), nil
}

func wtCodegraphIndex(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	resp, err := client.Call(ctx, "codegraph.index", nil)
	if err != nil {
		return "", err
	}
	var result struct {
		Symbols int `json:"symbols"`
	}
	_ = json.Unmarshal(resp.Result, &result)

	return fmt.Sprintf("Indexed %d symbols.", result.Symbols), nil
}

func wtCodegraphStats(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	resp, err := client.Call(ctx, "codegraph.stats", nil)
	if err != nil {
		return "", err
	}

	return string(resp.Result), nil
}

func wtCodegraphSearch(ctx context.Context, client *socket.Client, args string, _ bool) (string, error) {
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
	resp, err := client.Call(ctx, "project.discover", nil)
	if err != nil {
		return "", err
	}

	return string(resp.Result), nil
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
