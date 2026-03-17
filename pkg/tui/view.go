package tui

import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	statusBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("255")).
			Width(0). // set dynamically
			Padding(0, 1)

	tabActiveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true).
			Padding(0, 1)

	tabInactiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241")).
				Padding(0, 1)

	dividerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	workerActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82"))

	workerDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	helpBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2)
)

// View renders the full TUI frame.
func (m *Model) View() string {
	if !m.ready {
		return "Loading...\n"
	}

	if m.showHelp {
		return m.renderHelp()
	}

	var b strings.Builder

	b.WriteString(m.renderStatusBar())
	b.WriteString("\n")

	if len(m.worktrees) > 1 {
		b.WriteString(m.renderTabBar())
		b.WriteString("\n")
	}

	b.WriteString(m.output.View())
	b.WriteString("\n")

	if m.layout == LayoutDashboard {
		b.WriteString(m.renderWorkers())
	}

	b.WriteString(m.renderChatInput())

	return b.String()
}

// renderStatusBar renders the 1-line status bar at the top.
func (m *Model) renderStatusBar() string {
	style := statusBarStyle.Width(m.width)

	wt := m.activeWorktree()
	if wt == nil {
		return style.Render("kvelmo · no active tasks")
	}

	base := filepath.Base(wt.Dir)
	state := wt.State
	if state == "" {
		state = "none"
	}

	workerName := ""
	for _, w := range wt.Workers {
		if w.State == "working" {
			workerName = w.Name

			break
		}
	}

	text := "kvelmo · " + base + " · " + state
	if workerName != "" {
		text += " · " + workerName
	}

	return style.Render(text)
}

// renderTabBar renders the tab bar when multiple worktrees are active.
func (m *Model) renderTabBar() string {
	var parts []string
	for i, wt := range m.worktrees {
		base := filepath.Base(wt.Dir)
		if i == m.active {
			parts = append(parts, tabActiveStyle.Render("● "+base))
		} else {
			parts = append(parts, tabInactiveStyle.Render(base))
		}
	}

	return strings.Join(parts, "")
}

// renderWorkers renders the workers pane (dashboard layout only).
func (m *Model) renderWorkers() string {
	wt := m.activeWorktree()

	var b strings.Builder
	divider := strings.Repeat("─", max(0, m.width-10))
	b.WriteString(dividerStyle.Render(" Workers " + divider))
	b.WriteString("\n")

	if wt == nil || len(wt.Workers) == 0 {
		b.WriteString(workerDimStyle.Render(" (no workers)"))
		b.WriteString("\n")

		return b.String()
	}

	for _, w := range wt.Workers {
		if w.State == "working" {
			b.WriteString(workerActiveStyle.Render(" ● "))
		} else {
			b.WriteString(workerDimStyle.Render(" ○ "))
		}
		b.WriteString(padRight(w.Name, 12))
		b.WriteString(padRight(w.State, 12))
		b.WriteString(w.JobID)
		b.WriteString("\n")
	}

	return b.String()
}

// renderChatInput renders the chat divider and text input.
func (m *Model) renderChatInput() string {
	divider := strings.Repeat("─", max(0, m.width))

	return dividerStyle.Render(divider) + "\n" +
		"> " + m.chatInput.View() + "\n"
}

// renderHelp renders a centered help overlay with keybinding table.
func (m *Model) renderHelp() string {
	lines := []string{
		" Keybindings",
		"",
		" tab/shift+tab   Switch worktree",
		" enter           Send message",
		" p               Plan",
		" i               Implement",
		" s               Stop job",
		" ctrl+a          Abort task",
		" q/ctrl+c        Quit",
		" ?               Toggle help",
	}

	box := helpBoxStyle.Render(strings.Join(lines, "\n"))

	// Center horizontally and vertically.
	boxLines := strings.Split(box, "\n")
	boxHeight := len(boxLines)
	boxWidth := 0
	for _, l := range boxLines {
		if w := lipgloss.Width(l); w > boxWidth {
			boxWidth = w
		}
	}

	topPad := (m.height - boxHeight) / 2
	leftPad := (m.width - boxWidth) / 2
	if topPad < 0 {
		topPad = 0
	}
	if leftPad < 0 {
		leftPad = 0
	}

	var b strings.Builder
	for range topPad {
		b.WriteString("\n")
	}
	indent := strings.Repeat(" ", leftPad)
	for _, line := range boxLines {
		b.WriteString(indent + line + "\n")
	}

	return b.String()
}

// padRight pads or truncates s to exactly n runes.
func padRight(s string, n int) string {
	runes := []rune(s)
	if len(runes) >= n {
		return string(runes[:n])
	}

	return s + strings.Repeat(" ", n-len(runes))
}
