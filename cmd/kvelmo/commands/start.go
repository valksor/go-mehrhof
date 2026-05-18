package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	_ "net/http/pprof" //nolint:gosec // opt-in via KVELMO_PPROF_ADDR, not exposed by default
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/agent"
	"github.com/valksor/kvelmo/agent/anthropic"
	"github.com/valksor/kvelmo/agent/apiagent"
	"github.com/valksor/kvelmo/agent/claude"
	"github.com/valksor/kvelmo/agent/claudesdk"
	"github.com/valksor/kvelmo/agent/codex"
	"github.com/valksor/kvelmo/agent/ollama"
	"github.com/valksor/kvelmo/agent/openai"
	"github.com/valksor/kvelmo/internal/socket"
	"github.com/valksor/kvelmo/internal/worker"
	"github.com/valksor/kvelmo/meta"
	"github.com/valksor/kvelmo/settings"
)

var (
	startForeground    bool
	startVerbose       bool
	startFrom          string
	startText          string
	startAuto          bool
	startJSON          bool
	startSkip          []string
	startContextFiles  []string
	startContextSymbol []string
	startContextCommit []string
)

var StartCmd = &cobra.Command{
	Use:   "start [description]",
	Short: "Start a task in " + meta.Name,
	Long: `Start kvelmo sockets (in background) and optionally load a task.

By default, sockets start in the background and the command returns immediately.
Use --foreground to run in the foreground (for debugging or E2E tests).

Use --from to load a task from a source:
  kvelmo start --from file:task.md           Load from local file
  kvelmo start --from github:owner/repo#123  Load GitHub issue/PR
  kvelmo start --from https://github.com/... Load from URL

Use --text to create a task from inline description:
  kvelmo start --text "Fix login button alignment"  Load from inline text
  echo "Fix X" | kvelmo start --text -              Load from stdin

Attach context references so the agent gets precise code pointers:
  kvelmo start --file src/handler.go "fix the nil check"
  kvelmo start --file pkg/auth.go:40-60 --symbol Middleware "refactor auth"
  kvelmo start --commit abc123 "revert the regression from this commit"

Multiple context flags can be combined. File refs support line ranges (file.go:42 or file.go:40-60).

Examples:
  kvelmo start                              # Just start sockets
  kvelmo start --from github:org/repo#42    # Start and load task
  kvelmo start --text "Fix typo in README"  # Start and load inline task
  kvelmo start --file main.go "fix build"   # Start with file context
  kvelmo plan                               # Then run planning`,
	RunE: runStart,
}

func init() {
	StartCmd.Flags().BoolVar(&startForeground, "foreground", false, "Run in foreground (for debugging)")
	StartCmd.Flags().BoolVarP(&startVerbose, "verbose", "v", false, "Show socket paths")
	StartCmd.Flags().StringVar(&startFrom, "from", "", "Task source (file:path, github:owner/repo#123, or URL)")
	StartCmd.Flags().StringVar(&startText, "text", "", "Inline task description (creates task without external source)")
	StartCmd.Flags().BoolVar(&startAuto, "auto", false, "Auto-advance through plan → implement → review")
	StartCmd.Flags().StringSliceVar(&startSkip, "skip", nil, "Phases to skip during auto-advance (e.g., --skip simplify,optimize)")
	StartCmd.Flags().BoolVar(&startJSON, "json", false, "Output result as JSON")
	StartCmd.Flags().StringSliceVar(&startContextFiles, "file", nil, "Attach file context (e.g., --file src/main.go --file pkg/handler.go:40-60)")
	StartCmd.Flags().StringSliceVar(&startContextSymbol, "symbol", nil, "Attach symbol context (e.g., --symbol HandleRequest)")
	StartCmd.Flags().StringSliceVar(&startContextCommit, "commit", nil, "Attach commit context (e.g., --commit abc123)")
	StartCmd.Flags().Bool("provision-preview", false, "Preview what would be provisioned for the worktree (dry run)")
}

