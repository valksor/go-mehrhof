package socket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/valksor/kvelmo/pkg/conductor"
	"github.com/valksor/kvelmo/pkg/notify"
)

// projectsFile returns the path to the projects JSON file.
func (g *GlobalSocket) projectsFile() string {
	dir := g.projectsDir
	if dir == "" {
		dir = BaseDir()
	}

	return filepath.Join(dir, "projects.json")
}

// loadProjectsFromFile loads projects from the JSON file.
func (g *GlobalSocket) loadProjectsFromFile() {
	data, err := os.ReadFile(g.projectsFile())
	if err != nil {
		return
	}
	var projects []WorktreeInfo
	if err := json.Unmarshal(data, &projects); err != nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, p := range projects {
		g.worktrees[p.ID] = &WorktreeInfo{
			ID:       p.ID,
			Path:     p.Path,
			State:    p.State,
			LastSeen: time.Now(),
		}
	}
}

// saveProjectsToFile saves projects to the JSON file.
func (g *GlobalSocket) saveProjectsToFile() {
	g.mu.RLock()
	projects := make([]WorktreeInfo, 0, len(g.worktrees))
	for _, w := range g.worktrees {
		projects = append(projects, *w)
	}
	g.mu.RUnlock()

	data, err := json.MarshalIndent(projects, "", "  ")
	if err != nil {
		slog.Error("failed to marshal projects", "error", err)

		return
	}

	target := g.projectsFile()
	tmpFile := target + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0o644); err != nil {
		slog.Error("failed to write projects temp file", "path", tmpFile, "error", err)

		return
	}
	if err := os.Rename(tmpFile, target); err != nil {
		slog.Error("failed to rename projects temp file", "from", tmpFile, "to", target, "error", err)
	}
}

// GetOrCreateWorktreeSocket returns an existing worktree socket or creates one on-demand.
// This allows projects to use the global worker pool for planning/implementation.
func (g *GlobalSocket) GetOrCreateWorktreeSocket(projectPath string) (any, error) {
	id := WorktreeIDFromPath(projectPath)
	socketPath := WorktreeSocketPath(projectPath)

	// Fast path: check if socket already exists
	g.wtSocketsMu.RLock()
	if wt, ok := g.wtSockets[id]; ok {
		g.wtSocketsMu.RUnlock()

		return wt, nil
	}
	g.wtSocketsMu.RUnlock()

	// Slow path: create new socket
	g.wtSocketsMu.Lock()

	// Double-check after acquiring write lock
	if wt, ok := g.wtSockets[id]; ok {
		g.wtSocketsMu.Unlock()

		return wt, nil
	}

	// Check if we're shutting down
	select {
	case <-g.ctx.Done():
		g.wtSocketsMu.Unlock()

		return nil, errors.New("global socket is shutting down")
	default:
	}

	// Acquire project-level file lock to prevent concurrent access from other processes
	lockPath := WorktreeLockPath(projectPath)
	releaseLock, lockErr := AcquireGlobalLock(lockPath)
	if lockErr != nil {
		g.wtSocketsMu.Unlock()

		return nil, fmt.Errorf("project is already in use by another process: %w", lockErr)
	}

	// Create new worktree socket with pool access
	wt, err := NewWorktreeSocket(WorktreeConfig{
		WorktreePath: projectPath,
		SocketPath:   socketPath,
		GlobalPath:   g.server.Path(),
		Pool:         g.pool,
	})
	if err != nil {
		releaseLock()
		g.wtSocketsMu.Unlock()

		return nil, fmt.Errorf("create worktree socket: %w", err)
	}

	// Add to map before starting goroutine to avoid race condition
	g.wtSockets[id] = wt

	// Listen for state changes and broadcast to all global socket clients
	if wt.conductor != nil {
		wt.conductor.OnEvent(func(event conductor.ConductorEvent) {
			switch event.Type {
			case "state_changed":
				g.broadcastTaskStateChanged(projectPath, string(event.State))
			case "approval_required":
				g.sendApprovalNotification(projectPath, event.Message)
			}
		})
	}

	// Start the socket in background
	// Note: Callers should handle connection retries if socket isn't ready yet
	go func() {
		defer releaseLock() // Release project lock when socket stops

		if err := wt.Start(g.ctx); err != nil && !errors.Is(err, context.Canceled) {
			// Log error but don't panic - socket can be recreated
			slog.Error("worktree socket error", "id", id, "error", err)
		}

		// Remove from map when socket stops
		g.wtSocketsMu.Lock()
		delete(g.wtSockets, id)
		g.wtSocketsMu.Unlock()
	}()

	g.wtSocketsMu.Unlock()

	return wt, nil
}

// sendApprovalNotification sends a webhook notification when approval is required.
func (g *GlobalSocket) sendApprovalNotification(projectPath, message string) {
	n := GetNotifier()
	if n == nil {
		return
	}
	n.Send(notify.Payload{
		Event:       "approval_required",
		Timestamp:   time.Now(),
		Message:     message,
		ProjectPath: projectPath,
	})
}

// broadcastTaskStateChanged sends a task_state_changed notification to all global socket clients.
func (g *GlobalSocket) broadcastTaskStateChanged(projectPath string, state string) {
	notification := map[string]any{
		"jsonrpc": JSONRPCVersion,
		"method":  "task_state_changed",
		"params": map[string]string{
			"path":  projectPath,
			"state": state,
		},
	}
	data, err := json.Marshal(notification)
	if err != nil {
		return
	}
	g.server.Broadcast(append(data, '\n'))
}

// BroadcastWorkerChanged notifies all connected clients that the worker pool has changed.
func (g *GlobalSocket) BroadcastWorkerChanged() {
	notification := map[string]any{
		"jsonrpc": JSONRPCVersion,
		"method":  "worker_changed",
	}
	data, err := json.Marshal(notification)
	if err != nil {
		return
	}
	g.server.Broadcast(append(data, '\n'))
}

// GetWorktree returns worktree info by ID.
func (g *GlobalSocket) GetWorktree(id string) *WorktreeInfo {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return g.worktrees[id]
}

// ListWorktrees returns all registered worktrees.
func (g *GlobalSocket) ListWorktrees() []*WorktreeInfo {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]*WorktreeInfo, 0, len(g.worktrees))
	for _, w := range g.worktrees {
		cp := *w
		result = append(result, &cp)
	}

	return result
}
