package commands

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/agent"
	"github.com/valksor/kvelmo/agent/anthropic"
	"github.com/valksor/kvelmo/agent/apiagent"
	"github.com/valksor/kvelmo/agent/claude"
	"github.com/valksor/kvelmo/agent/codex"
	"github.com/valksor/kvelmo/agent/ollama"
	"github.com/valksor/kvelmo/agent/openai"
	"github.com/valksor/kvelmo/internal/activitylog"
	"github.com/valksor/kvelmo/internal/catalog"
	"github.com/valksor/kvelmo/internal/notify"
	"github.com/valksor/kvelmo/internal/socket"
	"github.com/valksor/kvelmo/internal/taskgroup"
	"github.com/valksor/kvelmo/internal/web"
	"github.com/valksor/kvelmo/internal/worker"
	"github.com/valksor/kvelmo/meta"
	"github.com/valksor/kvelmo/metrics"
	"github.com/valksor/kvelmo/paths"
	"github.com/valksor/kvelmo/settings"
)

// DefaultPreferredPort is the default port for the web server.
// Falls back to a random port if this port is already in use.
const DefaultPreferredPort = 6337

var (
	servePort    int
	serveStatic  string
	serveVerbose bool
	serveOpen    bool
	serveTLSCert string
	serveTLSKey  string
	serveAPIOnly bool
)

var ServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start " + meta.Name + " web UI",
	RunE:  runServe,
}

func init() {
	ServeCmd.Long = fmt.Sprintf(`Start the %[1]s web UI server.

The web UI provides a project picker to manage and connect to projects.
Projects are added/removed via the web interface.

The server listens on port 6337 by default.
If port 6337 is in use, a random available port is selected automatically.

Examples:
  %[1]s serve              # Port 6337 (or random if taken)
  %[1]s serve --port 8080  # Specific port
  %[1]s serve --open       # Open browser automatically
  %[1]s serve --api        # API-only mode (no web UI)`, meta.Name)
	ServeCmd.Flags().IntVarP(&servePort, "port", "p", 0, "Server port (default: 6337, 0 = random)")
	ServeCmd.Flags().StringVar(&serveStatic, "static", "", "Static files directory")
	ServeCmd.Flags().BoolVarP(&serveVerbose, "verbose", "v", false, "Verbose output")
	ServeCmd.Flags().BoolVar(&serveOpen, "open", false, "Open browser automatically")
	ServeCmd.Flags().StringVar(&serveTLSCert, "tls-cert", "", "TLS certificate file path")
	ServeCmd.Flags().StringVar(&serveTLSKey, "tls-key", "", "TLS key file path")
	ServeCmd.Flags().BoolVar(&serveAPIOnly, "api", false, "API-only mode (no web UI)")
}

