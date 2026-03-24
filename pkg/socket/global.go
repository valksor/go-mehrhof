package socket

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/valksor/kvelmo/pkg/agent"
	"github.com/valksor/kvelmo/pkg/agent/strategy"
	"github.com/valksor/kvelmo/pkg/browser"
	"github.com/valksor/kvelmo/pkg/conductor"
	"github.com/valksor/kvelmo/pkg/filter"
	"github.com/valksor/kvelmo/pkg/meta"
	"github.com/valksor/kvelmo/pkg/metrics"
	"github.com/valksor/kvelmo/pkg/notify"
	"github.com/valksor/kvelmo/pkg/page"
	"github.com/valksor/kvelmo/pkg/provider"
	"github.com/valksor/kvelmo/pkg/search"
	"github.com/valksor/kvelmo/pkg/settings"
	"github.com/valksor/kvelmo/pkg/worker"
)

// WorktreeInfo represents a registered worktree/project.
type WorktreeInfo struct {
	ID         string    `json:"id"`
	Path       string    `json:"path"`
	SocketPath string    `json:"socket_path,omitempty"`
	State      string    `json:"state"`
	LastSeen   time.Time `json:"last_seen,omitempty"`
	Healthy    *bool     `json:"healthy,omitempty"`
	LastPing   time.Time `json:"last_ping,omitempty"`
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
		return
	}
	_ = os.WriteFile(g.projectsFile(), data, 0o644)
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

	// Timeline
	g.server.Handle("timeline.recent", g.handleTimelineRecent)
	g.server.Handle("timeline.task", g.handleTimelineTask)
	g.server.Handle("timeline.summary", g.handleTimelineSummary)

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

// --- Provider Token Testing ---

func (g *GlobalSocket) handleProvidersList(_ context.Context, req *Request) (*Response, error) {
	type providerInfo struct {
		Name        string `json:"name"`
		Label       string `json:"label"`
		EnvVar      string `json:"env_var"`
		HelpURL     string `json:"help_url"`
		HelpSteps   string `json:"help_steps"`
		Scopes      string `json:"scopes"`
		TokenPrefix string `json:"token_prefix"`
		Configured  bool   `json:"configured"`
	}

	providers := []providerInfo{}
	for name, cfg := range providerLoginConfigs {
		token := resolveProviderToken(name)
		providers = append(providers, providerInfo{
			Name:        name,
			Label:       cfg.Name,
			EnvVar:      cfg.EnvVar,
			HelpURL:     cfg.HelpURL,
			HelpSteps:   cfg.HelpSteps,
			Scopes:      cfg.Scopes,
			TokenPrefix: cfg.TokenPrefix,
			Configured:  token != "",
		})
	}

	// Sort alphabetically for stable output
	slices.SortFunc(providers, func(a, b providerInfo) int {
		return strings.Compare(a.Name, b.Name)
	})

	return NewResultResponse(req.ID, map[string]any{
		"providers": providers,
	})
}

func (g *GlobalSocket) handleProvidersTest(ctx context.Context, req *Request) (*Response, error) {
	var params struct {
		Provider string `json:"provider"`
		Token    string `json:"token"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	if params.Provider == "" {
		return NewResultResponse(req.ID, map[string]any{
			"ok":    false,
			"error": "Provider is required",
		})
	}

	// Resolve token: use configured token if not explicitly provided
	token := params.Token
	if token == "" || token == "__use_configured__" {
		token = resolveProviderToken(params.Provider)
	}
	if token == "" {
		return NewResultResponse(req.ID, map[string]any{
			"ok":     false,
			"detail": "No token configured",
		})
	}

	ok, detail := testProviderToken(ctx, params.Provider, token)

	return NewResultResponse(req.ID, map[string]any{
		"ok":     ok,
		"detail": detail,
	})
}

// providerLoginConfigs is the canonical provider login configuration from pkg/provider.
var providerLoginConfigs = provider.LoginConfigs

func (g *GlobalSocket) handleProviderLogin(ctx context.Context, req *Request) (*Response, error) {
	var params struct {
		Provider string `json:"provider"`
		Token    string `json:"token"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	if params.Provider == "" {
		return NewErrorResponse(req.ID, -32602, "provider is required"), nil
	}
	if params.Token == "" {
		return NewErrorResponse(req.ID, -32602, "token is required"), nil
	}

	cfg, ok := providerLoginConfigs[params.Provider]
	if !ok {
		return NewErrorResponse(req.ID, -32602, "unknown provider: "+params.Provider), nil
	}

	// Validate token against provider API
	valid, detail := testProviderToken(ctx, params.Provider, params.Token)

	// Save token regardless of validation result (matches CLI behavior)
	if err := settings.SaveEnvVar(settings.ScopeGlobal, "", cfg.EnvVar, params.Token); err != nil {
		return NewErrorResponse(req.ID, -32603, "failed to save token: "+err.Error()), nil //nolint:nilerr // RPC error response pattern — Go error is nil, JSON-RPC error is in the response
	}

	envPath, envErr := settings.GlobalEnvPath()
	if envErr != nil {
		slog.Warn("failed to resolve global env path", "error", envErr)
	}

	return NewResultResponse(req.ID, map[string]any{
		"saved":    true,
		"valid":    valid,
		"detail":   detail,
		"env_path": envPath,
		"masked":   settings.MaskToken(params.Token),
	})
}

// --- Config Validation ---

func (g *GlobalSocket) handleConfigValidate(_ context.Context, req *Request) (*Response, error) {
	type check struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Detail string `json:"detail,omitempty"`
		Fix    string `json:"fix,omitempty"`
	}

	type result struct {
		Valid  bool    `json:"valid"`
		Checks []check `json:"checks"`
	}

	res := result{Valid: true}

	// Check settings can be loaded.
	effective := settings.DefaultSettings()
	global, settingsErr := settings.LoadGlobal()
	if settingsErr != nil {
		res.Valid = false
		res.Checks = append(res.Checks, check{
			Name:   "Settings",
			Status: "error",
			Detail: settingsErr.Error(),
			Fix:    "Run 'kvelmo config init' or fix YAML syntax in config file",
		})
	} else {
		if global != nil {
			settings.Merge(effective, global)
		}
		res.Checks = append(res.Checks, check{
			Name:   "Settings",
			Status: "ok",
			Detail: "valid",
		})
	}

	// Run preflight checks (git, agent CLIs).
	preflight := agent.RunPreflight() //nolint:contextcheck // RunPreflight manages its own timeouts internally
	for _, c := range preflight.Checks {
		status := "ok"
		switch c.Status {
		case agent.CheckPassed:
			// ok
		case agent.CheckFailed:
			status = "error"
			res.Valid = false
		case agent.CheckWarning:
			status = "warning"
		}
		res.Checks = append(res.Checks, check{
			Name:   c.Name,
			Status: status,
			Detail: c.Detail,
			Fix:    c.Fix,
		})
	}

	// Check provider tokens (informational, not required for validity).
	providerChecks := []struct {
		name   string
		envVar string
	}{
		{"GitHub", "GITHUB_TOKEN"},
		{"GitLab", "GITLAB_TOKEN"},
		{"Linear", "LINEAR_TOKEN"},
		{"Wrike", "WRIKE_TOKEN"},
	}

	for _, p := range providerChecks {
		if token := os.Getenv(p.envVar); token != "" {
			res.Checks = append(res.Checks, check{
				Name:   p.name,
				Status: "ok",
				Detail: "token configured",
			})
		} else if envMap, err := settings.LoadEnvMap(""); err == nil {
			if val, ok := envMap[p.envVar]; ok && val != "" {
				res.Checks = append(res.Checks, check{
					Name:   p.name,
					Status: "ok",
					Detail: "token configured",
				})
			} else {
				res.Checks = append(res.Checks, check{
					Name:   p.name,
					Status: "warning",
					Detail: "not configured",
					Fix:    fmt.Sprintf("Set %s or run 'kvelmo provider login %s'", p.envVar, strings.ToLower(p.name)),
				})
			}
		} else {
			res.Checks = append(res.Checks, check{
				Name:   p.name,
				Status: "warning",
				Detail: "not configured",
				Fix:    fmt.Sprintf("Set %s or run 'kvelmo provider login %s'", p.envVar, strings.ToLower(p.name)),
			})
		}
	}

	// Check agent default is valid if settings loaded.
	if effective.Agent.Default != "" {
		allowed := []string{"claude", "codex", "openai", "anthropic", "ollama"}
		valid := false
		for _, a := range allowed {
			if effective.Agent.Default == a {
				valid = true

				break
			}
		}
		if !valid {
			if _, ok := effective.CustomAgents[effective.Agent.Default]; ok {
				valid = true
			}
		}
		if !valid {
			res.Valid = false
			res.Checks = append(res.Checks, check{
				Name:   "agent.default",
				Status: "error",
				Detail: fmt.Sprintf("unknown agent %q", effective.Agent.Default),
				Fix:    "Set agent.default to 'claude', 'codex', 'openai', 'anthropic', or 'ollama'",
			})
		}
	}

	return NewResultResponse(req.ID, res)
}