func runStart(cmd *cobra.Command, args []string) error {
	provisionPreview, _ := cmd.Flags().GetBool("provision-preview")
	if provisionPreview {
		return runProvisionPreview()
	}

	if startText == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		startText = strings.TrimSpace(string(data))
	}
	// Accept positional arg as inline text: kvelmo start "fix the bug"
	if startText == "" && startFrom == "" && len(args) > 0 {
		startText = strings.Join(args, " ")
	}
	if startText != "" && startFrom != "" {
		return errors.New("--text and --from are mutually exclusive")
	}
	if startText != "" {
		startFrom = "empty:" + startText
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Check if this is a git repository
	if !isGitRepository(cwd) {
		fmt.Println()
		fmt.Println("  Warning: Not a git repository")
		fmt.Println()
		fmt.Println("  Some features will be limited:")
		fmt.Println("    • No branch creation for tasks")
		fmt.Println("    • No checkpoint/undo functionality")
		fmt.Println("    • No PR submission")
		fmt.Println()
		fmt.Println("  Run 'git init' to enable full functionality.")
		fmt.Println()
	}

	if err := socket.EnsureDir(); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	globalPath := socket.GlobalSocketPath()
	wtPath := socket.WorktreeSocketPath(cwd)

	if startVerbose {
		fmt.Printf("Global socket: %s\n", globalPath)
		fmt.Printf("Worktree socket: %s\n", wtPath)
	}

	// If --foreground, run in foreground (for E2E tests and debugging)
	if startForeground {
		return runInForeground(cwd, globalPath, wtPath)
	}

	// Default: start in background
	return runInBackground(cwd, wtPath)
}

// runInBackground ensures sockets are running (spawning if needed) and loads task.
func runInBackground(cwd, wtPath string) error {
	// Check if worktree socket is already running
	if socket.SocketExists(wtPath) {
		// Socket exists - just load the task if specified
		if startFrom != "" {
			if err := loadTaskViaRPC(wtPath, startFrom); err != nil {
				return fmt.Errorf("load task: %w", err)
			}
			fmt.Printf("Task loaded from %s\n", startFrom)
		} else {
			fmt.Println("Socket already running")
		}

		return nil
	}

	// Don't spawn subprocesses from test binaries (same fork-bomb guard as autostart.go).
	if isTestBinary() {
		return errors.New("cannot start background process in test environment")
	}

	// Need to start sockets - spawn a background process
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable: %w", err)
	}

	// Build args for background process
	bgArgs := []string{"start", "--foreground"}
	if startFrom != "" {
		bgArgs = append(bgArgs, "--from", startFrom)
	}

	bgCmd := exec.Command(exe, bgArgs...) //nolint:noctx // exec.Command is intentional: detached process must outlive caller; CommandContext would kill it on cancel
	bgCmd.Dir = cwd
	bgCmd.Stdout = nil // Detach stdout
	bgCmd.Stderr = nil // Detach stderr
	bgCmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Create new session (detach from terminal)
	}

	if err := bgCmd.Start(); err != nil {
		return fmt.Errorf("start background process: %w", err)
	}

	fmt.Printf("Starting kvelmo in background (PID %d)...\n", bgCmd.Process.Pid)

	// Wait for socket to be ready
	if !waitForSocket(wtPath, 10*time.Second) {
		return errors.New("socket failed to start (check logs)")
	}

	// If --from was specified, wait for task to load via status check instead of fixed sleep
	if startFrom != "" {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			client, err := socket.NewClient(wtPath, socket.WithTimeout(1*time.Second))
			if err == nil {
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				resp, err := client.Call(ctx, "status", nil)
				cancel()
				_ = client.Close()
				if err == nil {
					var status struct {
						State string `json:"state"`
					}
					if json.Unmarshal(resp.Result, &status) == nil && status.State != "none" {
						break
					}
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
		if !startJSON {
			fmt.Printf("Task loading from %s\n", startFrom)
		}
	}

	if startJSON {
		result := map[string]string{
			"status":  "ready",
			"socket":  wtPath,
			"workdir": cwd,
		}
		data, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("marshal json: %w", err)
		}
		fmt.Println(string(data))

		return nil
	}

	fmt.Println("Ready")

	return nil
}

