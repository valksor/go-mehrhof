package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	taskDirName     = ".mehrhof"
	workSubDirName  = "work"
	plannedDirName  = "planned"
	activeTaskFile  = ".active_task"
	workFileName    = "work.yaml"
	notesFileName   = "notes.md"
	specsDirName    = "specifications"
	sessionsDirName = "sessions"
	configFileName  = "config.yaml"
	envFileName     = ".env"

	// Usage buffer configuration.
	defaultUsageFlushInterval  = 5 * time.Second // Auto-flush interval
	defaultUsageFlushThreshold = 10              // Flush after N updates
)

// usageBuffer tracks accumulated usage for a task/step combination.
type usageBuffer struct {
	inputTokens  int
	outputTokens int
	cachedTokens int
	costUSD      float64
	callCount    int
}

// Workspace manages task storage within a repository.
type Workspace struct {
	root     string // Repository root
	taskRoot string // .mehrhof directory
	workRoot string // .mehrhof/work directory

	// Usage buffering for reducing I/O
	usageMu   sync.RWMutex
	usageBuf  map[string]map[string]*usageBuffer // taskID -> step -> buffer
	lastFlush time.Time
}

// OpenWorkspace opens or creates a workspace in the given directory.
// If cfg is nil, defaults are used for work directory path.
func OpenWorkspace(repoRoot string, cfg *WorkspaceConfig) (*Workspace, error) {
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	taskRoot := filepath.Join(absRoot, taskDirName)

	// Determine work directory path from config or default
	workDir := ".mehrhof/work" // default
	if cfg != nil && cfg.Storage.WorkDir != "" {
		workDir = cfg.Storage.WorkDir
	}
	// Work directory is relative to project root (absRoot), not taskRoot
	workRoot := filepath.Join(absRoot, workDir)

	return &Workspace{
		root:      absRoot,
		taskRoot:  taskRoot,
		workRoot:  workRoot,
		usageBuf:  make(map[string]map[string]*usageBuffer),
		lastFlush: time.Now(),
	}, nil
}

// Root returns the repository root path.
func (w *Workspace) Root() string {
	return w.root
}

// TaskRoot returns the .mehrhof directory path.
func (w *Workspace) TaskRoot() string {
	return w.taskRoot
}

// WorkRoot returns the .mehrhof/work directory path.
func (w *Workspace) WorkRoot() string {
	return w.workRoot
}

// ConfigPath returns the path to the config file.
func (w *Workspace) ConfigPath() string {
	return filepath.Join(w.taskRoot, configFileName)
}

// HasConfig returns true if the config file exists.
func (w *Workspace) HasConfig() bool {
	_, err := os.Stat(w.ConfigPath())

	return err == nil
}

// EnvPath returns the path to the .env file.
func (w *Workspace) EnvPath() string {
	return filepath.Join(w.taskRoot, envFileName)
}

// LoadEnv reads the .env file and returns key-value pairs.
// Returns empty map if file doesn't exist.
func (w *Workspace) LoadEnv() (map[string]string, error) {
	data, err := os.ReadFile(w.EnvPath())
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}

		return nil, fmt.Errorf("read .env: %w", err)
	}

	result := make(map[string]string)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Parse KEY=VALUE
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	return result, nil
}

// EnsureInitialized ensures that the workspace directory exists.
// Note: Does NOT update .gitignore - use UpdateGitignore() for that (typically only in init command).
func (w *Workspace) EnsureInitialized() error {
	// Create .mehrhof/work directory (also creates .mehrhof/)
	if err := os.MkdirAll(w.workRoot, 0o755); err != nil {
		return fmt.Errorf("create workspace directories: %w", err)
	}

	return nil
}

// UpdateGitignore adds standard mehrhof entries to .gitignore.
// This should only be called from the init command.
func (w *Workspace) UpdateGitignore() error {
	gitignorePath := filepath.Join(w.root, ".gitignore")

	// Read existing .gitignore
	var content string
	if _, err := os.Stat(gitignorePath); err == nil {
		data, err := os.ReadFile(gitignorePath)
		if err != nil {
			return fmt.Errorf("read .gitignore: %w", err)
		}
		content = string(data)
	}

	// Load config to get work directory setting
	workDirEntry := ".mehrhof/work/" // default
	cfg, err := w.LoadConfig()
	if err == nil && cfg.Storage.WorkDir != "" {
		workDirEntry = cfg.Storage.WorkDir
		// Ensure trailing slash for directory
		if !strings.HasSuffix(workDirEntry, "/") {
			workDirEntry += "/"
		}
	}

	// Define entries to add
	entries := []string{
		workDirEntry,
		taskDirName + "/" + envFileName,
		activeTaskFile,
	}

	modified := false
	lines := strings.Split(content, "\n")

	for _, entry := range entries {
		found := false
		for _, line := range lines {
			if strings.TrimSpace(line) == entry {
				found = true

				break
			}
		}

		if !found {
			if len(content) > 0 && !strings.HasSuffix(content, "\n") {
				content += "\n"
				lines = append(lines, "")
			}
			content += entry + "\n"
			lines = append(lines, entry)
			modified = true
		}
	}

	if modified {
		if err := os.WriteFile(gitignorePath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write .gitignore: %w", err)
		}
	}

	return nil
}
