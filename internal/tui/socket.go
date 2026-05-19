package tui

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os/exec"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/valksor/kvelmo/internal/conductor"
	"github.com/valksor/kvelmo/internal/socket"
)

// discoverWorktrees connects to the global socket, calls tasks.list, and
// returns the dirs of all tasks that share the same git root as cwd.
// If the global socket is not running, it falls back to just [cwd].
func discoverWorktrees(cwd string) tea.Cmd {
	return func() tea.Msg {
		globalPath := socket.GlobalSocketPath()
		if !socket.SocketExists(globalPath) {
			return worktreeListMsg{dirs: []string{cwd}}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		cwdRoot := gitRoot(ctx, cwd)

		client, err := socket.NewClient(globalPath, socket.WithTimeout(5*time.Second))
		if err != nil {
			return worktreeListMsg{dirs: []string{cwd}}
		}
		defer func() { _ = client.Close() }()

		resp, err := client.Call(ctx, "tasks.list", nil)
		if err != nil {
			return worktreeListMsg{dirs: []string{cwd}}
		}

		var result socket.TasksListResult
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			return worktreeListMsg{dirs: []string{cwd}}
		}

		var dirs []string
		for _, t := range result.Tasks {
			if t.Path == "" {
				continue
			}
			if gitRoot(ctx, t.Path) == cwdRoot {
				dirs = append(dirs, t.Path)
			}
		}

		if len(dirs) == 0 {
			dirs = []string{cwd}
		}

		return worktreeListMsg{dirs: dirs}
	}
}

// gitRoot returns the git repository root for the given directory,
// or the directory itself if the git command fails.
func gitRoot(ctx context.Context, dir string) string {
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return dir
	}

	return strings.TrimSpace(string(out))
}

// subscribeWorktree dials the worktree socket, sends stream.subscribe, starts
// a background goroutine that forwards NDJSON events to ch, and returns
// connectedMsg. If the socket does not exist, it returns disconnectedMsg directly.
func subscribeWorktree(ctx context.Context, dir string, ch chan<- tea.Msg) tea.Cmd {
	return func() tea.Msg {
		socketPath := socket.WorktreeSocketPath(dir)
		if !socket.SocketExists(socketPath) {
			return disconnectedMsg{worktreeDir: dir, err: errors.New("no worktree socket")}
		}

		conn, err := (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
		if err != nil {
			return disconnectedMsg{worktreeDir: dir, err: err}
		}

		req := socket.Request{
			JSONRPC: "2.0",
			ID:      "tui-sub",
			Method:  "stream.subscribe",
		}
		reqBytes, err := json.Marshal(req)
		if err != nil {
			_ = conn.Close()

			return disconnectedMsg{worktreeDir: dir, err: err}
		}
		reqBytes = append(reqBytes, '\n')
		if _, err := conn.Write(reqBytes); err != nil {
			_ = conn.Close()

			return disconnectedMsg{worktreeDir: dir, err: err}
		}

		// Create ONE scanner before the initial read, reused in the goroutine.
		scanner := bufio.NewScanner(conn)
		if !scanner.Scan() {
			_ = conn.Close()
			scanErr := scanner.Err()
			if scanErr == nil {
				scanErr = errors.New("connection closed before response")
			}

			return disconnectedMsg{worktreeDir: dir, err: scanErr}
		}

		var initResp socket.Response
		if err := json.Unmarshal(scanner.Bytes(), &initResp); err != nil {
			_ = conn.Close()

			return disconnectedMsg{worktreeDir: dir, err: err}
		}
		if initResp.Error != nil {
			_ = conn.Close()

			return disconnectedMsg{worktreeDir: dir, err: initResp.Error}
		}

		// Start goroutine to read the NDJSON event stream using the same scanner.
		go func() {
			defer func() { _ = conn.Close() }()
			for scanner.Scan() {
				line := scanner.Bytes()
				if len(line) == 0 {
					continue
				}
				var event conductor.ConductorEvent
				if err := json.Unmarshal(line, &event); err != nil {
					continue
				}
				select {
				case ch <- socketEventMsg{worktreeDir: dir, event: event}:
				case <-ctx.Done():
					return
				}
			}
			select {
			case ch <- disconnectedMsg{worktreeDir: dir}:
			case <-ctx.Done():
			}
		}()

		return connectedMsg{worktreeDir: dir}
	}
}

// startProgressPolling launches a goroutine that periodically calls progress.get
// on the worktree socket and sends progressMsg updates to the fan-in channel.
// Skips polling when the worktree is in a terminal or inactive state to avoid
// unnecessary socket calls.
func startProgressPolling(ctx context.Context, dir string, ch chan<- tea.Msg) {
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				socketPath := socket.WorktreeSocketPath(dir)
				client, err := socket.NewClient(socketPath, socket.WithTimeout(2*time.Second))
				if err != nil {
					continue
				}

				// Check worktree state first — skip progress polling for inactive states.
				stateResp, stateErr := client.Call(ctx, "status.get", nil)
				if stateErr == nil && stateResp.Result != nil {
					var status struct {
						State string `json:"state"`
					}
					if json.Unmarshal(stateResp.Result, &status) == nil {
						switch status.State {
						case stateNone, stateLoaded, stateFailed, stateSubmitted:
							_ = client.Close()

							continue
						}
					}
				}

				resp, callErr := client.Call(ctx, "progress.get", nil)
				_ = client.Close()
				if callErr != nil {
					continue
				}

				var result struct {
					Active     bool    `json:"active"`
					Percent    float64 `json:"percent"`
					ETASeconds int     `json:"eta_seconds"`
					Calibrated bool    `json:"calibrated"`
				}
				if err := json.Unmarshal(resp.Result, &result); err != nil {
					continue
				}

				select {
				case ch <- progressMsg{
					worktreeDir: dir,
					percent:     result.Percent,
					etaSeconds:  result.ETASeconds,
					calibrated:  result.Calibrated,
					active:      result.Active,
				}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
}

// waitForMsg returns a tea.Cmd that blocks until a message arrives on ch
// or the context is done, then returns it.
// Call this repeatedly from Update to drain the fan-in channel.
func waitForMsg(ctx context.Context, ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		select {
		case msg, ok := <-ch:
			if !ok {
				return nil
			}

			return msg
		case <-ctx.Done():
			return nil
		}
	}
}