// resolveProviderToken reads the configured token for a provider from env vars and settings .env files.
func resolveProviderToken(providerName string) string {
	envVars := map[string]string{
		"github": "GITHUB_TOKEN",
		"gitlab": "GITLAB_TOKEN",
		"linear": "LINEAR_TOKEN",
		"wrike":  "WRIKE_TOKEN",
	}

	envVar, ok := envVars[providerName]
	if !ok {
		return ""
	}

	// Check system environment variable
	if val := os.Getenv(envVar); val != "" {
		return val
	}

	// Check global .env file
	if envMap, err := settings.LoadEnvMap(""); err == nil {
		if val, ok := envMap[envVar]; ok && val != "" {
			return val
		}
	}

	return ""
}

// testProviderToken makes a lightweight API call to verify a token is valid.
func testProviderToken(ctx context.Context, providerName, token string) (bool, string) {
	switch providerName {
	case "github":
		return testHTTPToken(ctx, "https://api.github.com/user", token, "token")
	case "gitlab":
		return testHTTPToken(ctx, "https://gitlab.com/api/v4/user", token, "PRIVATE-TOKEN")
	case "linear":
		return testLinearToken(ctx, token)
	case "wrike":
		return testHTTPToken(ctx, "https://www.wrike.com/api/v4/contacts?me=true", token, "bearer")
	default:
		return false, "Unknown provider: " + providerName
	}
}

// testHTTPToken makes a GET request with the token and checks for 200 OK.
func testHTTPToken(ctx context.Context, url, token, authType string) (bool, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err.Error()
	}

	switch authType {
	case "token":
		req.Header.Set("Authorization", "token "+token)
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+token)
	case "PRIVATE-TOKEN":
		req.Header["PRIVATE-TOKEN"] = []string{token}
	}
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, "Connection failed: " + err.Error()
	}
	defer resp.Body.Close() //nolint:errcheck // close error is irrelevant; the response status is already captured

	if resp.StatusCode == http.StatusOK {
		return true, "Authenticated successfully"
	}

	return false, fmt.Sprintf("Authentication failed (HTTP %d)", resp.StatusCode)
}

