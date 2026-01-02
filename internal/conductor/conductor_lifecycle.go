package conductor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/valksor/go-mehrhof/internal/provider"
	"github.com/valksor/go-mehrhof/internal/storage"
	"github.com/valksor/go-mehrhof/internal/vcs"
)

// Initialize sets up the conductor for a repository.
func (c *Conductor) Initialize(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Initialize git (optional - might not be in a git repo)
	git, err := vcs.New(ctx, c.opts.WorkDir)
	if err == nil {
		c.git = git
	}

	// Determine workspace root
	// If we're in a worktree, use the main repo for storage
	root := c.opts.WorkDir
	if c.git != nil {
		if c.git.IsWorktree() {
			// Get main repo path for shared storage
			mainRepo, err := c.git.GetMainWorktreePath(ctx)
			if err != nil {
				return fmt.Errorf("get main repo from worktree: %w", err)
			}
			root = mainRepo
		} else {
			root = c.git.Root()
		}
	}

	// Load workspace config to get work directory setting
	var cfg *storage.WorkspaceConfig
	configPath := filepath.Join(root, ".mehrhof", "config.yaml")
	if data, err := os.ReadFile(configPath); err == nil {
		cfg = storage.NewDefaultWorkspaceConfig()
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return fmt.Errorf("parse config file: %w", err)
		}
	}

	// Initialize workspace with config
	ws, err := storage.OpenWorkspace(root, cfg)
	if err != nil {
		return fmt.Errorf("initialize workspace: %w", err)
	}
	c.workspace = ws

	// Auto-initialize if requested
	if c.opts.AutoInit {
		if err := ws.EnsureInitialized(); err != nil {
			return fmt.Errorf("auto-initialize workspace: %w", err)
		}
	}

	// Handle task detection differently based on context
	if c.git != nil && c.git.IsWorktree() {
		// Auto-detect task from worktree path
		active, err := ws.FindTaskByWorktreePath(c.git.Root())
		if err != nil {
			return fmt.Errorf("find task by worktree: %w", err)
		}
		if active != nil {
			c.activeTask = active
			// Load associated work
			work, err := ws.LoadWork(active.ID)
			if err == nil {
				c.taskWork = work
				// Restore state machine state
				c.machine.SetWorkUnit(c.buildWorkUnit())
			}
		}
	} else {
		// Standard behavior: check for existing active task from .active_task file
		if ws.HasActiveTask() {
			active, err := ws.LoadActiveTask()
			if err == nil {
				c.activeTask = active
				// Load associated work
				work, err := ws.LoadWork(active.ID)
				if err == nil {
					c.taskWork = work
					// Restore state machine state
					c.machine.SetWorkUnit(c.buildWorkUnit())
				}
			}
		}
	}

	// Register user-defined agent aliases from workspace config
	if c.workspace != nil {
		if cfg, err := c.workspace.LoadConfig(); err == nil {
			if err := c.registerAliasAgents(cfg); err != nil {
				return fmt.Errorf("register alias agents: %w", err)
			}

			// Load plugins
			if err := c.loadPlugins(ctx, cfg); err != nil {
				// Plugins are optional, but log the error for debugging
				// Don't fail initialization since plugins are optional
				c.logError(fmt.Errorf("load plugins (non-fatal): %w", err))
			}
		}
	}

	// Select agent with priority: CLI flag > stored task agent > auto-detect
	if c.opts.AgentName != "" {
		// Priority 1: CLI flag always wins
		agentInst, err := c.agents.Get(c.opts.AgentName)
		if err != nil {
			return fmt.Errorf("get agent %s: %w", c.opts.AgentName, err)
		}
		c.activeAgent = agentInst
	} else if c.taskWork != nil && c.taskWork.Agent.Name != "" {
		// Priority 2: Restore agent from stored task config (when resuming)
		agentInst, err := c.agents.Get(c.taskWork.Agent.Name)
		if err != nil {
			// Stored agent not available, fall back to auto-detect
			agentInst, err = c.agents.Detect()
			if err != nil {
				return fmt.Errorf("detect agent: %w", err)
			}
		} else {
			// Re-apply inline env vars if stored
			agentInst = applyAgentEnv(agentInst, c.taskWork.Agent.InlineEnv)
			// Re-apply args if stored
			if len(c.taskWork.Agent.Args) > 0 {
				agentInst = agentInst.WithArgs(c.taskWork.Agent.Args...)
			}
		}
		c.activeAgent = agentInst
	} else {
		// Priority 3: Auto-detect
		agentInst, err := c.agents.Detect()
		if err != nil {
			return fmt.Errorf("detect agent: %w", err)
		}
		c.activeAgent = agentInst
	}

	// Apply workspace env vars to agent (filtered by agent name prefix)
	if c.workspace != nil {
		if cfg, err := c.workspace.LoadConfig(); err == nil {
			agentEnv := cfg.GetEnvForAgent(c.activeAgent.Name())
			for k, v := range agentEnv {
				c.activeAgent = c.activeAgent.WithEnv(k, v)
			}
		}
	}

	return nil
}

