// Package tui provides terminal user interface components for kvelmo
// using bubbletea for task lifecycle management.
package tui

import (
	"github.com/valksor/kvelmo/internal/conductor"
)

// socketEventMsg carries a single conductor event from a worktree stream.
type socketEventMsg struct {
	worktreeDir string
	event       conductor.ConductorEvent
}

// worktreeListMsg carries the list of worktree dirs discovered for this project.
type worktreeListMsg struct {
	dirs []string
}

// connectedMsg signals a successful socket connection to a worktree.
type connectedMsg struct {
	worktreeDir string
}

// disconnectedMsg signals a socket disconnection.
type disconnectedMsg struct {
	worktreeDir string
	err         error
}

// progressMsg carries a progress update for a worktree.
type progressMsg struct {
	worktreeDir string
	percent     float64
	etaSeconds  int
	calibrated  bool
	active      bool
}

// errMsg carries a fatal error.
type errMsg struct {
	err error
}
