package socket

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"github.com/valksor/kvelmo/pkg/agent"
	"github.com/valksor/kvelmo/pkg/agent/strategy"
	"github.com/valksor/kvelmo/pkg/meta"
	"github.com/valksor/kvelmo/pkg/worker"
)

// WorktreeInfo represents a registered worktree/project.
type WorktreeInfo struct {
	ID         string    `json:"id"`
	Path       string    `json:"path"`
	SocketPath string    `json:"socket_path,omitempty"`
	State      string    `json:"state"`
	LastSeen   time.Time `json:"last_seen,omitzero"`
	Healthy    *bool     `json:"healthy,omitempty"`
	LastPing   time.Time `json:"last_ping,omitzero"`
	failCount  int       // consecutive ping failures (not serialized)
}

// ProjectListResult is the response for projects.list.
type ProjectListResult struct {
	Projects []WorktreeInfo `json:"projects"`
}

// RegisterParams is the request for projects.register.
type RegisterParams struct {
	Path       string `json:"path"`
	SocketPath string `json:"socket_path"`
}

// UnregisterParams is the request for projects.unregister.
type UnregisterParams struct {
	ID string `json:"id"`
}

// WorkerInfo represents a worker for API responses.
type WorkerInfo struct {
	ID         string `json:"id"`
	AgentName  string `json:"agent_name"`
	Status     string `json:"status"`
	CurrentJob string `json:"current_job,omitempty"`
	IsDefault  bool   `json:"is_default"`
}

// WorkersStats contains aggregate worker pool statistics.
type WorkersStats struct {
	TotalWorkers     int `json:"total_workers"`
	AvailableWorkers int `json:"available_workers"`
	WorkingWorkers   int `json:"working_workers"`
	QueuedJobs       int `json:"queued_jobs"`
	InProgressJobs   int `json:"in_progress_jobs"`
	CompletedJobs    int `json:"completed_jobs"`
	FailedJobs       int `json:"failed_jobs"`
}

// WorkersListResult is the response for workers.list.
type WorkersListResult struct {
	Workers []WorkerInfo `json:"workers"`
	Stats   WorkersStats `json:"stats"`
}

// AddWorkerParams is the request for workers.add.
type AddWorkerParams struct {
	Agent string `json:"agent"`
}

// RemoveWorkerParams is the request for workers.remove.
type RemoveWorkerParams struct {
	ID string `json:"id"`
}

// GlobalSocket manages the global kvelmo socket.
// Per flow_v2.md: "Global socket handles project registry, worker pool, job queue".
//
//nolint:containedctx // Lifecycle context required for managed socket coordination
type GlobalSocket struct {
	server      *Server
	worktrees   map[string]*WorktreeInfo
	pool        *worker.Pool
	projectsDir string // directory for projects.json; defaults to BaseDir()
	mu          sync.RWMutex

	// Managed worktree sockets (created on-demand with worker pool access)
	wtSockets   map[string]*WorktreeSocket
	wtSocketsMu sync.RWMutex
	ctx         context.Context    // For managed socket lifecycle
	cancel      context.CancelFunc // To stop managed sockets on shutdown
}

// GlobalSocketConfig configures the global socket.
type GlobalSocketConfig struct {
	SocketPath string
	Pool       *worker.Pool
}

// NewGlobalSocket creates a new global socket.
func NewGlobalSocket(path string) *GlobalSocket {
	return NewGlobalSocketWithPool(path, nil)
}

// NewGlobalSocketWithPool creates a global socket with worker pool integration.
func NewGlobalSocketWithPool(path string, pool *worker.Pool) *GlobalSocket {
	ctx, cancel := context.WithCancel(context.Background())
	g := &GlobalSocket{
		server:      NewServer(path),
		worktrees:   make(map[string]*WorktreeInfo),
		pool:        pool,
		projectsDir: filepath.Dir(path),
		wtSockets:   make(map[string]*WorktreeSocket),
		ctx:         ctx,
		cancel:      cancel,
	}

	// Load existing projects from file
	g.loadProjectsFromFile()

	g.registerHandlers()

	return g
}

// UseMiddleware inserts middleware after Recovery so it runs before
// rate limiting, metrics, and activity logging. Use this for auth
// middleware that should reject requests before they are logged.
func (g *GlobalSocket) UseMiddleware(mw ...Middleware) {
	g.server.UseAfterRecovery(mw...)
}

