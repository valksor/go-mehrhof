package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/valksor/kvelmo/pkg/socket"
)

// Layout controls the TUI pane arrangement.
type Layout int

const (
	LayoutStacked   Layout = iota // status bar + output + chat (default)
	LayoutDashboard               // status bar + output + workers + chat
)

// WorkerInfo holds display state for one worker.
type WorkerInfo struct {
	Name  string
	State string // "available", "working", "disconnected"
	JobID string
}

// WorktreeState holds all display state for one active worktree.
type WorktreeState struct {
	Dir              string
	State            string   // task state: none/loaded/implementing/etc.
	Output           []string // accumulated output lines
	Workers          []WorkerInfo
	JobID            string
	LastFailureClass string // failure classification: hard_stop, recoverable, degraded, skippable
}

// Model is the root bubbletea model.
type Model struct {
	cwd       string
	worktrees []WorktreeState
	active    int // index into worktrees
	layout    Layout
	output    viewport.Model  // bubbles viewport for scrollable output
	chatInput textinput.Model // bubbles text input for chat
	msgs      chan tea.Msg    // fan-in channel from all socket connections
	//nolint:containedctx // Model owns its shutdown context for goroutine lifecycle management
	ctx       context.Context
	cancel    context.CancelFunc
	width     int
	height    int
	ready     bool
	showHelp  bool
	dryRun    bool
	startMode bool // when true, Enter sends task start instead of chat message
	err       error
}

// NewModel constructs a new TUI model with the given cwd and layout.
func NewModel(cwd string, layout Layout) Model {
	ctx, cancel := context.WithCancel(context.Background())
	msgs := make(chan tea.Msg, 64)

	ti := textinput.New()
	ti.Placeholder = "Chat with agent..."

	return Model{
		cwd:       cwd,
		layout:    layout,
		chatInput: ti,
		msgs:      msgs,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Init starts worktree discovery.
func (m *Model) Init() tea.Cmd {
	return discoverWorktrees(m.cwd)
}

// Update handles all incoming messages and key events.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.output.Width = msg.Width
		m.output.Height = m.outputHeight()
		m.chatInput.Width = msg.Width - 2

		return m, nil

	case worktreeListMsg:
		return m.handleWorktreeList(msg)

	case connectedMsg:
		// Connection established — no state change needed.
		return m, nil

	case disconnectedMsg:
		for i, wt := range m.worktrees {
			if wt.Dir == msg.worktreeDir {
				m.worktrees[i].State = "disconnected"

				break
			}
		}

		return m, nil

	case socketEventMsg:
		return m.handleSocketEvent(msg)

	case specResultMsg:
		m.output.SetContent(msg.content)
		m.output.GotoTop()

		return m, nil

	case errMsg:
		m.err = msg.err

		return m, nil
	}

	return m, nil
}

// handleKey processes keyboard input.
func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.cancel()

		return m, tea.Quit

	case "tab":
		if len(m.worktrees) > 0 {
			m.active = (m.active + 1) % len(m.worktrees)
			m.syncViewport()
		}

		return m, nil

	case "shift+tab":
		if len(m.worktrees) > 0 {
			m.active = (m.active - 1 + len(m.worktrees)) % len(m.worktrees)
			m.syncViewport()
		}

		return m, nil

	case "?":
		m.showHelp = !m.showHelp

		return m, nil

	case "enter":
		text := m.chatInput.Value()
		if text != "" {
			m.chatInput.SetValue("")
			if m.startMode {
				m.startMode = false
				m.chatInput.Placeholder = "Chat with agent..."

				return m, m.sendStartTask(text)
			}

			return m, m.sendChatMessage(text)
		}

		return m, nil

	case "p":
		if !m.chatInput.Focused() {
			return m, m.sendWorkflowCmd("plan")
		}

	case "i":
		if !m.chatInput.Focused() {
			return m, m.sendWorkflowCmd("implement")
		}

	case "s":
		if !m.chatInput.Focused() {
			return m, m.sendWorkflowCmd("stop")
		}

	case "u":
		if !m.chatInput.Focused() {
			return m, m.sendWorkflowCmd("undo")
		}

	case "r":
		if !m.chatInput.Focused() {
			return m, m.sendWorkflowCmd("redo")
		}

	case "v":
		if !m.chatInput.Focused() {
			return m, m.fetchSpec()
		}

	case "t":
		if !m.chatInput.Focused() {
			m.startMode = !m.startMode
			if m.startMode {
				m.chatInput.Placeholder = "Enter task description..."
			} else {
				m.chatInput.Placeholder = "Chat with agent..."
			}

			return m, nil
		}

	case "R":
		if !m.chatInput.Focused() {
			return m, m.sendWorkflowCmd("review")
		}

	case "S":
		if !m.chatInput.Focused() {
			return m, m.sendWorkflowCmd("submit")
		}

	case "d":
		if !m.chatInput.Focused() {
			m.dryRun = !m.dryRun

			return m, nil
		}

	case "ctrl+a":
		if !m.chatInput.Focused() {
			return m, m.sendWorkflowCmd("abort")
		}

	default:
		var cmd tea.Cmd
		m.chatInput, cmd = m.chatInput.Update(msg)

		return m, cmd
	}

	// Workflow key pressed while chat is focused — forward to text input
	var cmd tea.Cmd
	m.chatInput, cmd = m.chatInput.Update(msg)

	return m, cmd
}