// testLinearToken uses the GraphQL API to test a Linear token.
func testLinearToken(ctx context.Context, token string) (bool, string) {
	body := strings.NewReader(`{"query":"{ viewer { id } }"}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.linear.app/graphql", body)
	if err != nil {
		return false, err.Error()
	}

	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, "Connection failed: " + err.Error()
	}
	defer resp.Body.Close() //nolint:errcheck // close error is irrelevant; the response status is already captured

	if resp.StatusCode == http.StatusOK {
		return true, "Authenticated successfully"
	}

	return false, fmt.Sprintf("Authentication failed (HTTP %d)", resp.StatusCode)
}

// --- Project Handlers ---

func (g *GlobalSocket) handleListProjects(ctx context.Context, req *Request) (*Response, error) {
	// Snapshot registered worktrees under a short read lock.
	g.mu.RLock()
	infos := make([]*WorktreeInfo, 0, len(g.worktrees))
	for _, w := range g.worktrees {
		cp := *w
		infos = append(infos, &cp)
	}
	g.mu.RUnlock()

	// Query live state from each worktree socket (2-second timeout each).
	const liveTimeout = 2 * time.Second
	type liveResult struct {
		idx   int
		state string
	}
	results := make(chan liveResult, len(infos))

	for i, info := range infos {
		go func(i int, info *WorktreeInfo) {
			if info.SocketPath == "" {
				results <- liveResult{i, "offline"}

				return
			}
			client, err := NewClient(info.SocketPath, WithTimeout(liveTimeout))
			if err != nil {
				results <- liveResult{i, "offline"}

				return
			}
			defer func() { _ = client.Close() }()
			queryCtx, cancel := context.WithTimeout(ctx, liveTimeout)
			defer cancel()
			resp, err := client.Call(queryCtx, "status", nil)
			if err != nil {
				results <- liveResult{i, "offline"}

				return
			}
			var sr StatusResult
			if err := json.Unmarshal(resp.Result, &sr); err != nil {
				results <- liveResult{i, "offline"}

				return
			}
			results <- liveResult{i, string(sr.State)}
		}(i, info)
	}

	for range infos {
		r := <-results
		infos[r.idx].State = r.state
	}

	// Persist updated states back to the registry (skip offline so last-known state is preserved).
	g.mu.Lock()
	for _, info := range infos {
		if w, ok := g.worktrees[info.ID]; ok && info.State != "offline" {
			w.State = info.State
			w.LastSeen = time.Now()
		}
	}
	g.mu.Unlock()

	projects := make([]WorktreeInfo, 0, len(infos))
	for _, info := range infos {
		projects = append(projects, *info)
	}

	return NewResultResponse(req.ID, ProjectListResult{Projects: projects})
}

func (g *GlobalSocket) handleRegisterProject(ctx context.Context, req *Request) (*Response, error) {
	var params RegisterParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	// Default socket path if not provided (frontend sends only path)
	socketPath := params.SocketPath
	if socketPath == "" {
		socketPath = WorktreeSocketPath(params.Path)
	}

	g.mu.Lock()
	id := WorktreeIDFromPath(params.Path)
	g.worktrees[id] = &WorktreeInfo{
		ID:         id,
		Path:       params.Path,
		SocketPath: socketPath,
		State:      "none",
		LastSeen:   time.Now(),
	}
	g.mu.Unlock()

	// Persist to file
	g.saveProjectsToFile()

	return NewResultResponse(req.ID, map[string]string{"id": id})
}

func (g *GlobalSocket) handleUnregisterProject(ctx context.Context, req *Request) (*Response, error) {
	var params UnregisterParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	g.mu.Lock()
	delete(g.worktrees, params.ID)
	g.mu.Unlock()

	// Persist to file
	g.saveProjectsToFile()

	return NewResultResponse(req.ID, map[string]bool{"ok": true})
}

// --- Worktree Handlers ---

func (g *GlobalSocket) handleWorktreeCreate(ctx context.Context, req *Request) (*Response, error) {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}
	if params.Path == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "path is required"), nil
	}

	_, err := g.GetOrCreateWorktreeSocket(params.Path) //nolint:contextcheck // provider clients are long-lived; request context must not cancel them
	if err != nil {
		return NewErrorResponse(req.ID, -32000, err.Error()), nil
	}

	socketPath := WorktreeSocketPath(params.Path)

	return NewResultResponse(req.ID, map[string]string{
		"socket_path": socketPath,
	})
}

// --- Worker Handlers ---

func (g *GlobalSocket) handleListWorkers(ctx context.Context, req *Request) (*Response, error) {
	if g.pool == nil {
		return NewResultResponse(req.ID, WorkersListResult{
			Workers: []WorkerInfo{},
			Stats:   WorkersStats{},
		})
	}

	workers := g.pool.ListWorkers()
	stats := g.pool.Stats()

	result := WorkersListResult{
		Workers: make([]WorkerInfo, len(workers)),
		Stats: WorkersStats{
			TotalWorkers:     stats.TotalWorkers,
			AvailableWorkers: stats.AvailableWorkers,
			WorkingWorkers:   stats.WorkingWorkers,
			QueuedJobs:       stats.QueuedJobs,
			InProgressJobs:   stats.InProgressJobs,
			CompletedJobs:    stats.CompletedJobs,
			FailedJobs:       stats.FailedJobs,
		},
	}

	for i, w := range workers {
		result.Workers[i] = WorkerInfo{
			ID:         w.ID,
			AgentName:  w.AgentName,
			Status:     string(w.Status),
			CurrentJob: w.CurrentJob,
			IsDefault:  w.IsDefault,
		}
	}

	return NewResultResponse(req.ID, result)
}

func (g *GlobalSocket) handleAddWorker(ctx context.Context, req *Request) (*Response, error) {
	if g.pool == nil {
		return NewErrorResponse(req.ID, -32603, "no worker pool configured"), nil
	}

	var params AddWorkerParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	// Default agent to claude if not specified
	if params.Agent == "" {
		params.Agent = "claude"
	}

	w := g.pool.AddWorkerWithAgent(params.Agent)
	if w == nil {
		return NewErrorResponse(req.ID, -32603, "max workers reached"), nil
	}

	return NewResultResponse(req.ID, WorkerInfo{
		ID:        w.ID,
		AgentName: w.AgentName,
		Status:    string(w.Status),
	})
}

func (g *GlobalSocket) handleRemoveWorker(ctx context.Context, req *Request) (*Response, error) {
	if g.pool == nil {
		return NewErrorResponse(req.ID, -32603, "no worker pool configured"), nil
	}

	var params RemoveWorkerParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if err := g.pool.RemoveWorker(params.ID); err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]bool{"ok": true})
}

func (g *GlobalSocket) handleWorkerStats(ctx context.Context, req *Request) (*Response, error) {
	if g.pool == nil {
		return NewResultResponse(req.ID, WorkersStats{})
	}

	stats := g.pool.Stats()

	return NewResultResponse(req.ID, WorkersStats{
		TotalWorkers:     stats.TotalWorkers,
		AvailableWorkers: stats.AvailableWorkers,
		WorkingWorkers:   stats.WorkingWorkers,
		QueuedJobs:       stats.QueuedJobs,
		InProgressJobs:   stats.InProgressJobs,
		CompletedJobs:    stats.CompletedJobs,
		FailedJobs:       stats.FailedJobs,
	})
}

func (g *GlobalSocket) handleMetrics(ctx context.Context, req *Request) (*Response, error) {
	snapshot := metrics.Global().Snapshot()

	return NewResultResponse(req.ID, snapshot)
}

// --- Job Handlers ---

func (g *GlobalSocket) handleListJobs(ctx context.Context, req *Request) (*Response, error) {
	if g.pool == nil {
		return NewResultResponse(req.ID, map[string]any{"jobs": []any{}})
	}

	jobs := g.pool.ListJobs()
	result := make([]map[string]any, len(jobs))
	for i, j := range jobs {
		result[i] = map[string]any{
			"id":         j.ID,
			"type":       j.Type,
			"status":     j.Status,
			"worktree":   j.WorktreeID,
			"created_at": j.CreatedAt.Format(time.RFC3339),
		}
	}

	return NewResultResponse(req.ID, map[string]any{"jobs": result})
}

func (g *GlobalSocket) handleGetJob(ctx context.Context, req *Request) (*Response, error) {
	if g.pool == nil {
		return NewErrorResponse(req.ID, -32603, "no worker pool configured"), nil
	}

	var params struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	job := g.pool.GetJob(params.ID)
	if job == nil {
		return NewErrorResponse(req.ID, -32604, "job not found: "+params.ID), nil
	}

	result := map[string]any{
		"id":         job.ID,
		"type":       job.Type,
		"status":     job.Status,
		"worktree":   job.WorktreeID,
		"prompt":     job.Prompt,
		"created_at": job.CreatedAt.Format(time.RFC3339),
	}

	if job.WorkerID != "" {
		result["worker_id"] = job.WorkerID
	}
	if job.StartedAt != nil {
		result["started_at"] = job.StartedAt.Format(time.RFC3339)
	}
	if job.CompletedAt != nil {
		result["completed_at"] = job.CompletedAt.Format(time.RFC3339)
	}
	if job.Result != "" {
		result["result"] = job.Result
	}
	if job.Error != "" {
		result["error"] = job.Error
	}

	return NewResultResponse(req.ID, result)
}

// --- Settings Handlers (new two-tier system) ---

// SettingsGetParams holds params for settings.get.
type SettingsGetParams struct {
	ProjectPath string `json:"project_path,omitempty"` // Path to project for project-level settings
}

func (g *GlobalSocket) handleSettingsGet(ctx context.Context, req *Request) (*Response, error) {
	var params SettingsGetParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	// Determine project path
	projectPath := params.ProjectPath
	if projectPath == "" {
		// Try to get from first registered worktree
		g.mu.RLock()
		for _, w := range g.worktrees {
			projectPath = w.Path

			break
		}
		g.mu.RUnlock()
	}

	// Load effective settings (merged global + project)
	effective, global, project, err := settings.LoadEffective(projectPath)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	// Generate schema with custom agents added to agent selection options
	schema := settings.GenerateSchemaWithCustomAgents(effective)

	// Mask sensitive fields before sending to client
	effectiveMasked := settings.MaskSettings(effective)
	globalMasked := settings.MaskSettings(global)
	projectMasked := settings.MaskSettings(project)

	return NewResultResponse(req.ID, settings.SettingsResponse{
		Schema:    schema,
		Effective: effectiveMasked,
		Global:    globalMasked,
		Project:   projectMasked,
	})
}

func (g *GlobalSocket) handleSettingsSet(ctx context.Context, req *Request) (*Response, error) {
	var params struct {
		Scope       settings.Scope `json:"scope"`
		Values      map[string]any `json:"values"`
		ProjectPath string         `json:"project_path,omitempty"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	// Validate scope
	if params.Scope != settings.ScopeGlobal && params.Scope != settings.ScopeProject {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "scope must be 'global' or 'project'"), nil
	}

	// Determine project path for project scope
	projectPath := params.ProjectPath
	if params.Scope == settings.ScopeProject && projectPath == "" {
		g.mu.RLock()
		for _, w := range g.worktrees {
			projectPath = w.Path

			break
		}
		g.mu.RUnlock()
		if projectPath == "" {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "project_path required for project scope"), nil
		}
	}

	// Load current settings for the target scope
	var current *settings.Settings
	var err error

	if params.Scope == settings.ScopeGlobal {
		current, err = settings.LoadGlobal()
	} else {
		current, err = settings.LoadProject(projectPath)
	}
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}
	if current == nil {
		current = &settings.Settings{}
	}

	// Apply each value
	for path, value := range params.Values {
		// Skip masked tokens
		if strVal, ok := value.(string); ok && settings.IsMaskedToken(strVal) {
			continue
		}

		// Check if this is a sensitive field
		if settings.IsSensitivePath(path) {
			// Write to .env file
			envVar := settings.GetEnvVarForPath(path)
			if strVal, ok := value.(string); ok && strVal != "" {
				if err := settings.SaveEnvVar(params.Scope, projectPath, envVar, strVal); err != nil {
					return NewErrorResponse(req.ID, -32603, fmt.Sprintf("save env var: %v", err)), nil
				}
			}

			continue
		}

		// Regular field - update settings struct
		if err := settings.SetValue(current, path, value); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
		}
	}

	// Save settings
	if params.Scope == settings.ScopeGlobal {
		err = settings.SaveGlobal(current)
	} else {
		err = settings.SaveProject(projectPath, current)
	}
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	// Return updated effective settings
	effective, global, project, err := settings.LoadEffective(projectPath)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, settings.SettingsResponse{
		Schema:    settings.GenerateSchemaWithCustomAgents(effective),
		Effective: settings.MaskSettings(effective),
		Global:    settings.MaskSettings(global),
		Project:   settings.MaskSettings(project),
	})
}