func runServe(cmd *cobra.Command, args []string) error {
	debugTiming := os.Getenv("KVELMO_DEBUG_TIMING") != ""
	phaseStart := time.Now()

	// Pre-flight check: verify system setup
	preflight := agent.RunPreflight()
	agent.PrintPreflight(preflight)
	runStartupChecks()

	if debugTiming {
		fmt.Printf("[timing] preflight: %v\n", time.Since(phaseStart))
		phaseStart = time.Now()
	}

	// Resolve port (6337 preferred, fallback to random)
	port := resolvePort(cmd, servePort)

	// Find static directory
	staticDir := findStaticDir(serveStatic)

	if serveVerbose && staticDir != "" {
		fmt.Printf("Static files: %s\n", staticDir)
	}

	// Ensure socket directories exist
	if err := socket.EnsureDir(); err != nil {
		return fmt.Errorf("create socket directories: %w", err)
	}

	if debugTiming {
		fmt.Printf("[timing] socket setup: %v\n", time.Since(phaseStart))
		phaseStart = time.Now()
	}

	// Create context for coordinated shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start GlobalSocket with worker pool
	globalPath := socket.GlobalSocketPath()
	lockPath := socket.GlobalLockPath()
	var globalSocket *socket.GlobalSocket

	// Try to acquire lock - if we get it, we're the primary instance
	release, err := socket.AcquireGlobalLock(lockPath)
	if err != nil {
		// Another instance has the lock — check if it's an older version
		if err := replaceOlderSocket(ctx, globalPath); err != nil {
			slog.Debug("version check on running socket", "error", err)
		}

		// Re-try lock after potential replacement
		release, err = socket.AcquireGlobalLock(lockPath)
	}
	if err != nil {
		// Still can't get lock — run as secondary
		if serveVerbose {
			fmt.Println("Global socket already running (another instance)")
		}
	} else {
		// We got the lock - clean up any stale socket and start fresh
		if socket.SocketExists(globalPath) {
			_ = os.Remove(globalPath)
		}

		// Create worker pool with available agents registered.
		// Use KvelmoPermissionHandler to allow Write/Edit/Bash for planning/implementation.
		registry := agent.NewRegistry()
		if err := claude.RegisterWithPermissionHandler(registry, agent.KvelmoPermissionHandler); err != nil {
			slog.Debug("claude agent not available", "error", err)
		}
		if err := codex.RegisterWithPermissionHandler(registry, agent.KvelmoPermissionHandler); err != nil {
			slog.Debug("codex agent not available", "error", err)
		}

		// Apply agent.default from settings
		effective, _, _, _ := settings.LoadEffective("")

		// Register API-based agents from settings
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

		if debugTiming {
			fmt.Printf("[timing] worker pool setup: %v\n", time.Since(phaseStart))
			phaseStart = time.Now()
		}

		poolCfg := worker.DefaultPoolConfig()
		poolCfg.Agents = registry
		if effective != nil && len(effective.Agent.Allowed) > 0 {
			poolCfg.AllowedAgents = effective.Agent.Allowed
		}
		pool := worker.NewPool(poolCfg)
		if err := pool.Start(); err != nil {
			return fmt.Errorf("start worker pool: %w", err)
		}

		// Ensure pool cleanup on early return
		var poolCleaned bool
		defer func() {
			if !poolCleaned && globalSocket == nil {
				_ = pool.Stop()
			}
		}()

		// Create global socket with pool (before agent connection so the web server is available immediately)
		globalSocket = socket.NewGlobalSocketWithPool(globalPath, pool)
		poolCleaned = true // Pool is now managed by globalSocket

		go func() {
			defer release()
			if err := globalSocket.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
				fmt.Printf("Global socket error: %v\n", err)
			}
		}()
		time.Sleep(100 * time.Millisecond)

		if debugTiming {
			fmt.Printf("[timing] global socket start: %v\n", time.Since(phaseStart))
			phaseStart = time.Now()
		}

		// Connect agent worker in background so the web server is available immediately.
		// Jobs run in simulation mode until the agent is ready.
		// Use the configured default agent (if set) instead of auto-detecting.
		defaultAgent := ""
		if effective != nil {
			defaultAgent = effective.Agent.Default
		}
		go func() {
			slog.Info("adding agent worker", "agent", defaultAgent, "registered", registry.List())
			_, err := pool.AddAgentWorker(ctx, defaultAgent, true)
			if err != nil {
				slog.Error("failed to add agent worker", "agent", defaultAgent, "error", err)
				fmt.Printf("Warning: Failed to add agent worker: %v\n", err)
				fmt.Println("Jobs will run in simulation mode until a worker is connected.")
				_ = pool.AddDefaultWorker("")
			}
			// Notify connected clients that workers changed
			globalSocket.BroadcastWorkerChanged()
		}()

		// Pre-warm memory adapter in background so the server accepts connections
		// immediately. Cybertron model download completes asynchronously before
		// the first memory.search call rather than blocking startup.
		go func() {
			socket.PrewarmMemory(ctx)
		}()

		// Start metrics persistence
		metricsPersister := metrics.NewPersister(metrics.Global(), "", 0)
		metricsPersister.Load()
		go metricsPersister.Start(ctx)

		// Reuse effective settings loaded earlier
		cfg := effective

		// Start notification engine if enabled
		if cfg.Notify.Enabled != nil && *cfg.Notify.Enabled && len(cfg.Notify.Webhooks) > 0 {
			endpoints := make([]notify.WebhookEndpoint, len(cfg.Notify.Webhooks))
			for i, wh := range cfg.Notify.Webhooks {
				endpoints[i] = notify.WebhookEndpoint{
					URL:    wh.URL,
					Format: notify.Format(wh.Format),
					Events: wh.Events,
				}
			}
			n := notify.New(endpoints, cfg.Notify.OnFailure)
			socket.SetNotifier(n)
			go n.Start(ctx)
		}

		// Initialize catalog (always available)
		socket.SetCatalog(catalog.New(""))

		// Initialize task group coordinator
		groupStore := taskgroup.NewStore(filepath.Join(paths.BaseDir(), "groups"))
		socket.SetTaskGroupCoordinator(taskgroup.NewCoordinator(groupStore))

		// Start activity log if enabled
		if cfg.Storage.ActivityLog.Enabled {
			actLog, logErr := activitylog.New("", cfg.Storage.ActivityLog.MaxFiles)
			if logErr != nil {
				fmt.Printf("Warning: Failed to start activity log: %v\n", logErr)
			} else {
				globalSocket.SetActivityLog(actLog)
				go actLog.Start(ctx)
			}
		}

		// Start time-series metrics if enabled
		if cfg.Storage.MetricsHistory.Enabled {
			interval := time.Duration(cfg.Storage.MetricsHistory.IntervalMin) * time.Minute
			ts := metrics.NewTimeSeriesStore(metrics.Global(), "", interval, cfg.Storage.MetricsHistory.RetentionDays)
			socket.SetTimeSeriesStore(ts)
			go ts.Start(ctx)
		}
	}

	// Create web server with worktree creator
	var webOpts []web.ServerOption
	if globalSocket != nil {
		// Primary instance: use direct access to global socket
		webOpts = append(webOpts, web.WithWorktreeCreator(globalSocket))
	} else {
		// Secondary instance: use RPC client to communicate with primary
		fmt.Println("  Running as secondary instance, using global socket client for worktree creation")
		client := web.NewWorktreeCreatorClient(socket.GlobalSocketPath())
		webOpts = append(webOpts, web.WithWorktreeCreator(client))
	}
	webOpts = append(webOpts, web.WithGlobalSocketPath(socket.GlobalSocketPath()))
	if serveTLSCert != "" && serveTLSKey != "" {
		webOpts = append(webOpts, web.WithTLS(serveTLSCert, serveTLSKey))
	}
	if serveAPIOnly {
		webOpts = append(webOpts, web.WithAPIOnly())
	}
	webServer, err := web.NewServer(staticDir, port, webOpts...)
	if err != nil {
		return fmt.Errorf("create web server: %w", err)
	}

	if debugTiming {
		fmt.Printf("[timing] web server setup: %v\n", time.Since(phaseStart))
	}

	// Signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start web server
	go func() {
		fmt.Printf("\n  %s running at %s\n\n", meta.Name, webServer.URL())
		if err := webServer.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Printf("Web server error: %v\n", err)
		}
	}()

	// Open browser if requested (skip in API-only mode)
	if serveOpen && !serveAPIOnly {
		openBrowser(webServer.URL())
	}

	// Wait for signal
	<-sigCh
	fmt.Println("\nShutting down...")

	// Cancel context to stop all components
	cancel()

	// Graceful shutdown — stop socket and web server concurrently
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if globalSocket != nil {
		_ = globalSocket.Stop()
	}

	_ = webServer.Shutdown(shutdownCtx)

	fmt.Println("Goodbye.")

	return nil
}
