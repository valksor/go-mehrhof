package claudemcp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/valksor/kvelmo/agent"
	"github.com/valksor/kvelmo/internal/mcp"
	"github.com/valksor/kvelmo/paths"
)

// Agent implements agent.Agent for the claude-mcp mode.
type Agent struct {
	config Config

	mu          sync.RWMutex
	pty         *ptyProcess
	rdz         *rendezvousListener
	events      chan agent.Event
	workDir     string // per-session work dir (mcp-config.json, system-prompt.md, adapter.sock)
	cancelSpawn context.CancelFunc

	connected bool
}

// New creates a claude-mcp agent with default configuration.
func New() *Agent { return NewWithConfig(DefaultConfig()) }

// NewWithConfig creates a claude-mcp agent with the given configuration.
func NewWithConfig(cfg Config) *Agent {
	if len(cfg.Command) == 0 {
		cfg.Command = []string{"claude"}
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Minute
	}
	if cfg.PermissionMode == "" {
		cfg.PermissionMode = PermissionModeAcceptEdits
	}
	if len(cfg.MCPServerCommand) == 0 {
		cfg.MCPServerCommand = []string{"kvelmo", "mcp", "--stdio"}
	}

	return &Agent{config: cfg}
}

// Name returns the agent identifier.
func (a *Agent) Name() string { return AgentName }

// Available checks if the claude CLI binary is installed and runnable.
func (a *Agent) Available() error {
	binary := a.config.Command[0]
	path, err := agent.ResolveCommandPath(binary)
	if err != nil {
		return fmt.Errorf("claude CLI not found: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, path, "--version").Run(); err != nil {
		return fmt.Errorf("claude CLI not working: %w", err)
	}

	return nil
}

// Connect for claude-mcp is a no-op marker — actual session lifecycle starts
// in SendPrompt. The interface contract still requires Connected/Connect, so
// we satisfy them stateless.
func (a *Agent) Connect(_ context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.connected = true

	return nil
}

func (a *Agent) Connected() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.connected
}

// SendPrompt starts a fresh interactive claude session for the given prompt
// and returns an event channel that closes when the session ends. Only one
// session is active at a time per Agent instance.
func (a *Agent) SendPrompt(ctx context.Context, prompt string) (<-chan agent.Event, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.pty != nil {
		return nil, errors.New("claudemcp: a session is already active; call Close() first")
	}

	// Resolve worktree from WorkDir (set by worker pool via WithWorkDir).
	// Fall back to KVELMO_WORKTREE env if WorkDir is empty (manual callers).
	worktree := a.config.WorkDir
	if worktree == "" {
		worktree = a.config.Environment["KVELMO_WORKTREE"]
	}
	if worktree == "" {
		return nil, errors.New("claudemcp: agent has no WorkDir (caller must use WithWorkDir before SendPrompt)")
	}

	// Worktree socket is derivable from the worktree path via the same hash
	// kvelmo uses everywhere. Override via KVELMO_WORKTREE_SOCKET if explicitly set.
	workSocket := a.config.Environment["KVELMO_WORKTREE_SOCKET"]
	if workSocket == "" {
		workSocket = paths.WorktreeSocketPath(worktree)
	}

	// Task identification is per-worktree (one active task at a time); use a
	// short random tag for the work-dir suffix when no explicit task id is set.
	taskID := a.config.Environment["KVELMO_TASK_ID"]
	if taskID == "" {
		taskID = "session-" + randomSuffix(4)
	}
	phase := a.config.Environment["KVELMO_PHASE"]

	if err := validatePermissionMode(a.config.PermissionMode); err != nil {
		return nil, err
	}

	workDir, err := a.prepareWorkDir(taskID)
	if err != nil {
		return nil, err
	}
	// Clean up the work dir if SendPrompt fails before handing the session
	// to the pump goroutine. The success path leaves it for the lifetime of
	// the session; pump's cleanupAfterSession removes the rendezvous socket
	// but intentionally leaves transcripts/config in place for debugging.
	cleanupOnError := func() { _ = os.RemoveAll(workDir) }
	a.workDir = workDir

	mcpConfigPath := filepath.Join(workDir, "mcp-config.json")
	systemPromptPath := filepath.Join(workDir, "system-prompt.md")
	rendezvousPath := filepath.Join(workDir, "adapter.sock")

	if err := writeSystemPrompt(systemPromptPath, a.config.SystemPromptOverride, taskID, phase, worktree); err != nil {
		cleanupOnError()

		return nil, fmt.Errorf("write system prompt: %w", err)
	}

	rdz, rdzEvents, err := startRendezvous(ctx, rendezvousPath)
	if err != nil {
		cleanupOnError()

		return nil, fmt.Errorf("start rendezvous: %w", err)
	}
	a.rdz = rdz

	if err := writeMCPConfig(
		mcpConfigPath,
		a.config.MCPServerCommand,
		taskID, worktree, workSocket, rendezvousPath,
		os.Getpid(),
	); err != nil {
		_ = rdz.Close()
		a.rdz = nil
		cleanupOnError()

		return nil, fmt.Errorf("write mcp config: %w", err)
	}

	argv := a.buildArgv(mcpConfigPath, systemPromptPath, prompt)
	slog.Info("claudemcp: spawning claude TUI",
		"argv", argv,
		"task_id", taskID,
		"work_dir", workDir)

	// Apply the configured session timeout to the spawn context if the
	// caller did not already supply a deadline shorter than Config.Timeout.
	spawnCtx := ctx
	if a.config.Timeout > 0 {
		if dl, ok := ctx.Deadline(); !ok || time.Until(dl) > a.config.Timeout {
			var cancel context.CancelFunc
			spawnCtx, cancel = context.WithTimeout(ctx, a.config.Timeout)
			a.cancelSpawn = cancel
		}
	}

	extraEnv := map[string]string{}
	// Do NOT inject ANTHROPIC_API_KEY — interactive claude uses its saved
	// login, which is what keeps the session on the Max subscription.
	// pty_process.go::startPTY actively strips ANTHROPIC_* vars inherited
	// from the parent env for the same reason.

	pty, err := startPTY(spawnCtx, argv, a.config.WorkDir, extraEnv)
	if err != nil {
		_ = rdz.Close()
		a.rdz = nil
		cleanupOnError()

		return nil, fmt.Errorf("start claude pty: %w", err)
	}
	a.pty = pty

	a.events = make(chan agent.Event, 64)
	go a.pump(spawnCtx, pty, rdzEvents)

	return a.events, nil
}