// --- Browse Handler ---

// BrowseParams holds params for browse.
type BrowseParams struct {
	Path  string `json:"path"`
	Files bool   `json:"files"` // include .md/.txt files
}

// BrowseEntry represents a file or directory entry.
type BrowseEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
}

func (g *GlobalSocket) handleBrowse(ctx context.Context, req *Request) (*Response, error) {
	var params BrowseParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, -32602, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	// Build list of allowed roots: home directory + registered project paths
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return NewErrorResponse(req.ID, -32603, "cannot determine home directory"), nil //nolint:nilerr // JSON-RPC error response
	}

	allowedRoots := []string{homeDir}
	g.mu.RLock()
	for _, w := range g.worktrees {
		allowedRoots = append(allowedRoots, w.Path)
	}
	g.mu.RUnlock()

	path := params.Path
	if path == "" {
		path = homeDir
	}

	// Validate path is within allowed roots
	path, err = ValidatePathWithRoots(allowedRoots, path)
	if err != nil {
		return NewErrorResponse(req.ID, -32602, "access denied: path outside allowed directories"), nil //nolint:nilerr // JSON-RPC error response
	}

	info, err := os.Stat(path)
	if err != nil {
		return NewErrorResponse(req.ID, -32602, "path not found"), nil //nolint:nilerr // JSON-RPC error response
	}
	if !info.IsDir() {
		return NewErrorResponse(req.ID, -32602, "not a directory"), nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, "cannot read directory"), nil //nolint:nilerr // JSON-RPC error response
	}

	result := []BrowseEntry{}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue // skip hidden
		}

		if e.IsDir() {
			result = append(result, BrowseEntry{
				Name:  name,
				Path:  filepath.Join(path, name),
				IsDir: true,
			})
		} else if params.Files {
			// Include .md and .txt files
			ext := strings.ToLower(filepath.Ext(name))
			if ext == ".md" || ext == ".txt" {
				result = append(result, BrowseEntry{
					Name:  name,
					Path:  filepath.Join(path, name),
					IsDir: false,
				})
			}
		}
	}

	return NewResultResponse(req.ID, map[string]any{
		"path":    path,
		"parent":  filepath.Dir(path),
		"entries": result,
	})
}