// Start registers a new task from a reference (does not run planning).
func (c *Conductor) Start(ctx context.Context, reference string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Reject starting new tasks from within a worktree
	if c.git != nil && c.git.IsWorktree() {
		mainRepo, _ := c.git.GetMainWorktreePath(ctx)
		return fmt.Errorf("this command must be run from the main repository; you are currently in a worktree, return to the main repository first: cd %s", mainRepo)
	}

	// Check for existing active task (only applies in main repo)
	if c.activeTask != nil {
		return fmt.Errorf("task already active: %s (use 'task status' to check)", c.activeTask.ID)
	}

	// If git is available and branch creation requested, check for clean workspace FIRST
	if err := c.ensureCleanWorkspace(ctx); err != nil {
		return err
	}

	// Detect provider and fetch work unit
	p, workUnit, err := c.fetchWorkUnit(ctx, reference)
	if err != nil {
		return err
	}

	// Capture task agent config from workUnit (if specified in task frontmatter)
	c.taskAgentConfig = workUnit.AgentConfig

	// Generate task ID first (needed for branch name)
	taskID := storage.GenerateTaskID()

	// Resolve naming (external key, branch pattern, commit prefix)
	namingInfo := c.resolveNaming(workUnit, taskID)

	// Create and switch to branch (or worktree) BEFORE creating work directory
	gitInfo, err := c.createBranchOrWorktree(ctx, taskID, namingInfo)
	if err != nil {
		return err
	}

	// Snapshot the source (read-only copy)
	snapshot := c.snapshotSource(ctx, p, reference, workUnit)

	// Register the task with workspace (writes source files)
	if err := c.registerTask(taskID, reference, workUnit, snapshot, gitInfo, namingInfo); err != nil {
		return err
	}

	c.publishProgress("Task registered", 100)
	return nil
}

// ensureCleanWorkspace checks if workspace is clean when branch creation is requested.
func (c *Conductor) ensureCleanWorkspace(ctx context.Context) error {
	if c.git == nil || !c.opts.CreateBranch {
		return nil
	}

	hasChanges, err := c.git.HasChanges(ctx)
	if err != nil {
		return fmt.Errorf("check git status: %w", err)
	}
	if hasChanges {
		return fmt.Errorf("workspace has uncommitted changes\nPlease commit or stash your changes, or use --no-branch to work on the current branch")
	}
	return nil
}

// fetchWorkUnit resolves the provider and fetches the work unit.
func (c *Conductor) fetchWorkUnit(ctx context.Context, reference string) (any, *provider.WorkUnit, error) {
	resolveOpts := provider.ResolveOptions{
		DefaultProvider: c.opts.DefaultProvider,
	}
	p, id, err := c.providers.Resolve(ctx, reference, provider.Config{}, resolveOpts)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve provider: %w", err)
	}

	reader, ok := p.(provider.Reader)
	if !ok {
		return nil, nil, fmt.Errorf("provider does not support reading")
	}

	workUnit, err := reader.Fetch(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch work unit: %w", err)
	}

	return p, workUnit, nil
}

// snapshotSource creates a snapshot of the source content.
func (c *Conductor) snapshotSource(ctx context.Context, p any, reference string, workUnit *provider.WorkUnit) *provider.Snapshot {
	if snapshotter, ok := p.(provider.Snapshotter); ok {
		snapshot, err := snapshotter.Snapshot(ctx, workUnit.ID)
		if err == nil && snapshot != nil {
			return snapshot
		}
	}

	// Fallback: return minimal snapshot with reference only
	return &provider.Snapshot{
		Type: workUnit.Provider,
		Ref:  reference,
	}
}

// buildSourceInfo creates storage.SourceInfo from provider snapshot (metadata only).
func (c *Conductor) buildSourceInfo(snapshot *provider.Snapshot) storage.SourceInfo {
	info := storage.SourceInfo{
		Type:   snapshot.Type,
		Ref:    snapshot.Ref,
		ReadAt: time.Now(),
	}

	// For single file content, store path reference
	if snapshot.Content != "" {
		// Generate filename from reference or use default
		filename := "source.md"
		if snapshot.Ref != "" {
			// Extract filename from reference if possible
			if idx := strings.LastIndex(snapshot.Ref, "/"); idx != -1 {
				filename = snapshot.Ref[idx+1:]
			} else if idx := strings.LastIndex(snapshot.Ref, ":"); idx != -1 {
				filename = snapshot.Ref[idx+1:] + ".md"
			}
		}
		info.Files = []string{"source/" + filename}
	}

	// For directory/multiple files, store file paths
	for _, f := range snapshot.Files {
		info.Files = append(info.Files, "source/"+f.Path)
	}

	return info
}

