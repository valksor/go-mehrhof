package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/valksor/kvelmo/pkg/socket"
)

// globalHandler handles a global-scoped slash command.
type globalHandler func(ctx context.Context, client *socket.Client, args string) (string, error)

// globalHandlers maps command names to their handler functions.
var globalHandlers = map[string]globalHandler{
	"/jobs":             glJobs,
	"/stats":            glStats,
	"/workers":          glWorkers,
	"/memory search":    glMemorySearch,
	"/memory stats":     glMemoryStats,
	"/memory clear":     glMemoryClear,
	"/group create":     glGroupCreate,
	"/group list":       glGroupList,
	"/group status":     glGroupStatus,
	"/group add":        glGroupAdd,
	"/group submit":     glGroupSubmit,
	"/group remove":     glGroupRemove,
	"/batch":            glBatch,
	"/activity":         glActivity,
	"/report":           glReport,
	"/backup":           glBackup,
	"/restore":          glRestore,
	"/access create":    glAccessCreate,
	"/access revoke":    glAccessRevoke,
	"/access":           glAccess,
	"/logs":             glLogs,
	"/workers add":      glWorkersAdd,
	"/workers remove":   glWorkersRemove,
	"/diagnose":         glDiagnose,
	"/security scan":    glSecurityScan,
	"/onboarding reset": glOnboardingReset,
	"/config check":     glConfigCheck,
	"/config show":      glConfigShow,
	"/config validate":  glConfigValidate,
	"/strategy":         glStrategy,
	"/catalog list":     glCatalogList,
	"/catalog use":      glCatalogUse,
}

// ── Jobs & Metrics ──────────────────────────────────────────────────────────