func (g *GlobalSocket) registerHandlers() {
	// Ping
	g.server.Handle("ping", g.handlePing)

	// System info
	g.server.Handle("system.docsURL", g.handleDocsURL)
	g.server.Handle("system.diagnose", g.handleDiagnose)
	g.server.Handle("system.health", g.handleSystemHealth)

	// Project management
	g.server.Handle("projects.list", g.handleListProjects)
	g.server.Handle("tasks.list", g.handleTasksList)
	g.server.Handle("projects.register", g.handleRegisterProject)
	g.server.Handle("projects.unregister", g.handleUnregisterProject)

	// Worktree management
	g.server.Handle("worktrees.create", g.handleWorktreeCreate)

	// Worker management
	g.server.Handle("workers.list", g.handleListWorkers)
	g.server.Handle("workers.add", g.handleAddWorker)
	g.server.Handle("workers.remove", g.handleRemoveWorker)
	g.server.Handle("workers.stats", g.handleWorkerStats)

	// Metrics
	g.server.Handle("metrics", g.handleMetrics)
	g.server.Handle("metrics.history", g.handleMetricsHistory)

	// Activity log
	g.server.Handle("activity.query", g.handleActivityQuery)

	// Job management
	g.server.Handle("jobs.list", g.handleListJobs)
	g.server.Handle("jobs.get", g.handleGetJob)

	// Settings management (new - two-tier with schema)
	g.server.Handle("settings.get", g.handleSettingsGet)
	g.server.Handle("settings.set", g.handleSettingsSet)

	// File browsing
	g.server.Handle("browse", g.handleBrowse)

	// Chat (uses worker pool for AI responses)
	g.server.HandleWithConn("chat.send", g.handleChatSendEnhanced) // Enhanced with mentions + streaming
	g.server.Handle("chat.stop", g.handleChatStop)                 // Stop current chat (keep worker)
	g.server.Handle("chat.history", g.handleChatHistory)           // Get chat history for task
	g.server.Handle("chat.clear", g.handleChatClear)               // Clear chat history for task

	// Files (for mentions/autocomplete)
	g.server.Handle("files.list", g.handleFilesList)
	g.server.Handle("files.search", g.handleFilesSearch)

	// Browser tools (playwright-cli wrapper)
	g.server.Handle("browser.install", g.handleBrowserInstall)
	g.server.Handle("browser.status", g.handleBrowserStatus)
	g.server.Handle("browser.config.get", g.handleBrowserConfigGet)
	g.server.Handle("browser.config.set", g.handleBrowserConfigSet)
	g.server.Handle("browser.snapshot", g.handleBrowserSnapshot)
	g.server.Handle("browser.eval", g.handleBrowserEval)
	g.server.Handle("browser.console", g.handleBrowserConsole)
	g.server.Handle("browser.network", g.handleBrowserNetwork)
	g.server.Handle("browser.screenshot", g.handleBrowserScreenshot)
	g.server.Handle("browser.navigate", g.handleBrowserNavigate)
	g.server.Handle("browser.click", g.handleBrowserClick)
	g.server.Handle("browser.type", g.handleBrowserType)
	g.server.Handle("browser.wait", g.handleBrowserWait)
	g.server.Handle("browser.fill", g.handleBrowserFill)
	g.server.Handle("browser.select", g.handleBrowserSelect)
	g.server.Handle("browser.hover", g.handleBrowserHover)
	g.server.Handle("browser.focus", g.handleBrowserFocus)
	g.server.Handle("browser.scroll", g.handleBrowserScroll)
	g.server.Handle("browser.press", g.handleBrowserPress)
	g.server.Handle("browser.back", g.handleBrowserBack)
	g.server.Handle("browser.forward", g.handleBrowserForward)
	g.server.Handle("browser.reload", g.handleBrowserReload)
	g.server.Handle("browser.dialog", g.handleBrowserDialog)
	g.server.Handle("browser.upload", g.handleBrowserUpload)
	g.server.Handle("browser.pdf", g.handleBrowserPDF)

	// Memory
	g.server.Handle("memory.search", g.handleMemorySearch)
	g.server.Handle("memory.stats", g.handleMemoryStats)
	g.server.Handle("memory.clear", g.handleMemoryClear)
	g.server.Handle("memory.outcomes", g.handleMemoryOutcomes)

	// Agent status
	g.server.Handle("agent.status", g.handleAgentStatus)
	g.server.Handle("strategy.list", g.handleStrategyList)

	// Provider token testing and login
	g.server.Handle("providers.test", g.handleProvidersTest)
	g.server.Handle("providers.list", g.handleProvidersList)
	g.server.Handle("provider.login", g.handleProviderLogin)

	// Configuration validation
	g.server.Handle("config.validate", g.handleConfigValidate)
	g.server.Handle("config.check", g.handleConfigCheck)

	// Security scanning
	g.server.Handle("security.scan", g.handleSecurityScan)

	// Recordings
	g.server.Handle("recordings.list", g.handleRecordingsList)
	g.server.Handle("recordings.view", g.handleRecordingsView)

	// Onboarding
	g.server.Handle("onboarding.status", g.handleOnboardingStatus)
	g.server.Handle("onboarding.complete", g.handleOnboardingComplete)
	g.server.Handle("onboarding.reset", g.handleOnboardingReset)

	// Backup
	g.server.Handle("backup.create", g.handleBackupCreate)
	g.server.Handle("backup.list", g.handleBackupList)
	g.server.Handle("backup.restore", g.handleBackupRestore)

	// Notifications
	g.server.Handle("notify.test", g.handleNotifyTest)

	// Export
	g.server.Handle("export", g.handleExport)

	// Compliance reports
	g.server.Handle("report.generate", g.handleReportGenerate)

	// Catalog
	g.server.Handle("catalog.list", g.handleCatalogList)
	g.server.Handle("catalog.get", g.handleCatalogGet)
	g.server.Handle("catalog.import", g.handleCatalogImport)

	// Batch operations across worktrees
	g.server.Handle("tasks.batch", g.handleBatch)

	// Access token management
	g.server.Handle("access.token.list", g.handleAccessTokenList)
	g.server.Handle("access.token.create", g.handleAccessTokenCreate)
	g.server.Handle("access.token.revoke", g.handleAccessTokenRevoke)

	// Task groups (cross-repo coordination)
	g.server.Handle("taskgroup.create", g.handleTaskGroupCreate)
	g.server.Handle("taskgroup.list", g.handleTaskGroupList)
	g.server.Handle("taskgroup.status", g.handleTaskGroupStatus)
	g.server.Handle("taskgroup.add", g.handleTaskGroupAdd)
	g.server.Handle("taskgroup.submit", g.handleTaskGroupSubmit)
	g.server.Handle("taskgroup.remove", g.handleTaskGroupRemove)
}

