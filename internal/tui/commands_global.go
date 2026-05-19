package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/valksor/kvelmo/internal/socket"
)

// globalHandler handles a global-scoped slash command.
type globalHandler func(ctx context.Context, client *socket.Client, args string) (string, error)

// globalHandlers maps command names to their handler functions.
var globalHandlers = map[string]globalHandler{
	"/jobs":            glJobs,
	"/stats":           glStats,
	"/workers":         glWorkers,
	"/memory search":   glMemorySearch,
	"/memory stats":    glMemoryStats,
	"/memory clear":    glMemoryClear,
	"/group create":    glGroupCreate,
	"/group list":      glGroupList,
	"/group status":    glGroupStatus,
	"/group add":       glGroupAdd,
	"/group submit":    glGroupSubmit,
	"/group remove":    glGroupRemove,
	"/batch":           glBatch,
	"/report":          glReport,
	"/backup":          glBackup,
	"/restore":         glRestore,
	"/rpc-log":         glRPCLog,
	"/workers add":     glWorkersAdd,
	"/workers remove":  glWorkersRemove,
	"/diagnose":        glDiagnose,
	"/security scan":   glSecurityScan,
	"/config check":    glConfigCheck,
	"/config show":     glConfigShow,
	"/config validate": glConfigValidate,
	"/strategy":        glStrategy,
	"/catalog list":    glCatalogList,
	"/catalog use":     glCatalogUse,
	// Parity additions — surface CLI-only commands in chat/TUI.
	"/agent":               glAgent,
	"/projects":            glProjectsList,
	"/projects unregister": glProjectsUnregister,
	"/recordings":          glRecordings,
	"/notify test":         glNotifyTest,
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

// ── Batch & Activity ────────────────────────────────────────────────────────

func glBatch(ctx context.Context, client *socket.Client, args string) (string, error) {
	if args == "" {
		return "Usage: /batch <action> (plan, implement, review, submit, abort, reset, stop)", nil
	}
	resp, err := client.Call(ctx, "tasks.batch", json.RawMessage(mustJSON(map[string]string{keyAction: args})))
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