func glJobs(ctx context.Context, client *socket.Client, _ string) (string, error) {
	resp, err := client.Call(ctx, "jobs.list", nil)
	if err != nil {
		return "", fmt.Errorf("jobs list: %w", err)
	}
	var result struct {
		Jobs []struct {
			ID     string `json:"id"`
			Type   string `json:"type"`
			Status string `json:"status"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if len(result.Jobs) == 0 {
		return "No jobs.", nil
	}
	var lines []string
	for _, j := range result.Jobs {
		lines = append(lines, fmt.Sprintf("%s [%s] %s", j.ID[:min(8, len(j.ID))], j.Status, j.Type))
	}

	return strings.Join(lines, "\n"), nil
}

func glStats(ctx context.Context, client *socket.Client, _ string) (string, error) {
	resp, err := client.Call(ctx, "metrics", nil)
	if err != nil {
		return "", fmt.Errorf("metrics: %w", err)
	}

	return string(resp.Result), nil
}

func glWorkers(ctx context.Context, client *socket.Client, _ string) (string, error) {
	resp, err := client.Call(ctx, "workers.list", nil)
	if err != nil {
		return "", fmt.Errorf("workers list: %w", err)
	}
	var result struct {
		Workers []struct {
			Name  string `json:"name"`
			State string `json:"state"`
		} `json:"workers"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if len(result.Workers) == 0 {
		return "No workers.", nil
	}
	var lines []string
	for _, w := range result.Workers {
		lines = append(lines, fmt.Sprintf("%s [%s]", w.Name, w.State))
	}

	return strings.Join(lines, "\n"), nil
}

// ── Memory ──────────────────────────────────────────────────────────────────

func glMemorySearch(ctx context.Context, client *socket.Client, args string) (string, error) {
	if args == "" {
		return "Usage: /memory search <query>", nil
	}
	resp, err := client.Call(ctx, "memory.search", json.RawMessage(mustJSON(map[string]any{"query": args, "limit": 5})))
	if err != nil {
		return "", fmt.Errorf("memory search: %w", err)
	}
	var result struct {
		Results []struct {
			Content string  `json:"content"`
			Score   float64 `json:"score"`
		} `json:"results"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if len(result.Results) == 0 {
		return "No results.", nil
	}
	var lines []string
	for i, r := range result.Results {
		lines = append(lines, fmt.Sprintf("%d. (%.2f) %s", i+1, r.Score, r.Content))
	}

	return strings.Join(lines, "\n"), nil
}

func glMemoryStats(ctx context.Context, client *socket.Client, _ string) (string, error) {
	resp, err := client.Call(ctx, "memory.stats", nil)
	if err != nil {
		return "", fmt.Errorf("memory stats: %w", err)
	}

	return string(resp.Result), nil
}

func glMemoryClear(ctx context.Context, client *socket.Client, _ string) (string, error) {
	_, err := client.Call(ctx, "memory.clear", nil)
	if err != nil {
		return "", fmt.Errorf("memory clear: %w", err)
	}

	return "Memory cleared.", nil
}

// ── Task Groups ─────────────────────────────────────────────────────────────

func glGroupCreate(ctx context.Context, client *socket.Client, args string) (string, error) {
	if args == "" {
		return "Usage: /group create <label>", nil
	}
	resp, err := client.Call(ctx, "taskgroup.create", json.RawMessage(mustJSON(map[string]string{"label": args})))
	if err != nil {
		return "", fmt.Errorf("group create: %w", err)
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	return "Group created: " + result.ID, nil
}

func glGroupList(ctx context.Context, client *socket.Client, _ string) (string, error) {
	resp, err := client.Call(ctx, "taskgroup.list", nil)
	if err != nil {
		return "", fmt.Errorf("group list: %w", err)
	}
	var result struct {
		Groups []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if len(result.Groups) == 0 {
		return "No groups.", nil
	}
	var lines []string
	for _, g := range result.Groups {
		lines = append(lines, fmt.Sprintf("%s — %s", g.ID[:min(8, len(g.ID))], g.Label))
	}

	return strings.Join(lines, "\n"), nil
}

func glGroupStatus(ctx context.Context, client *socket.Client, args string) (string, error) {
	if args == "" {
		return "Usage: /group status <group-id>", nil
	}
	resp, err := client.Call(ctx, "taskgroup.status", json.RawMessage(mustJSON(map[string]string{"id": args})))
	if err != nil {
		return "", fmt.Errorf("group status: %w", err)
	}

	return string(resp.Result), nil
}

func glGroupAdd(ctx context.Context, client *socket.Client, args string) (string, error) {
	parts := strings.SplitN(args, " ", 2)
	if len(parts) < 2 {
		return "Usage: /group add <group-id> <task-id>", nil
	}
	_, err := client.Call(ctx, "taskgroup.add", json.RawMessage(mustJSON(map[string]string{"id": parts[0], "task_id": parts[1]})))
	if err != nil {
		return "", fmt.Errorf("group add: %w", err)
	}

	return fmt.Sprintf("Task added to group %s.", parts[0][:min(8, len(parts[0]))]), nil
}

func glGroupSubmit(ctx context.Context, client *socket.Client, args string) (string, error) {
	if args == "" {
		return "Usage: /group submit <group-id>", nil
	}
	_, err := client.Call(ctx, "taskgroup.submit", json.RawMessage(mustJSON(map[string]string{"id": args})))
	if err != nil {
		return "", fmt.Errorf("group submit: %w", err)
	}

	return fmt.Sprintf("Group %s submitted.", args[:min(8, len(args))]), nil
}

func glGroupRemove(ctx context.Context, client *socket.Client, args string) (string, error) {
	if args == "" {
		return "Usage: /group remove <group-id>", nil
	}
	_, err := client.Call(ctx, "taskgroup.remove", json.RawMessage(mustJSON(map[string]string{"id": args})))
	if err != nil {
		return "", fmt.Errorf("group remove: %w", err)
	}

	return fmt.Sprintf("Group %s removed.", args[:min(8, len(args))]), nil
}

// ── Batch & Activity ────────────────────────────────────────────────────────

func glBatch(ctx context.Context, client *socket.Client, args string) (string, error) {
	if args == "" {
		return "Usage: /batch <action> (plan, implement, review, submit, abort, reset, stop)", nil
	}
	resp, err := client.Call(ctx, "tasks.batch", json.RawMessage(mustJSON(map[string]string{"action": args})))
	if err != nil {
		return "", fmt.Errorf("batch: %w", err)
	}
	var result struct {
		Total   int `json:"total"`
		Results []struct {
			Success bool `json:"success"`
		} `json:"results"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	succeeded := 0
	for _, r := range result.Results {
		if r.Success {
			succeeded++
		}
	}

	return fmt.Sprintf("Batch %s: %d/%d succeeded.", args, succeeded, result.Total), nil
}

func glActivity(ctx context.Context, client *socket.Client, _ string) (string, error) {
	resp, err := client.Call(ctx, "activity.query", json.RawMessage(mustJSON(map[string]int{"limit": 20})))
	if err != nil {
		return "", fmt.Errorf("activity query: %w", err)
	}
	var result struct {
		Entries []struct {
			Method    string `json:"method"`
			Timestamp string `json:"timestamp"`
			Duration  int    `json:"duration_ms"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if len(result.Entries) == 0 {
		return "No activity.", nil
	}
	var lines []string
	for _, e := range result.Entries {
		lines = append(lines, fmt.Sprintf("[%s] %s (%dms)", e.Timestamp, e.Method, e.Duration))
	}

	return strings.Join(lines, "\n"), nil
}

// ── Reports & Backup ────────────────────────────────────────────────────────

func glReport(ctx context.Context, client *socket.Client, _ string) (string, error) {
	resp, err := client.Call(ctx, "report.generate", nil)
	if err != nil {
		return "", fmt.Errorf("report generate: %w", err)
	}
	var result struct {
		Report string `json:"report"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if result.Report == "" {
		return "Report generated.", nil
	}

	return result.Report, nil
}

func glBackup(ctx context.Context, client *socket.Client, _ string) (string, error) {
	resp, err := client.Call(ctx, "backup.create", nil)
	if err != nil {
		return "", fmt.Errorf("backup create: %w", err)
	}
	var result struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	return "Backup created: " + result.Path, nil
}

func glAccess(ctx context.Context, client *socket.Client, _ string) (string, error) {
	resp, err := client.Call(ctx, "access.token.list", nil)
	if err != nil {
		return "", fmt.Errorf("access token list: %w", err)
	}
	var result struct {
		Tokens []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Created string `json:"created"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if len(result.Tokens) == 0 {
		return "No access tokens.", nil
	}
	var lines []string
	for _, t := range result.Tokens {
		lines = append(lines, fmt.Sprintf("%s — %s (%s)", t.ID[:min(8, len(t.ID))], t.Name, t.Created))
	}

	return strings.Join(lines, "\n"), nil
}

// ── Diagnostics & Infrastructure ────────────────────────────────────────────

func glDiagnose(ctx context.Context, client *socket.Client, _ string) (string, error) {
	resp, err := client.Call(ctx, "system.diagnose", nil)
	if err != nil {
		return "", fmt.Errorf("diagnose: %w", err)
	}
	var result struct {
		Checks []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
			Detail string `json:"detail"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
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
}

func glSecurityScan(ctx context.Context, client *socket.Client, _ string) (string, error) {
	resp, err := client.Call(ctx, "security.scan", nil)
	if err != nil {
		return "", fmt.Errorf("security scan: %w", err)
	}
	var result struct {
		Issues []struct {
			Severity string `json:"severity"`
			Message  string `json:"message"`
		} `json:"issues"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if len(result.Issues) == 0 {
		return "No security issues found.", nil
	}
	var lines []string
	for _, i := range result.Issues {
		lines = append(lines, fmt.Sprintf("[%s] %s", i.Severity, i.Message))
	}

	return strings.Join(lines, "\n"), nil
}

func glOnboardingReset(ctx context.Context, client *socket.Client, _ string) (string, error) {
	_, err := client.Call(ctx, "onboarding.reset", nil)
	if err != nil {
		return "", fmt.Errorf("onboarding reset: %w", err)
	}

	return "Onboarding reset. The guide will show again on next visit.", nil
}

func glConfigCheck(ctx context.Context, client *socket.Client, _ string) (string, error) {
	resp, err := client.Call(ctx, "config.check", nil)
	if err != nil {
		return "", fmt.Errorf("config check: %w", err)
	}
	var result struct {
		Drifted bool `json:"drifted"`
		Diffs   []struct {
			Key      string `json:"key"`
			Expected string `json:"expected"`
			Actual   string `json:"actual"`
		} `json:"diffs"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if !result.Drifted {
		return "Configuration: no drift detected.", nil
	}
	var lines []string
	for _, d := range result.Diffs {
		lines = append(lines, fmt.Sprintf("  %s: expected=%s, actual=%s", d.Key, d.Expected, d.Actual))
	}

	return "Configuration drift:\n" + strings.Join(lines, "\n"), nil
}

func glConfigShow(ctx context.Context, client *socket.Client, _ string) (string, error) {
	resp, err := client.Call(ctx, "settings.get", nil)
	if err != nil {
		return "", fmt.Errorf("settings get: %w", err)
	}
	var result struct {
		Effective json.RawMessage `json:"effective"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	out, err := json.MarshalIndent(result.Effective, "", "  ")
	if err != nil {
		return string(result.Effective), fmt.Errorf("format config: %w", err)
	}

	return string(out), nil
}

func glConfigValidate(ctx context.Context, client *socket.Client, _ string) (string, error) {
	resp, err := client.Call(ctx, "config.validate", nil)
	if err != nil {
		return "", fmt.Errorf("config validate: %w", err)
	}
	var result struct {
		Valid  bool `json:"valid"`
		Checks []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
			Detail string `json:"detail"`
			Fix    string `json:"fix"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	var lines []string
	for _, c := range result.Checks {
		icon := "PASS"
		switch c.Status {
		case "warning":
			icon = "WARN"
		case "error":
			icon = "FAIL"
		}
		line := fmt.Sprintf("  [%s] %s", icon, c.Name)
		if c.Detail != "" {
			line += " — " + c.Detail
		}
		if c.Fix != "" && c.Status != "ok" {
			line += "\n         Fix: " + c.Fix
		}
		lines = append(lines, line)
	}
	header := "Configuration valid"
	if !result.Valid {
		header = "Configuration INVALID"
	}

	return header + ":\n" + strings.Join(lines, "\n"), nil
}

func glStrategy(ctx context.Context, client *socket.Client, _ string) (string, error) {
	resp, err := client.Call(ctx, "strategy.list", nil)
	if err != nil {
		return "", fmt.Errorf("strategy list: %w", err)
	}
	var names []string
	if err := json.Unmarshal(resp.Result, &names); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if len(names) == 0 {
		return "No strategies registered.", nil
	}
	var lines []string
	for _, n := range names {
		lines = append(lines, "  - "+n)
	}

	return "Available strategies:\n" + strings.Join(lines, "\n"), nil
}

func glRestore(ctx context.Context, client *socket.Client, args string) (string, error) {
	archivePath := strings.TrimSpace(args)
	if archivePath == "" {
		return "Usage: /restore <archive-path>", nil
	}
	_, err := client.Call(ctx, "backup.restore", map[string]any{"archive_path": archivePath})
	if err != nil {
		return "", fmt.Errorf("restore: %w", err)
	}

	return "Restored from " + archivePath + ".", nil
}

func glCatalogList(ctx context.Context, client *socket.Client, _ string) (string, error) {
	resp, err := client.Call(ctx, "catalog.list", nil)
	if err != nil {
		return "", fmt.Errorf("catalog list: %w", err)
	}
	var result struct {
		Templates []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"templates"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if len(result.Templates) == 0 {
		return "No templates in catalog.", nil
	}
	var lines []string
	for _, t := range result.Templates {
		lines = append(lines, fmt.Sprintf("  %s — %s", t.Name, t.Description))
	}

	return strings.Join(lines, "\n"), nil
}

func glCatalogUse(ctx context.Context, client *socket.Client, args string) (string, error) {
	name := strings.TrimSpace(args)
	if name == "" {
		return "Usage: /catalog use <template-name>", nil
	}
	resp, err := client.Call(ctx, "catalog.get", map[string]any{"name": name})
	if err != nil {
		return "", fmt.Errorf("catalog get: %w", err)
	}
	var tmpl struct {
		Name   string `json:"name"`
		Source string `json:"source"`
	}
	if err := json.Unmarshal(resp.Result, &tmpl); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if tmpl.Source == "" {
		return fmt.Sprintf("Template %q has no source configured.", name), nil
	}

	return fmt.Sprintf("Template %q (source: %s). Run from a project: /start %s", name, tmpl.Source, tmpl.Source), nil
}

// ── Access Token Management ────────────────────────────────────────────────

func glAccessCreate(ctx context.Context, client *socket.Client, args string) (string, error) {
	label := strings.TrimSpace(args)
	if label == "" {
		return "Usage: /access create <label>", nil
	}
	resp, err := client.Call(ctx, "access.token.create", json.RawMessage(mustJSON(map[string]string{"role": "operator", "label": label})))
	if err != nil {
		return "", fmt.Errorf("access token create: %w", err)
	}
	var result struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	return fmt.Sprintf("Token created:\n%s\n\nSave this token — it cannot be shown again.", result.Token), nil
}

func glAccessRevoke(ctx context.Context, client *socket.Client, args string) (string, error) {
	id := strings.TrimSpace(args)
	if id == "" {
		return "Usage: /access revoke <token-id>", nil
	}
	_, err := client.Call(ctx, "access.token.revoke", json.RawMessage(mustJSON(map[string]string{"id": id})))
	if err != nil {
		return "", fmt.Errorf("access token revoke: %w", err)
	}

	return fmt.Sprintf("Token %s revoked.", id), nil
}

// ── Worker Management ──────────────────────────────────────────────────────

func glWorkersAdd(ctx context.Context, client *socket.Client, args string) (string, error) {
	name := strings.TrimSpace(args)
	if name == "" {
		return "Usage: /workers add <agent-name>", nil
	}
	_, err := client.Call(ctx, "workers.add", json.RawMessage(mustJSON(map[string]string{"name": name})))
	if err != nil {
		return "", fmt.Errorf("workers add: %w", err)
	}

	return fmt.Sprintf("Worker %q added.", name), nil
}

func glWorkersRemove(ctx context.Context, client *socket.Client, args string) (string, error) {
	name := strings.TrimSpace(args)
	if name == "" {
		return "Usage: /workers remove <worker-name>", nil
	}
	_, err := client.Call(ctx, "workers.remove", json.RawMessage(mustJSON(map[string]string{"name": name})))
	if err != nil {
		return "", fmt.Errorf("workers remove: %w", err)
	}

	return fmt.Sprintf("Worker %q removed.", name), nil
}

// ── Logs ────────────────────────────────────────────────────────────────────

func glLogs(ctx context.Context, client *socket.Client, _ string) (string, error) {
	resp, err := client.Call(ctx, "activity.query", json.RawMessage(mustJSON(map[string]int{"limit": 20})))
	if err != nil {
		return "", fmt.Errorf("logs: %w", err)
	}
	var result struct {
		Entries []struct {
			Timestamp string `json:"timestamp"`
			Level     string `json:"level"`
			Method    string `json:"method"`
			Message   string `json:"message"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if len(result.Entries) == 0 {
		return "No log entries.", nil
	}
	var lines []string
	for _, e := range result.Entries {
		level := e.Level
		if level == "" {
			level = "INFO"
		}
		msg := e.Message
		if msg == "" {
			msg = e.Method
		}
		lines = append(lines, fmt.Sprintf("[%s] [%s] %s", e.Timestamp, level, msg))
	}

	return strings.Join(lines, "\n"), nil
}