// --- Chat Handlers ---

// --- File Handlers ---

// FilesListParams holds params for files.list.
type FilesListParams struct {
	Path       string   `json:"path"`
	Extensions []string `json:"extensions,omitempty"` // Filter by extension
	MaxDepth   int      `json:"max_depth,omitempty"`  // Max directory depth
}

// FileEntry represents a file for autocomplete/mentions.
type FileEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	RelPath string `json:"rel_path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size,omitempty"`
}

// Searchable interface for pkg/search hybrid fuzzy matching.
func (f FileEntry) SearchTitle() string        { return f.Name }
func (f FileEntry) SearchDescription() string  { return f.RelPath }
func (f FileEntry) SearchTags() []string       { return nil }
func (f FileEntry) SearchStatus() string       { return "" }
func (f FileEntry) SearchCreatedAt() time.Time { return time.Time{} }
func (f FileEntry) SearchPriority() int        { return 0 }

func (g *GlobalSocket) handleFilesList(ctx context.Context, req *Request) (*Response, error) {
	var params FilesListParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, -32602, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	// Build list of allowed roots from registered projects
	var allowedRoots []string
	g.mu.RLock()
	for _, w := range g.worktrees {
		allowedRoots = append(allowedRoots, w.Path)
	}
	g.mu.RUnlock()

	path := params.Path
	if path == "" {
		// Use first registered project as default
		if len(allowedRoots) > 0 {
			path = allowedRoots[0]
		}
	}

	if path == "" {
		return NewErrorResponse(req.ID, -32602, "no path specified and no projects registered"), nil
	}

	// Validate path is within registered projects
	basePath, err := ValidatePathWithRoots(allowedRoots, path)
	if err != nil {
		return NewErrorResponse(req.ID, -32602, "access denied: path outside registered projects"), nil //nolint:nilerr // JSON-RPC error response
	}

	maxDepth := params.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 3
	}

	entries := []FileEntry{}

	_ = filepath.WalkDir(basePath, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // Continue walking on individual file errors
		}

		// Skip hidden files/dirs
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}

			return nil
		}

		// Calculate depth
		relPath, _ := filepath.Rel(basePath, p)
		depth := strings.Count(relPath, string(filepath.Separator))
		if depth > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}

			return nil
		}

		// Skip common non-source directories
		if d.IsDir() {
			skipDirs := map[string]bool{
				"node_modules": true, "vendor": true, "dist": true,
				"build": true, "__pycache__": true, ".git": true,
			}
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
		}

		// Filter by extensions if specified
		if len(params.Extensions) > 0 && !d.IsDir() {
			ext := strings.ToLower(filepath.Ext(d.Name()))
			found := false
			for _, e := range params.Extensions {
				if ext == e || ext == "."+e {
					found = true

					break
				}
			}
			if !found {
				return nil
			}
		}

		info, _ := d.Info()
		var size int64
		if info != nil && !d.IsDir() {
			size = info.Size()
		}

		entries = append(entries, FileEntry{
			Name:    d.Name(),
			Path:    p,
			RelPath: relPath,
			IsDir:   d.IsDir(),
			Size:    size,
		})

		// Limit results
		if len(entries) >= 500 {
			return filepath.SkipAll
		}

		return nil
	})

	return NewResultResponse(req.ID, map[string]any{
		"path":    basePath,
		"entries": entries,
	})
}

// FilesSearchParams holds params for files.search.
type FilesSearchParams struct {
	Query      string `json:"query"`
	Path       string `json:"path,omitempty"`
	MaxResults int    `json:"max_results,omitempty"`
}

func (g *GlobalSocket) handleFilesSearch(ctx context.Context, req *Request) (*Response, error) {
	var params FilesSearchParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if params.Query == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "query is required"), nil
	}

	// Build list of allowed roots from registered projects
	var allowedRoots []string
	g.mu.RLock()
	for _, w := range g.worktrees {
		allowedRoots = append(allowedRoots, w.Path)
	}
	g.mu.RUnlock()

	path := params.Path
	if path == "" {
		// Use first registered project as default
		if len(allowedRoots) > 0 {
			path = allowedRoots[0]
		}
	}

	if path == "" {
		return NewErrorResponse(req.ID, -32602, "no path specified and no projects registered"), nil
	}

	// Validate path is within registered projects
	basePath, err := ValidatePathWithRoots(allowedRoots, path)
	if err != nil {
		return NewErrorResponse(req.ID, -32602, "access denied: path outside registered projects"), nil //nolint:nilerr // JSON-RPC error response
	}

	maxResults := params.MaxResults
	if maxResults <= 0 {
		maxResults = 20
	}

	// Phase B: Collect all candidate entries up to a cap of 500.
	const collectCap = 500
	candidates := []FileEntry{}

	_ = filepath.WalkDir(basePath, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // Continue walking on individual file errors
		}

		// Skip hidden and common non-source dirs
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}

			return nil
		}
		if d.IsDir() {
			skipDirs := map[string]bool{
				"node_modules": true, "vendor": true, "dist": true,
				"build": true, "__pycache__": true,
			}
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
		}

		relPath, _ := filepath.Rel(basePath, p)
		candidates = append(candidates, FileEntry{
			Name:    d.Name(),
			Path:    p,
			RelPath: relPath,
			IsDir:   d.IsDir(),
		})

		if len(candidates) >= collectCap {
			return filepath.SkipAll
		}

		return nil
	})

	// Rank candidates using hybrid fuzzy + exact search, then unwrap.
	ranked := search.Search(candidates, params.Query, search.Options{})
	entries := make([]FileEntry, 0, min(len(ranked), maxResults))
	for i, r := range ranked {
		if i >= maxResults {
			break
		}
		entries = append(entries, r.Item)
	}

	return NewResultResponse(req.ID, map[string]any{
		"query":   params.Query,
		"path":    basePath,
		"entries": entries,
	})
}