// runInForeground runs sockets in foreground (for E2E tests and debugging).
func runInForeground(cwd, globalPath, wtPath string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Optional pprof HTTP endpoint for profiling leaks during long-running
	// foreground sessions. Enabled only when KVELMO_PPROF_ADDR is set, so
	// production is unaffected. Example: KVELMO_PPROF_ADDR=127.0.0.1:6060.
	if addr := os.Getenv("KVELMO_PPROF_ADDR"); addr != "" {
		go func() {
			slog.Info("pprof listening", "addr", addr)
			if err := http.ListenAndServe(addr, nil); err != nil { //nolint:gosec // opt-in profiling endpoint
				slog.Warn("pprof server exited", "error", err)
			}
		}()
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Track global socket errors for later reporting
	var globalErrCh chan error
	if !socket.SocketExists(globalPath) {
		release, err := socket.AcquireGlobalLock(socket.GlobalLockPath())
		if err != nil {
			fmt.Println("Waiting for global socket...")
			time.Sleep(500 * time.Millisecond)
		} else {
			fmt.Println("Starting global socket...")
			globalErrCh = make(chan error, 1)
			go func() {
				defer release()
				global := socket.NewGlobalSocket(globalPath)
				globalErrCh <- global.Start(ctx)
			}()
			// Wait briefly to catch immediate startup failures
			select {
			case err := <-globalErrCh:
				if err != nil && !errors.Is(err, context.Canceled) {
					return fmt.Errorf("global socket: %w", err)
				}
			case <-time.After(100 * time.Millisecond):
				// Socket is starting, proceed
			}
		}
	} else {
		fmt.Println("Global socket already running")
	}

	// Create worker pool for standalone mode (same as serve)
	// Use KvelmoPermissionHandler to allow Write/Edit/Bash for planning/implementation
	registry := agent.NewRegistry()
	if err := claude.RegisterWithPermissionHandler(registry, agent.KvelmoPermissionHandler); err != nil {
		slog.Debug("claude agent not available", "error", err)
	}
	// claude-sdk is registered but broken on the official Anthropic CLI
	// (--sdk-url is rejected). Retained for proxy setups that accept the
	// flag (Claude Code Router and similar). See agent/claudesdk/.
	if err := claudesdk.RegisterWithPermissionHandler(registry, agent.KvelmoPermissionHandler); err != nil {
		slog.Debug("claude-sdk agent not available", "error", err)
	}
	if err := codex.RegisterWithPermissionHandler(registry, agent.KvelmoPermissionHandler); err != nil {
		slog.Debug("codex agent not available", "error", err)
	}
	// Load settings and register API agents
	effective, _, _, _ := settings.LoadEffective(cwd) //nolint:dogsled // Only need effective settings

	// Register the claude-mcp adapter with user settings applied.
	if err := registerClaudeMCP(registry, effective); err != nil {
		slog.Debug("claude-mcp agent not available", "error", err)
	}

	if effective != nil {
		apiCfg := apiagent.DefaultAPIConfig()
		apiCfg.TokenBudget = effective.Agent.TokenBudget

		cfg := effective.Agent.OpenAI
		if err := registry.Register(openai.New(
			openai.Config{APIKey: cfg.APIKey, BaseURL: cfg.BaseURL, Model: cfg.Model}, apiCfg,
		)); err != nil {
			slog.Debug("openai agent registration failed", "error", err)
		}

		acfg := effective.Agent.Anthropic
		if err := registry.Register(anthropic.New(
			anthropic.Config{APIKey: acfg.APIKey, BaseURL: acfg.BaseURL, Model: acfg.Model}, apiCfg,
		)); err != nil {
			slog.Debug("anthropic agent registration failed", "error", err)
		}

		ocfg := effective.Agent.Ollama
		if err := registry.Register(ollama.New(
			ollama.Config{BaseURL: ocfg.BaseURL, Model: ocfg.Model}, apiCfg,
		)); err != nil {
			slog.Debug("ollama agent registration failed", "error", err)
		}
	}
	if effective != nil && effective.Agent.Default != "" {
		if setErr := registry.SetDefault(effective.Agent.Default); setErr != nil {
			slog.Warn("agent.default setting invalid", "agent", effective.Agent.Default, "error", setErr)
		}
	}

	poolCfg := worker.DefaultPoolConfig()
	poolCfg.Agents = registry
	pool := worker.NewPool(poolCfg)
	if err := pool.Start(); err != nil {
		return fmt.Errorf("start worker pool: %w", err)
	}
	defer func() { _ = pool.Stop() }()

	fmt.Printf("Starting worktree socket for %s\n", cwd)
	wt, err := socket.NewWorktreeSocket(socket.WorktreeConfig{
		WorktreePath: cwd,
		SocketPath:   wtPath,
		GlobalPath:   globalPath,
		Pool:         pool,
	})
	if err != nil {
		return fmt.Errorf("create worktree socket: %w", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- wt.Start(ctx)
	}()

	// Wait for socket to be ready before loading task
	if !waitForSocket(wtPath, 5*time.Second) {
		return errors.New("worktree socket failed to start")
	}

	// Add agent worker in background
	defaultAgent := ""
	if effective != nil {
		defaultAgent = effective.Agent.Default
	}
	go func() {
		if _, err := pool.AddAgentWorker(ctx, defaultAgent, true); err != nil {
			fmt.Printf("Warning: Failed to add agent worker: %v\n", err)
			fmt.Println("Jobs will run in simulation mode.")
			_ = pool.AddDefaultWorker("")
		}
	}()

	// If --from was specified, load the task via RPC
	if startFrom != "" {
		if err := loadTaskViaRPC(wtPath, startFrom); err != nil {
			fmt.Printf("Warning: failed to load task: %v\n", err)
		} else {
			fmt.Printf("Task loaded from %s\n", startFrom)
		}
	}

	fmt.Println("Ready. Press Ctrl+C to stop.")

	select {
	case sig := <-sigCh:
		fmt.Printf("\nReceived %v, shutting down...\n", sig)
		cancel()
		select {
		case <-errCh:
		case <-time.After(3 * time.Second):
		}
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("worktree socket: %w", err)
		}
	case err := <-globalErrCh:
		// globalErrCh is nil if we didn't start the global socket, so this case won't fire
		if err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("global socket: %w", err)
		}
	}

	return nil
}