// validatePermissionMode rejects values that aren't in the known-safe set.
// Defends against typos in user config silently degrading to claude's own
// fallback (which can be more permissive than expected).
func validatePermissionMode(mode string) error {
	switch mode {
	case PermissionModeAcceptEdits,
		PermissionModeAuto,
		PermissionModeBypassPermission,
		PermissionModeDontAsk,
		PermissionModeDefault,
		"":
		return nil
	}

	return fmt.Errorf("claudemcp: invalid permission_mode %q (valid: acceptEdits, auto, bypassPermissions, dontAsk, default)", mode)
}

// buildArgv constructs the claude CLI invocation. Deliberately omits --print
// and --sdk-url so that the session bills against the Max sub (interactive
// Claude Code), not the post-2026-06-15 Agent SDK credit pool.
func (a *Agent) buildArgv(mcpConfig, systemPrompt, seedPrompt string) []string {
	args := []string{a.config.Command[0]}
	if a.config.StrictMCPConfig {
		args = append(args, "--strict-mcp-config")
	}
	args = append(
		args,
		"--mcp-config", mcpConfig,
		"--append-system-prompt", systemPrompt,
		"--permission-mode", a.config.PermissionMode,
	)
	if a.config.Model != "" {
		args = append(args, "--model", a.config.Model)
	}
	args = append(args, a.config.Args...)
	if seedPrompt != "" {
		args = append(args, seedPrompt)
	}

	return args
}

// prepareWorkDir creates ~/.valksor/kvelmo/work/<task>/claudemcp-<rand>/ so
// concurrent sessions for the same task don't collide on the rendezvous
// socket path. Returns the absolute path.
func (a *Agent) prepareWorkDir(taskID string) (string, error) {
	root := a.config.WorkRoot
	if root == "" {
		root = filepath.Join(paths.BaseDir(), "work")
	}
	suffix := randomSuffix(6)
	dir := filepath.Join(root, taskID, "claudemcp-"+suffix)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create work dir %s: %w", dir, err)
	}

	return dir, nil
}

