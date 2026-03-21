package commands

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/pkg/activitylog"
	"github.com/valksor/kvelmo/pkg/agent"
	"github.com/valksor/kvelmo/pkg/agent/claude"
	"github.com/valksor/kvelmo/pkg/agent/codex"
	"github.com/valksor/kvelmo/pkg/catalog"
	"github.com/valksor/kvelmo/pkg/meta"
	"github.com/valksor/kvelmo/pkg/metrics"
	"github.com/valksor/kvelmo/pkg/notify"
	"github.com/valksor/kvelmo/pkg/settings"
	"github.com/valksor/kvelmo/pkg/socket"
	"github.com/valksor/kvelmo/pkg/web"
	"github.com/valksor/kvelmo/pkg/worker"
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
  %[1]s serve --open       # Open browser automatically`, meta.Name)
	ServeCmd.Flags().IntVarP(&servePort, "port", "p", 0, "Server port (default: 6337, 0 = random)")
	ServeCmd.Flags().StringVar(&serveStatic, "static", "", "Static files directory")
	ServeCmd.Flags().BoolVarP(&serveVerbose, "verbose", "v", false, "Verbose output")
	ServeCmd.Flags().BoolVar(&serveOpen, "open", false, "Open browser automatically")
	ServeCmd.Flags().StringVar(&serveTLSCert, "tls-cert", "", "TLS certificate file path")
	ServeCmd.Flags().StringVar(&serveTLSKey, "tls-key", "", "TLS key file path")
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
		// Another instance has the lock
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

		// Add a default worker with auto-detected agent (actually connects to Claude/Codex CLI)
		// The default worker cannot be removed
		// Empty string for agent name triggers auto-detection (claude > codex > error)
		_, err := pool.AddAgentWorker(ctx, "", true)
		if err != nil {
			fmt.Printf("Warning: Failed to add agent worker: %v\n", err)
			fmt.Println("Jobs will run in simulation mode until a worker is connected.")
			// Fall back to default worker (simulation mode)
			_ = pool.AddDefaultWorker("")
		}

		// Create global socket with pool
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

		// Load settings for optional features
		cfg, _, _, _ := settings.LoadEffective("")

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
		if err := webServer.Start(); err != nil {
			fmt.Printf("Web server error: %v\n", err)
		}
	}()

	// Open browser if requested
	if serveOpen {
		openBrowser(webServer.URL())
	}

	// Wait for signal
	<-sigCh
	fmt.Println("\nShutting down...")

	// Cancel context to stop all components
	cancel()

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	_ = webServer.Shutdown(shutdownCtx)

	if globalSocket != nil {
		_ = globalSocket.Stop()
	}

	fmt.Println("Goodbye.")

	return nil
}

// resolvePort determines the actual port to use.
// If port is explicitly set, use it. Otherwise try preferred port, fallback to random.
func resolvePort(cmd *cobra.Command, explicit int) int {
	// If explicit port specified via flag, use it
	if cmd.Flags().Changed("port") {
		return explicit
	}

	// Try preferred port
	if portAvailable("localhost", DefaultPreferredPort) {
		return DefaultPreferredPort
	}

	// Fallback to random
	fmt.Printf("Port %d in use, using random available port\n", DefaultPreferredPort)

	return 0
}

// portAvailable checks if a port is available for binding.
func portAvailable(host string, port int) bool {
	addr := fmt.Sprintf("%s:%d", host, port)
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", addr)
	if err != nil {
		return false
	}
	_ = ln.Close()

	return true
}

// findStaticDir locates the static files directory.
func findStaticDir(explicit string) string {
	if explicit != "" {
		return explicit
	}

	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	candidates := []string{
		filepath.Join(cwd, "web", "dist"),
		"web/dist",
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c
		}
	}

	return ""
}

// openBrowser opens the specified URL in the default browser.
//
//nolint:noctx // exec.Command is intentional: the browser process must outlive the caller; CommandContext would kill it on cancel
func openBrowser(url string) {
	var cmd *exec.Cmd

	switch {
	case fileExists("/usr/bin/open"): // macOS
		cmd = exec.Command("/usr/bin/open", url)
	case fileExists("/usr/bin/xdg-open"): // Linux
		cmd = exec.Command("/usr/bin/xdg-open", url)
	default:
		// Fallback: try "open" from PATH (macOS) or "xdg-open" (Linux)
		if path, err := exec.LookPath("open"); err == nil {
			cmd = exec.Command(path, url)
		} else if path, err := exec.LookPath("xdg-open"); err == nil {
			cmd = exec.Command(path, url)
		} else {
			return
		}
	}

	_ = cmd.Start()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}
