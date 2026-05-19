package tui

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
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
	AutoFixAttempt   int    // current auto-fix attempt (0 = inactive)
	AutoFixMax       int    // max auto-fix attempts
	ActiveForks      int    // number of active conversation forks
	RiskLevel        string // "low", "medium", "high", or "" if not evaluated

	// Progress estimation during active phases.
	ProgressActive     bool
	ProgressPercent    float64
	ProgressETASeconds int
	ProgressCalibrated bool
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
	ctx           context.Context
	cancel        context.CancelFunc
	width         int
	height        int
	ready         bool
	showHelp      bool
	dryRun        bool
	startMode     bool // when true, Enter sends task start instead of chat message
	changelogMode bool // when true, Enter sends changelog generate (source..target)
	changelogFull bool // when true, changelog includes body text
	err           error
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
	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.output.SetWidth(msg.Width)
		m.output.SetHeight(m.outputHeight())
		m.chatInput.SetWidth(msg.Width - 2)

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

	case changelogResultMsg:
		m.output.SetContent(msg.content)
		m.output.GotoTop()

		return m, nil

	case commandResultMsg:
		if msg.output != "" {
			if wt := m.activeWorktree(); wt != nil {
				idx := m.active
				m.worktrees[idx].Output = append(m.worktrees[idx].Output, msg.output)
				m.syncViewport()
			}
		}

		return m, nil

	case progressMsg:
		for i, wt := range m.worktrees {
			if wt.Dir == msg.worktreeDir {
				m.worktrees[i].ProgressActive = msg.active
				m.worktrees[i].ProgressPercent = msg.percent
				m.worktrees[i].ProgressETASeconds = msg.etaSeconds
				m.worktrees[i].ProgressCalibrated = msg.calibrated

				break
			}
		}

		return m, nil

	case errMsg:
		m.err = msg.err

		return m, nil
	}

	return m, nil
}

// handleKey processes keyboard input.
func (m *Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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
			if m.changelogMode {
				full := m.changelogFull
				m.changelogMode = false
				m.changelogFull = false
				m.chatInput.Placeholder = "Chat with agent..."

				return m, m.fetchChangelog(text, full)
			}

			// Check for slash commands before sending as chat message.
			if cmd, args := parseSlashCommand(text); cmd != nil {
				return m, m.executeCommand(cmd, args)
			}

			return m, m.sendChatMessage(text)
		}

		return m, nil

	case "p":
		if !m.chatInput.Focused() {
			return m, m.sendWorkflowCmd(phasePlan)
		}

	case "i":
		if !m.chatInput.Focused() {
			return m, m.sendWorkflowCmd(phaseImplement)
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
			return m, m.sendWorkflowCmd(phaseReview)
		}

	case "S":
		if !m.chatInput.Focused() {
			return m, m.sendWorkflowCmd(phaseSubmit)
		}

	case "d":
		if !m.chatInput.Focused() {
			m.dryRun = !m.dryRun

			return m, nil
		}

	case "c":
		if !m.chatInput.Focused() {
			m.changelogMode = true
			m.changelogFull = false
			m.chatInput.Placeholder = "Enter source..target [note] (e.g. v1.0..v2.0 frontend only)"

			return m, nil
		}

	case "C":
		if !m.chatInput.Focused() {
			m.changelogMode = true
			m.changelogFull = true
			m.chatInput.Placeholder = "Enter source..target [note] (e.g. v1.0..v2.0 frontend only) [full]"

			return m, nil
		}

	case "f":
		if !m.chatInput.Focused() {
			return m, m.sendWorkflowCmd(phaseSimplify)
		}

	case "o":
		if !m.chatInput.Focused() {
			return m, m.sendWorkflowCmd(phaseOptimize)
		}

	case "F":
		if !m.chatInput.Focused() {
			return m, m.sendWorkflowCmd("task.finish")
		}

	case "e":
		if !m.chatInput.Focused() {
			return m, m.sendChatMessage("Explain what you did in the last action, why you made those choices, and any assumptions or constraints you encountered.")
		}

	case "U":
		if !m.chatInput.Focused() {
			return m, m.sendWorkflowCmd("update")
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
			startProgressPolling(m.ctx, dir, m.msgs)
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
		case "adversarial_review_started", "adversarial_review_complete", "adversarial_review_blocked":
			m.worktrees[i].Output = append(m.worktrees[i].Output, msg.event.Message)
		case "cache_hit":
			m.worktrees[i].Output = append(m.worktrees[i].Output, msg.event.Message+" [cached]")
			if m.active == i {
				m.syncViewport()
			}
		case "state_changed":
			m.worktrees[i].State = string(msg.event.State)
			if string(msg.event.State) != stateFailed {
				m.worktrees[i].LastFailureClass = ""
			}
			// Clear progress when leaving active phases.
			switch string(msg.event.State) {
			case statePlanning, stateImplementing, stateSimplifying, stateOptimizing:
				// Keep progress active — polling will update it.
			default:
				m.worktrees[i].ProgressActive = false
			}
		case "phase_failure_classified":
			m.worktrees[i].LastFailureClass = string(msg.event.FailureClass)
		case "autofix_attempt":
			m.worktrees[i].Output = append(m.worktrees[i].Output, msg.event.Message)
			if m.active == i {
				m.syncViewport()
			}
			// Parse attempt/max from the event data if available
			m.worktrees[i].AutoFixAttempt = parseAutoFixAttempt(msg.event.Data)
			m.worktrees[i].AutoFixMax = parseAutoFixMax(msg.event.Data)
		case "autofix_success", "autofix_exhausted":
			m.worktrees[i].Output = append(m.worktrees[i].Output, msg.event.Message)
			if m.active == i {
				m.syncViewport()
			}
			m.worktrees[i].AutoFixAttempt = 0
			m.worktrees[i].AutoFixMax = 0
		case "worktree_provisioned":
			m.worktrees[i].Output = append(m.worktrees[i].Output, msg.event.Message)
			if m.active == i {
				m.syncViewport()
			}
		case "risk_evaluated":
			if msg.event.Data != nil {
				var riskData struct {
					Level string `json:"level"`
				}
				if err := json.Unmarshal(msg.event.Data, &riskData); err == nil && riskData.Level != "" {
					m.worktrees[i].RiskLevel = riskData.Level
				}
			}
		}

		break
	}

	return m, waitForMsg(m.ctx, m.msgs)
}

// hasWorktree returns true if the given dir is already tracked.
func (m *Model) hasWorktree(dir string) bool {
	return slices.ContainsFunc(m.worktrees, func(wt WorktreeState) bool {
		return wt.Dir == dir
	})
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
		annotated := annotateOutputLines(wt.Output)
		m.output.SetContent(strings.Join(annotated, "\n"))
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
	h := max(m.height-reserved, 1)

	return h
}