// writeSourceFiles writes snapshot content to the work directory's source/ subdirectory.
func (c *Conductor) writeSourceFiles(taskID string, snapshot *provider.Snapshot) error {
	if snapshot == nil {
		return nil
	}

	workPath := c.workspace.WorkPath(taskID)
	sourceDir := filepath.Join(workPath, "source")

	// Create source directory if it doesn't exist
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		return fmt.Errorf("create source directory: %w", err)
	}

	// Write single file content
	if snapshot.Content != "" {
		filename := "source.md"
		if snapshot.Ref != "" {
			// Extract filename from reference
			if idx := strings.LastIndex(snapshot.Ref, "/"); idx != -1 {
				filename = snapshot.Ref[idx+1:]
			} else if idx := strings.LastIndex(snapshot.Ref, ":"); idx != -1 {
				filename = snapshot.Ref[idx+1:] + ".md"
			}
		}
		destPath := filepath.Join(sourceDir, filename)
		if err := os.WriteFile(destPath, []byte(snapshot.Content), 0o644); err != nil {
			return fmt.Errorf("write source file: %w", err)
		}
	}

	// Write multiple files (directory provider)
	for _, f := range snapshot.Files {
		destPath := filepath.Join(sourceDir, f.Path)
		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return fmt.Errorf("create directory: %w", err)
		}
		if err := os.WriteFile(destPath, []byte(f.Content), 0o644); err != nil {
			return fmt.Errorf("write file %s: %w", f.Path, err)
		}
	}

	return nil
}

// registerTask creates the work directory and active task reference.
func (c *Conductor) registerTask(taskID, reference string, workUnit *provider.WorkUnit, snapshot *provider.Snapshot, gi *gitInfo, ni *namingInfo) error {
	// Resolve agent for this task (uses priority: CLI > task > workspace > auto)
	agentInst, agentSource, err := c.resolveAgentForTask()
	if err != nil {
		return fmt.Errorf("resolve agent: %w", err)
	}
	c.activeAgent = agentInst

	// Build SourceInfo with metadata (files will be written separately)
	sourceInfo := c.buildSourceInfo(snapshot)

	// Create work directory (creates source/ subdirectory)
	work, err := c.workspace.CreateWork(taskID, sourceInfo)
	if err != nil {
		return fmt.Errorf("create work: %w", err)
	}

	// Write source files to work directory
	if err := c.writeSourceFiles(taskID, snapshot); err != nil {
		return fmt.Errorf("write source files: %w", err)
	}

	// Set metadata from work unit and naming info
	work.Metadata.Title = workUnit.Title
	work.Metadata.ExternalKey = ni.externalKey
	work.Metadata.TaskType = ni.taskType
	work.Metadata.Slug = ni.slug

	// Store git info if branch was created
	if gi.branchName != "" {
		work.Git.Branch = gi.branchName
		work.Git.BaseBranch = gi.baseBranch
		work.Git.WorktreePath = gi.worktreePath
		work.Git.CreatedAt = time.Now()
		work.Git.CommitPrefix = gi.commitPrefix
		work.Git.BranchPattern = gi.branchPattern
	}

	// Store agent info for persistence (so subsequent commands use the same agent)
	work.Agent = storage.AgentInfo{
		Name:   agentInst.Name(),
		Source: agentSource,
	}
	// Store inline env vars if specified in task (for reference, not resolved values)
	if c.taskAgentConfig != nil && len(c.taskAgentConfig.Env) > 0 {
		work.Agent.InlineEnv = c.taskAgentConfig.Env
	}

	if err := c.workspace.SaveWork(work); err != nil {
		return fmt.Errorf("save work: %w", err)
	}

	// Create active task reference
	active := storage.NewActiveTask(taskID, reference, c.workspace.WorkPath(taskID))

	// Set git info on active task
	if c.git != nil {
		active.UseGit = true
		if gi.branchName != "" {
			active.Branch = gi.branchName
		}
		if gi.worktreePath != "" {
			active.WorktreePath = gi.worktreePath
		}
	}

	// Save active task
	if err := c.workspace.SaveActiveTask(active); err != nil {
		return fmt.Errorf("save active task: %w", err)
	}

	c.activeTask = active
	c.taskWork = work

	// Set up state machine
	c.machine.SetWorkUnit(c.buildWorkUnit())

	return nil
}

