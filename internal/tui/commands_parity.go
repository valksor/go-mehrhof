package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/valksor/kvelmo/internal/socket"
)

// Handlers added to bring chat/TUI to parity with CLI commands that previously
// had no slash-command surface (agent, projects, hooks, recordings, screenshots,
// notify test).

func glAgent(ctx context.Context, client *socket.Client, _ string) (string, error) {
	resp, err := client.Call(ctx, "agent.status", nil)
	if err != nil {
		return "", fmt.Errorf("agent.status: %w", err)
	}
	var result struct {
		AgentAvailable bool `json:"agent_available"`
		SimulationMode bool `json:"simulation_mode"`
		Checks         []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
			Detail string `json:"detail"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	var lines []string
	switch {
	case result.AgentAvailable:
		lines = append(lines, "Agent: available")
	case result.SimulationMode:
		lines = append(lines, "Agent: simulation mode")
	default:
		lines = append(lines, "Agent: not available")
	}
	for _, c := range result.Checks {
		status := c.Status
		if status == "ok" || status == "pass" || status == "passed" {
			status = "OK"
		}
		line := fmt.Sprintf("  %s: %s", c.Name, status)
		if c.Detail != "" {
			line += " (" + c.Detail + ")"
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n"), nil
}

func glProjectsList(ctx context.Context, client *socket.Client, _ string) (string, error) {
	resp, err := client.Call(ctx, "projects.list", nil)
	if err != nil {
		return "", fmt.Errorf("projects.list: %w", err)
	}
	var result struct {
		Projects []struct {
			ID   string `json:"id"`
			Path string `json:"path"`
			Name string `json:"name"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if len(result.Projects) == 0 {
		return "No projects registered.", nil
	}
	var lines []string
	for _, p := range result.Projects {
		label := p.Name
		if label == "" {
			label = p.Path
		}
		idShort := p.ID
		if len(idShort) > 8 {
			idShort = idShort[:8]
		}
		lines = append(lines, fmt.Sprintf("%s %s", idShort, label))
	}

	return strings.Join(lines, "\n"), nil
}

func glProjectsUnregister(ctx context.Context, client *socket.Client, args string) (string, error) {
	id := strings.TrimSpace(args)
	if id == "" {
		return "Usage: /projects unregister <id>", nil
	}
	_, err := client.Call(ctx, "projects.unregister", json.RawMessage(mustJSON(map[string]string{"id": id})))
	if err != nil {
		return "", fmt.Errorf("projects.unregister: %w", err)
	}

	return fmt.Sprintf("Project %s unregistered.", id), nil
}

func glRecordings(ctx context.Context, client *socket.Client, _ string) (string, error) {
	resp, err := client.Call(ctx, "recordings.list", nil)
	if err != nil {
		return "", fmt.Errorf("recordings.list: %w", err)
	}
	var result struct {
		Recordings []struct {
			ID        string `json:"id"`
			Path      string `json:"path"`
			CreatedAt string `json:"created_at"`
		} `json:"recordings"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if len(result.Recordings) == 0 {
		return "No recordings.", nil
	}
	var lines []string
	for _, r := range result.Recordings {
		idShort := r.ID
		if len(idShort) > 8 {
			idShort = idShort[:8]
		}
		line := fmt.Sprintf("%s %s", idShort, r.Path)
		if r.CreatedAt != "" {
			line += " (" + r.CreatedAt + ")"
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n"), nil
}

func wtScreenshots(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	resp, err := client.Call(ctx, "screenshots.list", nil)
	if err != nil {
		return "", fmt.Errorf("screenshots.list: %w", err)
	}
	var result struct {
		Screenshots []struct {
			ID   string `json:"id"`
			Path string `json:"path"`
		} `json:"screenshots"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if len(result.Screenshots) == 0 {
		return "No screenshots.", nil
	}
	var lines []string
	for _, s := range result.Screenshots {
		idShort := s.ID
		if len(idShort) > 8 {
			idShort = idShort[:8]
		}
		lines = append(lines, fmt.Sprintf("%s %s", idShort, s.Path))
	}

	return strings.Join(lines, "\n"), nil
}

func glNotifyTest(ctx context.Context, client *socket.Client, _ string) (string, error) {
	resp, err := client.Call(ctx, "notify.test", nil)
	if err != nil {
		return "", fmt.Errorf("notify.test: %w", err)
	}
	var result struct {
		Sent    int    `json:"sent"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if result.Sent > 0 {
		return fmt.Sprintf("Notification test sent (%d delivered).", result.Sent), nil
	}
	if result.Message != "" {
		return "Notification test: " + result.Message, nil
	}

	return "Notification test complete.", nil
}

func wtHooks(ctx context.Context, client *socket.Client, _ string, _ bool) (string, error) {
	resp, err := client.Call(ctx, "hooks.list", nil)
	if err != nil {
		return "", fmt.Errorf("hooks.list: %w", err)
	}
	var result struct {
		Hooks []struct {
			Event   string `json:"event"`
			Name    string `json:"name"`
			Command string `json:"command"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if len(result.Hooks) == 0 {
		return "No hooks configured.", nil
	}
	var lines []string
	for _, h := range result.Hooks {
		label := h.Event
		if label == "" {
			label = h.Name
		}
		lines = append(lines, fmt.Sprintf("%s: %s", label, h.Command))
	}

	return strings.Join(lines, "\n"), nil
}
