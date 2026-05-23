package claudemcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/valksor/kvelmo/settings"
)

// anthropicCredEnvPrefixes lists env var names that, if present, cause the
// claude CLI to bill against an API account instead of the Max subscription.
// The adapter strips these from the child environment before spawn.
var anthropicCredEnvPrefixes = []string{
	"ANTHROPIC_API_KEY=",
	"ANTHROPIC_AUTH_TOKEN=",
	"ANTHROPIC_BEARER_TOKEN=",
}

// isAnthropicCredEnv reports whether kv (a "KEY=VALUE" entry) is one of the
// Anthropic credential vars that must not leak into the spawned claude session.
func isAnthropicCredEnv(kv string) bool {
	for _, p := range anthropicCredEnvPrefixes {
		if strings.HasPrefix(kv, p) {
			return true
		}
	}

	return false
}

// ptyProcess wraps a `claude` invocation running under a pseudo-terminal.
// Captures stdout as opaque transcript lines (never parsed for control); the
// authoritative control signals come from the rendezvous socket and the
// process exit code.
type ptyProcess struct {
	cmd       *exec.Cmd
	pty       *os.File
	lines     chan string
	done      chan struct{}
	readDone  chan struct{} // closed by readLoop when it stops sending to lines
	closeOnce sync.Once
}

// startPTY spawns the given argv under a PTY with the inherited env plus the
// supplied extras. The returned ptyProcess streams stdout lines into .lines
// and signals process exit via .done.
//
// extraEnv is appended to the parent process environment so the spawned
// claude inherits the user's saved login (critical for sub billing) and any
// other env vars set by the conductor.
func startPTY(ctx context.Context, argv []string, workDir string, extraEnv map[string]string) (*ptyProcess, error) {
	if len(argv) == 0 {
		return nil, errors.New("argv is empty")
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)

	// Filter Anthropic credential env vars out of the child environment.
	// Interactive `claude` falls back to its saved login when these are
	// absent — and that fallback is precisely what keeps the session billed
	// against the Max subscription instead of the API account. If the user
	// has ANTHROPIC_API_KEY set for other tools, leaving it in the child env
	// would silently route the session onto API billing and defeat the
	// entire purpose of this adapter.
	// Build the child env via settings.ProcessEnv: kvelmo's config-dir .env plus
	// extraEnv, over a minimal host base (HOME/PATH/...). Host secrets are not
	// inherited; ANTHROPIC_* is then stripped below.
	merged := settings.ProcessEnv(workDir, extraEnv)
	env := make([]string, 0, len(merged))
	stripped := false
	for _, kv := range merged {
		if isAnthropicCredEnv(kv) {
			stripped = true

			continue
		}
		env = append(env, kv)
	}
	if stripped {
		const msg = "claudemcp: stripped ANTHROPIC_* credential env vars from the child claude process " +
			"so it falls back to its saved login (required for Max-subscription billing). " +
			"If you actually wanted the API key to be used, switch agent.default to 'anthropic' " +
			"(direct API key billing) instead of 'claude-mcp'. See docs/agents/claude-mcp.md."
		slog.Warn(msg)
		// Also surface on stderr so CLI users see it without enabling debug logs.
		fmt.Fprintln(os.Stderr, msg)
	}
	cmd.Env = env
	if workDir != "" {
		cmd.Dir = workDir
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	master, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("pty.Start %s: %w", argv[0], err)
	}

	p := &ptyProcess{
		cmd:      cmd,
		pty:      master,
		lines:    make(chan string, 1024),
		done:     make(chan struct{}),
		readDone: make(chan struct{}),
	}

	go p.readLoop()
	go p.waitLoop()

	return p, nil
}

// readLoop pumps PTY output into p.lines in byte chunks. claude runs a
// full-screen TUI that paints with cursor-positioning ANSI and few newlines,
// so line-based reading (ReadString('\n')) would block indefinitely without
// surfacing the screen content. Reading raw chunks captures the transcript as
// it is written and lets the pump observe PTY activity (for idle detection).
// When the buffer is full, oldest chunks are dropped to keep memory bounded.
func (p *ptyProcess) readLoop() {
	// Signal that no further sends to p.lines will happen. waitLoop waits on
	// this before closing p.lines, so the channel close cannot race a send
	// here (which previously panicked with "send on closed channel" when the
	// child exited with output still buffered).
	defer close(p.readDone)
	buf := make([]byte, 8*1024)
	for {
		n, err := p.pty.Read(buf)
		if n > 0 {
			chunk := string(buf[:n])
			select {
			case p.lines <- chunk:
			default:
				// Drop oldest to make room — bounded buffer policy.
				select {
				case <-p.lines:
				default:
				}
				select {
				case p.lines <- chunk:
				default:
				}
			}
		}
		if err != nil {
			return
		}
	}
}

func (p *ptyProcess) waitLoop() {
	_ = p.cmd.Wait()
	// Closing the PTY master unblocks readLoop's ReadString, which then exits
	// and closes p.readDone. Wait for that before closing p.lines so the close
	// can never race an in-flight send in readLoop.
	_ = p.pty.Close()
	<-p.readDone
	close(p.lines)
	close(p.done)
}

// Interrupt sends Ctrl-C to the PTY, then escalates to SIGTERM/SIGKILL.
func (p *ptyProcess) Interrupt() error {
	if p == nil || p.cmd.Process == nil {
		return nil
	}
	_, _ = p.pty.Write([]byte{0x03}) // Ctrl-C
	select {
	case <-p.done:
		return nil
	case <-time.After(5 * time.Second):
	}
	_ = p.cmd.Process.Signal(syscall.SIGTERM)
	select {
	case <-p.done:
		return nil
	case <-time.After(5 * time.Second):
	}

	return p.cmd.Process.Kill()
}

// Close closes the PTY master and ensures the process exits. Safe to call
// repeatedly from any goroutine.
func (p *ptyProcess) Close() error {
	if p == nil {
		return nil
	}
	p.closeOnce.Do(func() {
		_ = p.pty.Close()
		if p.cmd.Process != nil {
			_ = p.cmd.Process.Signal(syscall.SIGTERM)
			select {
			case <-p.done:
			case <-time.After(2 * time.Second):
				_ = p.cmd.Process.Kill()
			}
		}
	})

	return nil
}
