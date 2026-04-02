package claude

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/valksor/kvelmo/pkg/agent"
)

// launchClaude starts the Claude CLI process with --sdk-url.
func (w *WebSocketConnection) launchClaude(ctx context.Context) error {
	w.cmdMu.Lock()
	defer w.cmdMu.Unlock()

	args := w.buildArgs()
	w.cmd = exec.CommandContext(ctx, w.config.Command[0], args...)

	if w.config.WorkDir != "" {
		w.cmd.Dir = w.config.WorkDir
	}

	// Build environment: start with parent env, exclude CLAUDECODE to allow nested sessions
	env := make([]string, 0, len(os.Environ())+len(w.config.Environment))
	for _, e := range os.Environ() {
		// Skip CLAUDECODE to allow running Claude CLI from within Claude Code
		if !strings.HasPrefix(e, "CLAUDECODE=") {
			env = append(env, e)
		}
	}
	// Add custom config environment variables
	for k, v := range w.config.Environment {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	w.cmd.Env = env

	// Capture stdout for debugging
	stdout, err := w.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	// Capture stderr for debugging
	stderr, err := w.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := w.cmd.Start(); err != nil {
		return fmt.Errorf("start claude: %w", err)
	}

	slog.Info("claude CLI started", "pid", w.cmd.Process.Pid)

	// Log stdout in background
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			slog.Debug("claude CLI stdout", "line", line)
		}
		slog.Debug("claude CLI stdout closed")
	}()

	// Log stderr in background
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			slog.Error("claude CLI stderr", "line", line)
			if strings.TrimSpace(line) != "" {
				w.trySendEvent(agent.Event{
					Type:      agent.EventError,
					Content:   line,
					Timestamp: time.Now(),
				})
			}
		}
		slog.Info("claude CLI stderr closed")
	}()

	// Wait for process completion in background.
	// Cancel lifecycleCtx on unexpected exit to prevent goroutine leaks
	// from the done-channel watcher spawned in Connect().
	go func() {
		err := w.cmd.Wait()
		slog.Info("claude CLI exited", "error", err)
		w.cmdMu.Lock()
		w.cmdErr = err
		w.cmdMu.Unlock()
		w.connected.Store(false)
		if w.lifecycleCancel != nil {
			w.lifecycleCancel()
		}
	}()

	return nil
}

// buildArgs constructs CLI arguments for Claude with WebSocket.
func (w *WebSocketConnection) buildArgs() []string {
	args := []string{
		"--sdk-url", fmt.Sprintf("ws://127.0.0.1:%d", w.port),
		"--print",
		"--verbose",
		"--output-format", "stream-json",
		"--input-format", "stream-json",
		"--include-partial-messages",
		"--permission-mode", "bypassPermissions", // kvelmo manages permissions via KvelmoPermissionHandler
	}
	slog.Debug("claude websocket buildArgs", "args", args)

	// Add configured arguments
	args = append(args, w.config.Args...)

	// Add model if specified
	if w.config.Model != "" {
		args = append(args, "--model", w.config.Model)
	}

	return args
}