// Resume loads an existing active task.
func (c *Conductor) Resume(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.workspace.HasActiveTask() {
		return fmt.Errorf("no active task")
	}

	active, err := c.workspace.LoadActiveTask()
	if err != nil {
		return fmt.Errorf("load active task: %w", err)
	}

	work, err := c.workspace.LoadWork(active.ID)
	if err != nil {
		return fmt.Errorf("load work: %w", err)
	}

	c.activeTask = active
	c.taskWork = work
	c.machine.SetWorkUnit(c.buildWorkUnit())

	return nil
}

// Delete abandons the current task without merging.
func (c *Conductor) Delete(ctx context.Context, opts DeleteOptions) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.activeTask == nil {
		return fmt.Errorf("no active task")
	}

	taskID := c.activeTask.ID

	// Handle git operations if applicable
	if c.git != nil && c.activeTask.UseGit && c.activeTask.Branch != "" && !opts.KeepBranch {
		currentBranch, _ := c.git.CurrentBranch(ctx)
		taskBranch := c.activeTask.Branch
		worktreePath := c.activeTask.WorktreePath

		// NOTE: Cleanup errors below are logged but not returned intentionally.
		// Delete operation should succeed even if cleanup partially fails.
		// This is best-effort cleanup that should not block task deletion.

		// If using worktree, remove it first
		if worktreePath != "" {
			if err := c.git.RemoveWorktree(ctx, worktreePath, true); err != nil {
				c.logError(fmt.Errorf("remove worktree: %w", err))
			}
		} else if currentBranch == taskBranch {
			// If we're on the task branch (not worktree), switch to base branch first
			var baseBranch string
			if c.taskWork != nil && c.taskWork.Git.BaseBranch != "" {
				baseBranch = c.taskWork.Git.BaseBranch
			} else {
				var err error
				baseBranch, err = c.git.GetBaseBranch(ctx)
				if err != nil {
					return fmt.Errorf("get base branch: %w", err)
				}
			}

			if err := c.git.Checkout(ctx, baseBranch); err != nil {
				return fmt.Errorf("checkout base branch: %w", err)
			}
		}

		// Checkpoint deletion is best-effort; ignore errors
		_ = c.git.DeleteAllCheckpoints(ctx, taskID)

		// Delete the branch
		if err := c.git.DeleteBranch(ctx, taskBranch, true); err != nil {
			c.logError(fmt.Errorf("delete branch: %w", err))
		}
	}

	// Delete work directory based on: CLI flag > config > default (delete)
	var shouldDelete bool
	if opts.DeleteWork != nil {
		shouldDelete = *opts.DeleteWork // CLI explicitly set
	} else {
		cfg, _ := c.workspace.LoadConfig()              // ignore error, use defaults
		shouldDelete = cfg.Workflow.DeleteWorkOnAbandon // default: true
	}
	if shouldDelete {
		if err := c.workspace.DeleteWork(taskID); err != nil {
			c.logError(fmt.Errorf("delete work directory: %w", err))
		}
	}

	// Clear active task
	if err := c.workspace.ClearActiveTask(); err != nil {
		c.logError(fmt.Errorf("clear active task: %w", err))
	}

	c.activeTask = nil
	c.taskWork = nil

	c.publishProgress("Task deleted", 100)
	return nil
}

// Status returns the current task status.
func (c *Conductor) Status() (*TaskStatus, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.activeTask == nil {
		return nil, fmt.Errorf("no active task")
	}

	// Count specifications - errors ignored; empty list is acceptable for status display
	specifications, _ := c.workspace.ListSpecifications(c.activeTask.ID)

	status := &TaskStatus{
		TaskID:         c.activeTask.ID,
		Title:          c.taskWork.Metadata.Title,
		ExternalKey:    c.taskWork.Metadata.ExternalKey,
		State:          c.activeTask.State,
		Ref:            c.activeTask.Ref,
		Branch:         c.activeTask.Branch,
		WorktreePath:   c.activeTask.WorktreePath,
		Specifications: len(specifications),
		Checkpoints:    c.countCheckpoints(),
		Started:        c.activeTask.Started,
	}

	// Add agent info
	if c.taskWork != nil && c.taskWork.Agent.Name != "" {
		status.Agent = c.taskWork.Agent.Name
		status.AgentSource = c.taskWork.Agent.Source
	} else if c.activeAgent != nil {
		status.Agent = c.activeAgent.Name()
		status.AgentSource = "auto"
	}

	return status, nil
}

// TaskStatus represents the current task state.
type TaskStatus struct {
	TaskID         string
	Title          string
	ExternalKey    string // User-facing key (e.g., "FEATURE-123")
	State          string
	Ref            string
	Branch         string
	WorktreePath   string
	Specifications int
	Checkpoints    int
	Started        time.Time
	Agent          string // Agent name being used
	AgentSource    string // Where agent was configured from: "cli", "task", "workspace", "auto"
}
