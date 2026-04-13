package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/valksor/kvelmo/internal/socket"
)

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