// isGitRepository checks if the given path is inside a git repository.
func isGitRepository(path string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", path, "rev-parse", "--git-dir")

	return cmd.Run() == nil
}

// waitForSocket waits for a socket to become available.
func waitForSocket(socketPath string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if socket.SocketExists(socketPath) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}

	return false
}

// loadTaskViaRPC connects to the worktree socket and loads a task.
func loadTaskViaRPC(socketPath, source string) error {
	client, err := socket.NewClient(socketPath, socket.WithTimeout(30*time.Second))
	if err != nil {
		return fmt.Errorf("connect to socket: %w", err)
	}
	defer func() { _ = client.Close() }()

	params := map[string]any{
		"source":       source,
		"auto_advance": startAuto,
	}
	if len(startSkip) > 0 {
		params["skip_phases"] = startSkip
	}

	// Build and validate context items from CLI flags
	contextItems := buildContextItems()
	if err := validateContextItems(contextItems); err != nil {
		return err
	}
	if len(contextItems) > 0 {
		params["context_items"] = contextItems
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err = client.Call(ctx, "start", params)
	if err != nil {
		return fmt.Errorf("call start: %w", err)
	}

	return nil
}

// (registerClaudeMCP moved to agent_registry.go so serve, changelog, pipe
// can call the same helper. Omitting claudemcp from any registration site
// breaks the default `agent.default: claude-mcp` config.)