// pump fans PTY lines and rendezvous events into a.events until the session
// ends (either signal_complete arrives, the PTY exits, or ctx is cancelled).
func (a *Agent) pump(ctx context.Context, pty *ptyProcess, rdz <-chan mcp.RendezvousEvent) {
	defer close(a.events)
	defer a.cleanupAfterSession()

	finished := false
	for !finished {
		select {
		case <-ctx.Done():
			a.events <- agent.Event{
				Type:      agent.EventError,
				Error:     "claudemcp: context cancelled",
				Timestamp: time.Now(),
			}
			_ = pty.Interrupt()
			finished = true
		case line, ok := <-pty.lines:
			if !ok {
				// PTY closed before signal_complete — record error and exit.
				a.events <- agent.Event{
					Type:      agent.EventError,
					Error:     "claudemcp: claude exited without signalling completion",
					Timestamp: time.Now(),
				}
				finished = true

				continue
			}
			a.events <- agent.Event{
				Type:      agent.EventStream,
				Content:   line,
				Data:      map[string]any{"raw_ansi": true},
				Timestamp: time.Now(),
			}
		case evt, ok := <-rdz:
			if !ok {
				continue
			}
			switch evt.Type {
			case "complete":
				a.events <- agent.Event{
					Type:      agent.EventComplete,
					Content:   evt.Summary,
					Data:      map[string]any{"phase": evt.Phase},
					Timestamp: time.Now(),
				}
				_ = pty.Interrupt()
				finished = true
			case "failure":
				a.events <- agent.Event{
					Type:      agent.EventError,
					Error:     fmt.Sprintf("phase %s failed: %s", evt.Phase, evt.Reason),
					Data:      map[string]any{"phase": evt.Phase, "retryable": evt.Retryable},
					Timestamp: time.Now(),
				}
				_ = pty.Interrupt()
				finished = true
			}
		}
	}
}

// cleanupAfterSession releases the session's resources. Extracts pointers
// under the lock then releases it before calling Close on each, so a slow
// Close (e.g. rdz.Close blocked on connWG.Wait waiting on a stuck connection
// goroutine) does not hold a.mu and deadlock concurrent Interrupt/Close
// callers.
func (a *Agent) cleanupAfterSession() {
	a.mu.Lock()
	rdz := a.rdz
	a.rdz = nil
	pty := a.pty
	a.pty = nil
	cancel := a.cancelSpawn
	a.cancelSpawn = nil
	a.mu.Unlock()

	if rdz != nil {
		_ = rdz.Close()
	}
	if pty != nil {
		_ = pty.Close()
	}
	if cancel != nil {
		cancel()
	}
}

// HandlePermission is a no-op for claude-mcp — permissions are handled inside
// the interactive claude session via --permission-mode.
func (a *Agent) HandlePermission(_ string, _ bool) error { return nil }

// Interrupt terminates the active claude session if any.
func (a *Agent) Interrupt() error {
	a.mu.RLock()
	pty := a.pty
	a.mu.RUnlock()
	if pty == nil {
		return nil
	}

	return pty.Interrupt()
}

// Close terminates the session and releases resources.
func (a *Agent) Close() error {
	a.cleanupAfterSession()
	a.mu.Lock()
	a.connected = false
	a.mu.Unlock()

	return nil
}

// WithEnv returns a new Agent with an additional environment variable.
func (a *Agent) WithEnv(key, value string) agent.Agent {
	newCfg := a.config
	env := make(map[string]string, len(a.config.Environment)+1)
	maps.Copy(env, a.config.Environment)
	env[key] = value
	newCfg.Environment = env

	return NewWithConfig(newCfg)
}

func (a *Agent) WithArgs(args ...string) agent.Agent {
	newCfg := a.config
	newCfg.Args = append(append([]string(nil), a.config.Args...), args...)

	return NewWithConfig(newCfg)
}

func (a *Agent) WithWorkDir(dir string) agent.Agent {
	newCfg := a.config
	newCfg.WorkDir = dir

	return NewWithConfig(newCfg)
}

func (a *Agent) WithTimeout(d time.Duration) agent.Agent {
	newCfg := a.config
	newCfg.Timeout = d

	return NewWithConfig(newCfg)
}

// Register adds the claude-mcp agent to a registry with default config.
func Register(r *agent.Registry) error { return r.Register(New()) }

// randomSuffix returns a hex string of n bytes for unique session work dirs.
func randomSuffix(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}

	return hex.EncodeToString(buf)
}

// Ensure Agent implements agent.Agent.
var _ agent.Agent = (*Agent)(nil)
