package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/valksor/kvelmo/internal/socket"
)

// sendChatMessage returns a tea.Cmd that sends a chat message to the active worktree.
func (m *Model) sendChatMessage(text string) tea.Cmd {
	wt := m.activeWorktree()
	if wt == nil {
		return nil
	}
	dir := wt.Dir

	return func() tea.Msg {
		socketPath := socket.WorktreeSocketPath(dir)
		client, err := socket.NewClient(socketPath, socket.WithTimeout(5*time.Second))
		if err != nil {
			return errMsg{err: fmt.Errorf("chat: %w", err)}
		}
		defer func() { _ = client.Close() }()

		params, err := json.Marshal(map[string]string{"message": text})
		if err != nil {
			return errMsg{err: fmt.Errorf("marshal params: %w", err)}
		}
		if _, err := client.Call(m.ctx, "chat.send", json.RawMessage(params)); err != nil {
			return errMsg{err: fmt.Errorf("chat.send: %w", err)}
		}

		return nil
	}
}

// sendStartTask returns a tea.Cmd that starts a new task with the given description.
func (m *Model) sendStartTask(description string) tea.Cmd {
	wt := m.activeWorktree()
	if wt == nil {
		return nil
	}
	dir := wt.Dir

	return func() tea.Msg {
		socketPath := socket.WorktreeSocketPath(dir)
		client, err := socket.NewClient(socketPath, socket.WithTimeout(5*time.Second))
		if err != nil {
			return errMsg{err: fmt.Errorf("start: %w", err)}
		}
		defer func() { _ = client.Close() }()

		params, err := json.Marshal(map[string]string{keySource: description})
		if err != nil {
			return errMsg{err: fmt.Errorf("marshal params: %w", err)}
		}
		if _, err := client.Call(m.ctx, "start", json.RawMessage(params)); err != nil {
			return errMsg{err: fmt.Errorf("start: %w", err)}
		}

		return nil
	}
}

// specResultMsg carries the spec content fetched from the worktree socket.
type specResultMsg struct {
	content string
}

// fetchSpec returns a tea.Cmd that fetches the current specification and displays it in the viewport.
func (m *Model) fetchSpec() tea.Cmd {
	wt := m.activeWorktree()
	if wt == nil {
		return nil
	}
	dir := wt.Dir

	return func() tea.Msg {
		socketPath := socket.WorktreeSocketPath(dir)
		client, err := socket.NewClient(socketPath, socket.WithTimeout(5*time.Second))
		if err != nil {
			return errMsg{err: fmt.Errorf("show.spec: %w", err)}
		}
		defer func() { _ = client.Close() }()

		resp, err := client.Call(m.ctx, "show.spec", nil)
		if err != nil {
			return errMsg{err: fmt.Errorf("show.spec: %w", err)}
		}

		var result struct {
			Specifications []struct {
				Content string `json:"content"`
			} `json:"specifications"`
		}
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			return errMsg{err: fmt.Errorf("parse spec: %w", err)}
		}

		var content string
		for _, spec := range result.Specifications {
			if content != "" {
				content += "\n---\n\n"
			}
			content += spec.Content
		}
		if content == "" {
			content = "No specification available."
		}

		return specResultMsg{content: content}
	}
}

// sendWorkflowCmd returns a tea.Cmd that calls a workflow RPC on the active worktree.
func (m *Model) sendWorkflowCmd(method string) tea.Cmd {
	wt := m.activeWorktree()
	if wt == nil {
		return nil
	}
	dir := wt.Dir
	dryRun := m.dryRun

	return func() tea.Msg {
		socketPath := socket.WorktreeSocketPath(dir)
		client, err := socket.NewClient(socketPath, socket.WithTimeout(5*time.Second))
		if err != nil {
			return errMsg{err: fmt.Errorf("%s: %w", method, err)}
		}
		defer func() { _ = client.Close() }()

		var params any
		if dryRun {
			params = map[string]any{"dry_run": true}
		}

		if _, err := client.Call(m.ctx, method, params); err != nil {
			return errMsg{err: fmt.Errorf("%s: %w", method, err)}
		}

		return nil
	}
}

// changelogResultMsg carries the generated changelog content.
type changelogResultMsg struct {
	content string
}

// fetchChangelog returns a tea.Cmd that calls changelog.generate on the active worktree.
// The input should be "source..target [note]" format.
func (m *Model) fetchChangelog(input string, full bool) tea.Cmd {
	wt := m.activeWorktree()
	if wt == nil {
		return nil
	}
	dir := wt.Dir

	// Parse "source..target [note]" input.
	refPart, note, _ := strings.Cut(input, " ")
	parts := strings.SplitN(refPart, "..", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return func() tea.Msg {
			return errMsg{err: fmt.Errorf("invalid changelog range %q — use source..target format", input)}
		}
	}
	source, target := parts[0], parts[1]
	note = strings.TrimSpace(note)

	return func() tea.Msg {
		socketPath := socket.WorktreeSocketPath(dir)
		client, err := socket.NewClient(socketPath, socket.WithTimeout(30*time.Second))
		if err != nil {
			return errMsg{err: fmt.Errorf("changelog: %w", err)}
		}
		defer func() { _ = client.Close() }()

		p := map[string]any{
			keySource: source,
			"target":  target,
			"full":    full,
		}
		if note != "" {
			p["note"] = note
		}
		params, err := json.Marshal(p)
		if err != nil {
			return errMsg{err: fmt.Errorf("marshal params: %w", err)}
		}

		resp, err := client.Call(m.ctx, "changelog.generate", json.RawMessage(params))
		if err != nil {
			return errMsg{err: fmt.Errorf("changelog.generate: %w", err)}
		}

		var result struct {
			Markdown string `json:"markdown"`
		}
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			return errMsg{err: fmt.Errorf("parse changelog: %w", err)}
		}

		content := result.Markdown
		if content == "" {
			content = fmt.Sprintf("No commits between %s and %s", source, target)
		}

		return changelogResultMsg{content: content}
	}
}

// parseAutoFixAttempt extracts the "attempt" field from event data JSON.
func parseAutoFixAttempt(data json.RawMessage) int {
	var d struct {
		Attempt int `json:"attempt"`
	}
	if err := json.Unmarshal(data, &d); err != nil {
		return 0
	}

	return d.Attempt
}

// parseAutoFixMax extracts the "max_attempts" field from event data JSON.
func parseAutoFixMax(data json.RawMessage) int {
	var d struct {
		MaxAttempts int `json:"max_attempts"`
	}
	if err := json.Unmarshal(data, &d); err != nil {
		return 0
	}

	return d.MaxAttempts
}