// handleWorktreeList adds newly discovered worktrees and starts subscriptions.
func (m *Model) handleWorktreeList(msg worktreeListMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	for _, dir := range msg.dirs {
		if !m.hasWorktree(dir) {
			m.worktrees = append(m.worktrees, WorktreeState{Dir: dir})
			cmds = append(cmds, subscribeWorktree(m.ctx, dir, m.msgs))
		}
	}
	cmds = append(cmds, waitForMsg(m.ctx, m.msgs))

	return m, tea.Batch(cmds...)
}

// handleSocketEvent processes an event from a worktree socket stream.
func (m *Model) handleSocketEvent(msg socketEventMsg) (tea.Model, tea.Cmd) {
	for i, wt := range m.worktrees {
		if wt.Dir != msg.worktreeDir {
			continue
		}
		switch msg.event.Type {
		case "job_output", "context_attached":
			m.worktrees[i].Output = append(m.worktrees[i].Output, msg.event.Message)
			if m.active == i {
				m.syncViewport()
			}
		case "cache_hit":
			m.worktrees[i].Output = append(m.worktrees[i].Output, msg.event.Message+" [cached]")
			if m.active == i {
				m.syncViewport()
			}
		case "state_changed":
			m.worktrees[i].State = string(msg.event.State)
			if string(msg.event.State) != "failed" {
				m.worktrees[i].LastFailureClass = ""
			}
		case "phase_failure_classified":
			m.worktrees[i].LastFailureClass = string(msg.event.FailureClass)
		}

		break
	}

	return m, waitForMsg(m.ctx, m.msgs)
}

// hasWorktree returns true if the given dir is already tracked.
func (m *Model) hasWorktree(dir string) bool {
	for _, wt := range m.worktrees {
		if wt.Dir == dir {
			return true
		}
	}

	return false
}

// activeWorktree returns a pointer to the active WorktreeState, or nil.
func (m *Model) activeWorktree() *WorktreeState {
	if len(m.worktrees) == 0 {
		return nil
	}

	return &m.worktrees[m.active]
}

// syncViewport rebuilds the viewport content from the active worktree output.
func (m *Model) syncViewport() {
	if wt := m.activeWorktree(); wt != nil {
		m.output.SetContent(strings.Join(wt.Output, "\n"))
		m.output.GotoBottom()
	}
}

// outputHeight returns the number of lines available for the output viewport.
func (m *Model) outputHeight() int {
	reserved := 3 // status bar + chat divider + chat input
	if len(m.worktrees) > 1 {
		reserved++ // tab bar
	}
	if m.layout == LayoutDashboard {
		reserved += 3 // workers header + 2 worker rows
	}
	h := m.height - reserved
	if h < 1 {
		h = 1
	}

	return h
}

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

		params, err := json.Marshal(map[string]string{"source": description})
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