// --- Browser Handlers ---

// BrowserParams holds common browser operation params.
type BrowserParams struct {
	WorktreeID  string `json:"worktree_id,omitempty"`
	SessionName string `json:"session_name,omitempty"`
}

func (g *GlobalSocket) getBrowserOpts(params BrowserParams) *browser.ExecOptions {
	opts := &browser.ExecOptions{
		SessionName: params.SessionName,
	}

	// Get worktree path if specified
	if params.WorktreeID != "" {
		g.mu.RLock()
		if wt, ok := g.worktrees[params.WorktreeID]; ok {
			opts.WorktreePath = wt.Path
		}
		g.mu.RUnlock()
	}

	return opts
}

func (g *GlobalSocket) handleBrowserSnapshot(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // Error conveyed via JSON-RPC response
		}
	}

	result, err := browser.Snapshot(ctx, g.getBrowserOpts(params))
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

// BrowserEvalParams holds params for browser.eval.
type BrowserEvalParams struct {
	BrowserParams

	JS string `json:"js"`
}

func (g *GlobalSocket) handleBrowserEval(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserEvalParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if params.JS == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "js is required"), nil
	}

	result, err := browser.Eval(ctx, g.getBrowserOpts(params.BrowserParams), params.JS)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

func (g *GlobalSocket) handleBrowserConsole(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // Error conveyed via JSON-RPC response
		}
	}

	result, err := browser.Console(ctx, g.getBrowserOpts(params))
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

func (g *GlobalSocket) handleBrowserNetwork(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // Error conveyed via JSON-RPC response
		}
	}

	result, err := browser.Network(ctx, g.getBrowserOpts(params))
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

// BrowserScreenshotParams holds params for browser.screenshot.
type BrowserScreenshotParams struct {
	BrowserParams

	Path     string `json:"path,omitempty"`
	FullPage bool   `json:"full_page,omitempty"`
	Element  string `json:"element,omitempty"`
	Format   string `json:"format,omitempty"`
	Quality  int    `json:"quality,omitempty"`
}

func (g *GlobalSocket) handleBrowserScreenshot(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserScreenshotParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil
		}
	}

	screenshotOpts := &browser.ScreenshotOptions{
		Path:     params.Path,
		FullPage: params.FullPage,
		Element:  params.Element,
		Format:   params.Format,
		Quality:  params.Quality,
	}

	result, err := browser.Screenshot(ctx, g.getBrowserOpts(params.BrowserParams), screenshotOpts)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	if params.WorktreeID == "" {
		return NewResultResponse(req.ID, result)
	}

	// worktree_id provided — store via screenshots.capture
	g.mu.RLock()
	wt, ok := g.worktrees[params.WorktreeID]
	g.mu.RUnlock()
	if !ok {
		return NewResultResponse(req.ID, result)
	}

	imageData, err := os.ReadFile(result.Path)
	if err != nil {
		return NewResultResponse(req.ID, result)
	}

	format := params.Format
	if format == "" {
		format = "png"
	}

	captureParams, err := json.Marshal(map[string]any{
		"source": "user",
		"format": format,
		"data":   base64.StdEncoding.EncodeToString(imageData),
	})
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "failed to marshal params"), nil
	}

	client, err := NewClient(wt.SocketPath, WithTimeout(5*time.Second))
	if err != nil {
		return NewResultResponse(req.ID, result)
	}
	defer func() { _ = client.Close() }()

	captureResp, err := client.Call(ctx, "screenshots.capture", captureParams)
	if err != nil || captureResp == nil || captureResp.Result == nil {
		return NewResultResponse(req.ID, result)
	}

	var ss map[string]any
	if err := json.Unmarshal(captureResp.Result, &ss); err != nil {
		return NewResultResponse(req.ID, result)
	}

	return NewResultResponse(req.ID, map[string]any{
		"id":   ss["id"],
		"path": result.Path,
	})
}

// BrowserNavigateParams holds params for browser.navigate.
type BrowserNavigateParams struct {
	BrowserParams

	URL string `json:"url"`
}

func (g *GlobalSocket) handleBrowserNavigate(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserNavigateParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if params.URL == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "url is required"), nil
	}

	result, err := browser.Navigate(ctx, g.getBrowserOpts(params.BrowserParams), params.URL)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

// BrowserClickParams holds params for browser.click.
type BrowserClickParams struct {
	BrowserParams

	Selector string `json:"selector"`
}

func (g *GlobalSocket) handleBrowserClick(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserClickParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if params.Selector == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "selector is required"), nil
	}

	result, err := browser.Click(ctx, g.getBrowserOpts(params.BrowserParams), params.Selector)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

// BrowserTypeParams holds params for browser.type.
type BrowserTypeParams struct {
	BrowserParams

	Selector string `json:"selector"`
	Text     string `json:"text"`
}

func (g *GlobalSocket) handleBrowserType(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserTypeParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if params.Selector == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "selector is required"), nil
	}

	result, err := browser.Type(ctx, g.getBrowserOpts(params.BrowserParams), params.Selector, params.Text)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

// BrowserWaitParams holds params for browser.wait.
type BrowserWaitParams struct {
	BrowserParams

	Selector  string `json:"selector"`
	TimeoutMs int    `json:"timeout_ms,omitempty"`
}

func (g *GlobalSocket) handleBrowserWait(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserWaitParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if params.Selector == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "selector is required"), nil
	}

	result, err := browser.Wait(ctx, g.getBrowserOpts(params.BrowserParams), params.Selector, params.TimeoutMs)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

