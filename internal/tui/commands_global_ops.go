package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/valksor/kvelmo/internal/socket"
)

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

func glConfigCheck(ctx context.Context, client *socket.Client, _ string) (string, error) {
	resp, err := client.Call(ctx, "config.check", nil)
	if err != nil {
		return "", fmt.Errorf("config check: %w", err)
	}
	var result struct {
		Drifts []struct {
			Path     string `json:"path"`
			Expected any    `json:"expected"`
			Actual   any    `json:"actual"`
		} `json:"drifts"`
		Count int `json:"count"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if result.Count == 0 {
		return "Configuration: no drift detected.", nil
	}
	var lines []string
	for _, d := range result.Drifts {
		lines = append(lines, fmt.Sprintf("  %s: expected=%v, actual=%v", d.Path, d.Expected, d.Actual))
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
	resp, err := client.Call(ctx, "catalog.get", map[string]any{keyName: name})
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

// ── Worker Management ──────────────────────────────────────────────────────

func glWorkersAdd(ctx context.Context, client *socket.Client, args string) (string, error) {
	name := strings.TrimSpace(args)
	if name == "" {
		return "Usage: /workers add <agent-name>", nil
	}
	_, err := client.Call(ctx, "workers.add", json.RawMessage(mustJSON(map[string]string{"agent": name})))
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
	_, err := client.Call(ctx, "workers.remove", json.RawMessage(mustJSON(map[string]string{keyName: name})))
	if err != nil {
		return "", fmt.Errorf("workers remove: %w", err)
	}

	return fmt.Sprintf("Worker %q removed.", name), nil
}

// ── Logs ────────────────────────────────────────────────────────────────────

func glRPCLog(ctx context.Context, client *socket.Client, _ string) (string, error) {
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