// --- Ping ---

func (g *GlobalSocket) handlePing(ctx context.Context, req *Request) (*Response, error) {
	return NewResultResponse(req.ID, map[string]string{
		"status":  "ok",
		"version": meta.Version,
		"commit":  meta.Commit,
	})
}

// --- System Info ---

func (g *GlobalSocket) handleDocsURL(ctx context.Context, req *Request) (*Response, error) {
	return NewResultResponse(req.ID, map[string]string{
		"url":     meta.DocsURL(),
		"version": meta.Version,
	})
}

// --- Agent Status ---

func (g *GlobalSocket) handleAgentStatus(_ context.Context, req *Request) (*Response, error) {
	result := agent.RunPreflight() //nolint:contextcheck // RunPreflight manages its own timeouts internally

	// If pool has a worker with a connected agent, that's a stronger availability signal
	if g.pool != nil {
		for _, w := range g.pool.ListWorkers() {
			if w.Agent != nil && w.Agent.Connected() {
				result.SimulationMode = false
				result.AgentAvailable = true

				break
			}
		}
	}

	return NewResultResponse(req.ID, result)
}

func (g *GlobalSocket) handleStrategyList(_ context.Context, req *Request) (*Response, error) {
	return NewResultResponse(req.ID, strategy.List())
}

// --- Lifecycle ---

func (g *GlobalSocket) Start(ctx context.Context) error {
	go g.StartHealthChecks(ctx)

	return g.server.Start(ctx)
}

func (g *GlobalSocket) Stop() error {
	// Cancel context to stop all managed worktree sockets
	if g.cancel != nil {
		g.cancel()
	}

	// Collect sockets to stop, then release lock before stopping
	// This avoids potential deadlock if Stop() tries to acquire wtSocketsMu
	g.wtSocketsMu.Lock()
	socketsToStop := make([]*WorktreeSocket, 0, len(g.wtSockets))
	for _, wt := range g.wtSockets {
		socketsToStop = append(socketsToStop, wt)
	}
	g.wtSockets = make(map[string]*WorktreeSocket)
	g.wtSocketsMu.Unlock()

	// Stop sockets without holding the lock
	for _, wt := range socketsToStop {
		_ = wt.Stop()
	}

	return g.server.Stop()
}

func (g *GlobalSocket) Server() *Server {
	return g.server
}

func (g *GlobalSocket) Pool() *worker.Pool {
	return g.pool
}

func (g *GlobalSocket) SetPool(pool *worker.Pool) {
	g.pool = pool
}