func (g *GlobalSocket) handleBrowserInstall(ctx context.Context, req *Request) (*Response, error) {
	if err := browser.Install(ctx); err != nil {
		return NewErrorResponse(req.ID, -32603, fmt.Sprintf("install runtime: %v", err)), nil
	}
	if err := browser.InstallBrowsers(ctx); err != nil {
		return NewErrorResponse(req.ID, -32603, fmt.Sprintf("install browsers: %v", err)), nil
	}

	return NewResultResponse(req.ID, map[string]any{
		"ok":      true,
		"message": "Browser runtime installed successfully",
		"path":    browser.Paths(),
	})
}

func (g *GlobalSocket) handleBrowserStatus(ctx context.Context, req *Request) (*Response, error) {
	installed := browser.IsInstalled()
	result := map[string]any{
		"installed":   installed,
		"runtime_dir": browser.Paths(),
		"binary_path": browser.BinaryPath(),
	}

	if installed {
		version, err := browser.Version()
		if err != nil {
			result["version_error"] = err.Error()
		} else {
			result["version"] = version
		}
	}

	cfg, err := browser.LoadConfig()
	if err != nil {
		result["config_error"] = err.Error()
	} else {
		result["config"] = cfg
	}

	return NewResultResponse(req.ID, result)
}

func (g *GlobalSocket) handleBrowserConfigGet(ctx context.Context, req *Request) (*Response, error) {
	cfg, err := browser.LoadConfig()
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, cfg)
}

// BrowserConfigSetParams holds params for browser.config.set.
type BrowserConfigSetParams struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (g *GlobalSocket) handleBrowserConfigSet(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserConfigSetParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	cfg, err := browser.LoadConfig()
	if err != nil {
		return NewErrorResponse(req.ID, -32603, fmt.Sprintf("load config: %v", err)), nil
	}

	switch strings.ToLower(params.Key) {
	case "headless":
		cfg.Headless = params.Value == "true" || params.Value == "1" || params.Value == "yes"
	case "browser":
		if params.Value != "chromium" && params.Value != "firefox" && params.Value != "webkit" {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "browser must be chromium, firefox, or webkit"), nil
		}
		cfg.Browser = params.Value
	case "profile":
		cfg.Profile = params.Value
	case "timeout":
		var timeout int
		if _, scanErr := fmt.Sscanf(params.Value, "%d", &timeout); scanErr != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "timeout must be a number"), nil //nolint:nilerr // JSON-RPC error response
		}
		cfg.Timeout = timeout
	default:
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "unknown config key: "+params.Key), nil
	}

	if err := cfg.Save(); err != nil {
		return NewErrorResponse(req.ID, -32603, fmt.Sprintf("save config: %v", err)), nil
	}

	return NewResultResponse(req.ID, cfg)
}

// BrowserFillParams holds params for browser.fill.
type BrowserFillParams struct {
	BrowserParams

	Selector string `json:"selector"`
	Value    string `json:"value"`
}

func (g *GlobalSocket) handleBrowserFill(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserFillParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if params.Selector == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "selector is required"), nil
	}

	result, err := browser.Fill(ctx, g.getBrowserOpts(params.BrowserParams), params.Selector, params.Value)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

// BrowserSelectParams holds params for browser.select.
type BrowserSelectParams struct {
	BrowserParams

	Selector string   `json:"selector"`
	Values   []string `json:"values"`
}

func (g *GlobalSocket) handleBrowserSelect(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserSelectParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if params.Selector == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "selector is required"), nil
	}

	if len(params.Values) == 0 {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "values is required"), nil
	}

	result, err := browser.Select(ctx, g.getBrowserOpts(params.BrowserParams), params.Selector, params.Values...)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

// BrowserHoverParams holds params for browser.hover.
type BrowserHoverParams struct {
	BrowserParams

	Selector string `json:"selector"`
}

func (g *GlobalSocket) handleBrowserHover(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserHoverParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if params.Selector == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "selector is required"), nil
	}

	result, err := browser.Hover(ctx, g.getBrowserOpts(params.BrowserParams), params.Selector)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

// BrowserFocusParams holds params for browser.focus.
type BrowserFocusParams struct {
	BrowserParams

	Selector string `json:"selector"`
}

func (g *GlobalSocket) handleBrowserFocus(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserFocusParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if params.Selector == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "selector is required"), nil
	}

	result, err := browser.Focus(ctx, g.getBrowserOpts(params.BrowserParams), params.Selector)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

// BrowserScrollParams holds params for browser.scroll.
type BrowserScrollParams struct {
	BrowserParams

	Direction string `json:"direction"` // up, down, left, right
	Amount    int    `json:"amount,omitempty"`
	Selector  string `json:"selector,omitempty"`
}

func (g *GlobalSocket) handleBrowserScroll(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserScrollParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if params.Direction == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "direction is required"), nil
	}

	result, err := browser.Scroll(ctx, g.getBrowserOpts(params.BrowserParams), params.Direction, params.Amount, params.Selector)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

// BrowserPressParams holds params for browser.press.
type BrowserPressParams struct {
	BrowserParams

	Key      string `json:"key"`
	Selector string `json:"selector,omitempty"`
}

func (g *GlobalSocket) handleBrowserPress(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserPressParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if params.Key == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "key is required"), nil
	}

	result, err := browser.Press(ctx, g.getBrowserOpts(params.BrowserParams), params.Key, params.Selector)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

func (g *GlobalSocket) handleBrowserBack(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // Error conveyed via JSON-RPC response
		}
	}

	result, err := browser.Back(ctx, g.getBrowserOpts(params))
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

func (g *GlobalSocket) handleBrowserForward(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // Error conveyed via JSON-RPC response
		}
	}

	result, err := browser.Forward(ctx, g.getBrowserOpts(params))
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

func (g *GlobalSocket) handleBrowserReload(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // Error conveyed via JSON-RPC response
		}
	}

	result, err := browser.Reload(ctx, g.getBrowserOpts(params))
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

// BrowserDialogParams holds params for browser.dialog.
type BrowserDialogParams struct {
	BrowserParams

	Action string `json:"action"` // accept or dismiss
	Text   string `json:"text,omitempty"`
}

func (g *GlobalSocket) handleBrowserDialog(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserDialogParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if params.Action != "accept" && params.Action != "dismiss" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "action must be 'accept' or 'dismiss'"), nil
	}

	result, err := browser.Dialog(ctx, g.getBrowserOpts(params.BrowserParams), params.Action, params.Text)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

// BrowserUploadParams holds params for browser.upload.
type BrowserUploadParams struct {
	BrowserParams

	Selector string   `json:"selector"`
	Files    []string `json:"files"`
}

