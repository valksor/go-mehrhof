package socket

import (
	"context"
	"log/slog"
	"time"
)

const (
	healthCheckInterval   = 60 * time.Second
	healthCheckTimeout    = 2 * time.Second
	healthCheckMaxFails   = 3
	healthCheckPingMethod = "ping"
)

// StartHealthChecks runs a background loop that pings all registered worktree
// sockets and updates their Healthy/LastPing fields. Worktrees that fail 3
// consecutive pings are marked unhealthy. Blocks until ctx is cancelled.
func (g *GlobalSocket) StartHealthChecks(ctx context.Context) {
	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			g.runHealthChecks(ctx)
		}
	}
}

func (g *GlobalSocket) runHealthChecks(ctx context.Context) {
	g.mu.RLock()
	worktrees := make([]*WorktreeInfo, 0, len(g.worktrees))
	for _, wt := range g.worktrees {
		worktrees = append(worktrees, wt)
	}
	g.mu.RUnlock()

	for _, wt := range worktrees {
		if wt.SocketPath == "" {
			continue
		}

		healthy := g.pingWorktree(ctx, wt.SocketPath)
		now := time.Now()

		g.mu.Lock()
		// Re-fetch from map to ensure we're updating the live pointer
		if live, ok := g.worktrees[wt.ID]; ok {
			live.LastPing = now
			if healthy {
				h := true
				live.Healthy = &h
				live.failCount = 0
			} else {
				live.failCount++
				if live.failCount >= healthCheckMaxFails {
					h := false
					live.Healthy = &h
				}
			}
		}
		g.mu.Unlock()
	}
}

func (g *GlobalSocket) pingWorktree(ctx context.Context, socketPath string) bool {
	if !SocketExists(socketPath) {
		return false
	}

	client, err := NewClient(socketPath, WithTimeout(healthCheckTimeout))
	if err != nil {
		slog.Debug("health check: connect failed", "socket", socketPath, "error", err)

		return false
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()

	_, err = client.Call(ctx, healthCheckPingMethod, nil)
	if err != nil {
		slog.Debug("health check: ping failed", "socket", socketPath, "error", err)

		return false
	}

	return true
}

// handleSystemHealth returns health status for all registered worktrees.
func (g *GlobalSocket) handleSystemHealth(_ context.Context, req *Request) (*Response, error) {
	g.mu.RLock()
	type worktreeHealth struct {
		ID       string    `json:"id"`
		Path     string    `json:"path"`
		State    string    `json:"state"`
		Healthy  *bool     `json:"healthy"`
		LastPing time.Time `json:"last_ping,omitzero"`
	}

	results := make([]worktreeHealth, 0, len(g.worktrees))
	for _, wt := range g.worktrees {
		results = append(results, worktreeHealth{
			ID:       wt.ID,
			Path:     wt.Path,
			State:    wt.State,
			Healthy:  wt.Healthy,
			LastPing: wt.LastPing,
		})
	}
	g.mu.RUnlock()

	return NewResultResponse(req.ID, map[string]any{
		"worktrees": results,
	})
}