func (g *GlobalSocket) handleBrowserUpload(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserUploadParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if params.Selector == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "selector is required"), nil
	}

	if len(params.Files) == 0 {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "files is required"), nil
	}

	result, err := browser.Upload(ctx, g.getBrowserOpts(params.BrowserParams), params.Selector, params.Files)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
}

// BrowserPDFParams holds params for browser.pdf.
type BrowserPDFParams struct {
	BrowserParams

	Path      string `json:"path,omitempty"`
	Format    string `json:"format,omitempty"`
	Landscape bool   `json:"landscape,omitempty"`
}

func (g *GlobalSocket) handleBrowserPDF(ctx context.Context, req *Request) (*Response, error) {
	var params BrowserPDFParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // Error conveyed via JSON-RPC response
		}
	}

	pdfOpts := &browser.PDFOptions{
		Path:      params.Path,
		Format:    params.Format,
		Landscape: params.Landscape,
	}

	result, err := browser.GeneratePDF(ctx, g.getBrowserOpts(params.BrowserParams), pdfOpts)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, err.Error()), nil
	}

	return NewResultResponse(req.ID, result)
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

// GetOrCreateWorktreeSocket returns an existing worktree socket or creates one on-demand.
// This allows projects to use the global worker pool for planning/implementation.
func (g *GlobalSocket) GetOrCreateWorktreeSocket(projectPath string) (interface{}, error) {
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

func (g *GlobalSocket) Server() *Server {
	return g.server
}

func (g *GlobalSocket) Pool() *worker.Pool {
	return g.pool
}

func (g *GlobalSocket) SetPool(pool *worker.Pool) {
	g.pool = pool
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
		result = append(result, w)
	}

	return result
}

// --- Tasks List Handler ---

// TaskListSummary represents a task for the tasks.list response.
type TaskListSummary struct {
	ID               string `json:"id"`
	Path             string `json:"path"`
	State            string `json:"state"`
	TaskID           string `json:"task_id,omitempty"`
	TaskTitle        string `json:"task_title,omitempty"`
	Source           string `json:"source,omitempty"`
	QueueCount       int    `json:"queue_count,omitempty"`
	LastError        string `json:"last_error,omitempty"`
	LastFailureClass string `json:"last_failure_class,omitempty"`
	PendingPromptID  string `json:"pending_prompt_id,omitempty"`
}

// Searchable interface implementation for TaskListSummary.
func (t TaskListSummary) SearchTitle() string        { return t.TaskTitle }
func (t TaskListSummary) SearchDescription() string  { return t.Source }
func (t TaskListSummary) SearchTags() []string       { return nil }
func (t TaskListSummary) SearchStatus() string       { return t.State }
func (t TaskListSummary) SearchCreatedAt() time.Time { return time.Time{} }
func (t TaskListSummary) SearchPriority() int        { return 0 }

// TasksListResult is the response for tasks.list.
type TasksListResult struct {
	Tasks      []TaskListSummary `json:"tasks"`
	Total      int               `json:"total"`
	Page       int               `json:"page,omitempty"`
	PerPage    int               `json:"per_page,omitempty"`
	TotalPages int               `json:"total_pages,omitempty"`
	HasNext    bool              `json:"has_next,omitempty"`
}

func (g *GlobalSocket) handleTasksList(ctx context.Context, req *Request) (*Response, error) {
	var params struct {
		State   string `json:"state,omitempty"`
		Query   string `json:"query,omitempty"`
		Page    int    `json:"page,omitempty"`
		PerPage int    `json:"per_page,omitempty"`
	}
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	g.mu.RLock()
	worktrees := make([]*WorktreeInfo, 0, len(g.worktrees))
	for _, w := range g.worktrees {
		worktrees = append(worktrees, w)
	}
	g.mu.RUnlock()

	tasks := make([]TaskListSummary, 0, len(worktrees))
	for _, wt := range worktrees {
		summary := TaskListSummary{
			ID:    wt.ID,
			Path:  wt.Path,
			State: wt.State,
		}

		// Try to get more task details by calling the worktree socket
		if wt.SocketPath != "" && SocketExists(wt.SocketPath) {
			client, err := NewClient(wt.SocketPath, WithTimeout(1*time.Second))
			if err == nil {
				resp, callErr := client.Call(ctx, "status", nil)
				if callErr == nil && resp != nil && resp.Result != nil {
					var status StatusResult
					if jsonErr := json.Unmarshal(resp.Result, &status); jsonErr == nil {
						if status.Task != nil {
							summary.TaskID = status.Task.ID
							summary.TaskTitle = status.Task.Title
							summary.Source = status.Task.Source
						}
						summary.State = string(status.State)
						summary.LastError = status.LastError
						summary.LastFailureClass = status.LastFailureClass
						summary.PendingPromptID = status.PendingPromptID
					}
				}
				// Fetch queue count
				qResp, qErr := client.Call(ctx, "queue.list", nil)
				if qErr == nil && qResp != nil && qResp.Result != nil {
					var qResult struct {
						Count int `json:"count"`
					}
					if jsonErr := json.Unmarshal(qResp.Result, &qResult); jsonErr == nil {
						summary.QueueCount = qResult.Count
					}
				}
				_ = client.Close()
			}
		}

		tasks = append(tasks, summary)
	}

	// Apply filters if provided
	if params.State != "" {
		f := filter.New[TaskListSummary]().
			Eq(func(t TaskListSummary) any { return t.State }, params.State)
		tasks = f.Apply(tasks)
	}

	// Apply search if query provided
	if params.Query != "" {
		ranked := search.Search(tasks, params.Query, search.Options{BoostActive: true})
		tasks = make([]TaskListSummary, len(ranked))
		for i, r := range ranked {
			tasks[i] = r.Item
		}
	}

	total := len(tasks)

	// Apply pagination if requested
	if params.Page > 0 || params.PerPage > 0 {
		pg := paginationWithDefault(params.Page, params.PerPage)
		p := page.NewPage(tasks, pg)

		return NewResultResponse(req.ID, TasksListResult{
			Tasks:      p.Items,
			Total:      int(p.Total),
			Page:       p.PageNum,
			PerPage:    p.PerPage,
			TotalPages: p.TotalPages,
			HasNext:    p.HasNext,
		})
	}

	return NewResultResponse(req.ID, TasksListResult{Tasks: tasks, Total: total})
}
