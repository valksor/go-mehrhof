package conductor

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/valksor/go-mehrhof/internal/agent"
	"github.com/valksor/go-mehrhof/internal/events"
	"github.com/valksor/go-mehrhof/internal/naming"
	"github.com/valksor/go-mehrhof/internal/plugin"
	"github.com/valksor/go-mehrhof/internal/provider"
	"github.com/valksor/go-mehrhof/internal/storage"
	"github.com/valksor/go-mehrhof/internal/vcs"
	"github.com/valksor/go-mehrhof/internal/workflow"
)

// Conductor orchestrates the task automation workflow
type Conductor struct {
	mu sync.RWMutex

	// Core components
	machine   *workflow.Machine
	eventBus  *events.Bus
	workspace *storage.Workspace
	git       *vcs.Git

	// Registries
	providers *provider.Registry
	agents    *agent.Registry
	plugins   *plugin.Registry

	// Workflow plugin adapters (for lifecycle management)
	workflowAdapters []*plugin.WorkflowAdapter

	// Current state
	activeTask *storage.ActiveTask
	taskWork   *storage.TaskWork

	// Configuration
	opts Options

	// Active agent
	activeAgent     agent.Agent
	taskAgentConfig *provider.AgentConfig // Agent config from task source (if any)

	// Session tracking (for conversation history and token usage)
	currentSession     *storage.Session
	currentSessionFile string
}

// New creates a new Conductor with the given options
func New(opts ...Option) (*Conductor, error) {
	options := DefaultOptions()
	options.Apply(opts...)

	// Create event bus
	bus := events.NewBus()

	// Create state machine
	machine := workflow.NewMachine(bus)

	// Create registries
	providerRegistry := provider.NewRegistry()
	agentRegistry := agent.NewRegistry()

	c := &Conductor{
		machine:   machine,
		eventBus:  bus,
		providers: providerRegistry,
		agents:    agentRegistry,
		opts:      options,
	}

	// Subscribe to state changes
	bus.Subscribe(events.TypeStateChanged, c.onStateChanged)

	return c, nil
}

// applyAgentEnv applies environment variables to an agent instance.
// It resolves any ${VAR} references in the env map and applies each key-value pair.
// This is a helper to avoid code duplication across agent resolution logic.
func applyAgentEnv(agentInst agent.Agent, env map[string]string) agent.Agent {
	if len(env) == 0 {
		return agentInst
	}
	resolvedEnv := agent.ResolveEnvReferences(env)
	for k, v := range resolvedEnv {
		agentInst = agentInst.WithEnv(k, v)
	}
	return agentInst
}

// readOptionalWorkspaceData reads optional workspace files, returning empty values
// for any that don't exist. This is used for context gathering where missing files
// are not errors.
func (c *Conductor) readOptionalWorkspaceData(taskID string) (
	sourceContent string, notes string, specs string, pendingQ *storage.PendingQuestion,
) {
	sourceContent, _ = c.workspace.GetSourceContent(taskID)
	notes, _ = c.workspace.ReadNotes(taskID)
	specs, _ = c.workspace.GatherSpecificationsContent(taskID)
	pendingQ, _ = c.workspace.LoadPendingQuestion(taskID)
	return sourceContent, notes, specs, pendingQ
}

// Initialize sets up the conductor for a repository
func (c *Conductor) Initialize(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Initialize git (optional - might not be in a git repo)
	git, err := vcs.New(c.opts.WorkDir)
	if err == nil {
		c.git = git
	}

	// Determine workspace root
	// If we're in a worktree, use the main repo for storage
	root := c.opts.WorkDir
	if c.git != nil {
		if c.git.IsWorktree() {
			// Get main repo path for shared storage
			mainRepo, err := c.git.GetMainWorktreePath()
			if err != nil {
				return fmt.Errorf("get main repo from worktree: %w", err)
			}
			root = mainRepo
		} else {
			root = c.git.Root()
		}
	}

	// Initialize workspace
	ws, err := storage.OpenWorkspace(root)
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
				// Log warning but don't fail initialization
				// Plugins are optional and shouldn't block core functionality
				_ = err // In production, use proper logging
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

// Start registers a new task from a reference (does not run planning)
func (c *Conductor) Start(ctx context.Context, reference string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Reject starting new tasks from within a worktree
	if c.git != nil && c.git.IsWorktree() {
		mainRepo, _ := c.git.GetMainWorktreePath()
		return fmt.Errorf("this command must be run from the main repository; you are currently in a worktree, return to the main repository first: cd %s", mainRepo)
	}

	// Check for existing active task (only applies in main repo)
	if c.activeTask != nil {
		return fmt.Errorf("task already active: %s (use 'task status' to check)", c.activeTask.ID)
	}

	// If git is available and branch creation requested, check for clean workspace FIRST
	if err := c.ensureCleanWorkspace(); err != nil {
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
	gitInfo, err := c.createBranchOrWorktree(taskID, namingInfo)
	if err != nil {
		return err
	}

	// Snapshot the source (read-only copy)
	sourceInfo := c.snapshotSource(ctx, p, reference, workUnit)

	// Register the task with workspace
	if err := c.registerTask(taskID, reference, workUnit, sourceInfo, gitInfo, namingInfo); err != nil {
		return err
	}

	c.publishProgress("Task registered", 100)
	return nil
}

// ensureCleanWorkspace checks if workspace is clean when branch creation is requested
func (c *Conductor) ensureCleanWorkspace() error {
	if c.git == nil || !c.opts.CreateBranch {
		return nil
	}

	hasChanges, err := c.git.HasChanges()
	if err != nil {
		return fmt.Errorf("check git status: %w", err)
	}
	if hasChanges {
		return fmt.Errorf("workspace has uncommitted changes\nPlease commit or stash your changes before starting a new task with --branch")
	}
	return nil
}

// fetchWorkUnit resolves the provider and fetches the work unit
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

// gitInfo holds git branch/worktree information created during task start
type gitInfo struct {
	branchName    string
	baseBranch    string
	worktreePath  string
	commitPrefix  string // Resolved commit prefix (e.g., "[FEATURE-123]")
	branchPattern string // Template used to generate branch
}

// namingInfo holds resolved naming for a task
type namingInfo struct {
	externalKey   string // User-facing key (e.g., "FEATURE-123")
	taskType      string // Task type (e.g., "feature", "fix")
	slug          string // URL-safe title slug
	branchName    string // Resolved branch name
	commitPrefix  string // Resolved commit prefix
	branchPattern string // Template used for branch
}

// resolveNaming resolves external key, branch name, and commit prefix from workUnit and options
func (c *Conductor) resolveNaming(workUnit *provider.WorkUnit, taskID string) *namingInfo {
	// Load workspace config for templates
	cfg, _ := c.workspace.LoadConfig()

	// Resolve external key: CLI flag > workUnit > taskID fallback
	externalKey := c.opts.ExternalKey
	if externalKey == "" {
		externalKey = workUnit.ExternalKey
	}
	if externalKey == "" {
		externalKey = taskID
	}

	// Resolve task type
	taskType := workUnit.TaskType
	if taskType == "" {
		taskType = "task"
	}

	// Resolve slug
	slug := workUnit.Slug
	if slug == "" {
		slug = naming.Slugify(workUnit.Title, 50)
	}

	// Resolve branch pattern template: CLI flag > workspace config
	branchPattern := c.opts.BranchPatternTemplate
	if branchPattern == "" {
		branchPattern = cfg.Git.BranchPattern
	}

	// Resolve commit prefix template: CLI flag > workspace config
	commitPrefixTemplate := c.opts.CommitPrefixTemplate
	if commitPrefixTemplate == "" {
		commitPrefixTemplate = cfg.Git.CommitPrefix
	}

	// Build template variables
	vars := naming.TemplateVars{
		Key:    externalKey,
		TaskID: taskID,
		Type:   taskType,
		Slug:   slug,
		Title:  workUnit.Title,
	}

	// Expand templates
	branchName := naming.ExpandTemplate(branchPattern, vars)
	branchName = naming.CleanBranchName(branchName)

	commitPrefix := naming.ExpandTemplate(commitPrefixTemplate, vars)

	return &namingInfo{
		externalKey:   externalKey,
		taskType:      taskType,
		slug:          slug,
		branchName:    branchName,
		commitPrefix:  commitPrefix,
		branchPattern: branchPattern,
	}
}

// createBranchOrWorktree creates a git branch or worktree for the task
func (c *Conductor) createBranchOrWorktree(taskID string, ni *namingInfo) (*gitInfo, error) {
	if c.git == nil || !c.opts.CreateBranch {
		return &gitInfo{}, nil
	}

	baseBranch, _ := c.git.GetBaseBranch()
	branchName := ni.branchName // Use resolved branch name from naming

	if c.opts.UseWorktree {
		worktreePath := c.git.GetWorktreePath(taskID)
		if err := c.git.EnsureWorktreesDir(); err != nil {
			return nil, fmt.Errorf("create worktrees directory: %w", err)
		}
		if err := c.git.CreateWorktreeNewBranch(worktreePath, branchName, baseBranch); err != nil {
			return nil, fmt.Errorf("create worktree: %w", err)
		}
		return &gitInfo{
			branchName:    branchName,
			baseBranch:    baseBranch,
			worktreePath:  worktreePath,
			commitPrefix:  ni.commitPrefix,
			branchPattern: ni.branchPattern,
		}, nil
	}

	// Create and checkout the branch
	if err := c.git.CreateBranch(branchName, baseBranch); err != nil {
		return nil, fmt.Errorf("create branch: %w", err)
	}

	// Switch to the new branch
	if err := c.git.Checkout(branchName); err != nil {
		// Try to clean up the branch we just created
		_ = c.git.DeleteBranch(branchName, false)
		return nil, fmt.Errorf("checkout branch: %w", err)
	}

	return &gitInfo{
		branchName:    branchName,
		baseBranch:    baseBranch,
		commitPrefix:  ni.commitPrefix,
		branchPattern: ni.branchPattern,
	}, nil
}

// snapshotSource creates a snapshot of the source content
func (c *Conductor) snapshotSource(ctx context.Context, p any, reference string, workUnit *provider.WorkUnit) storage.SourceInfo {
	if snapshotter, ok := p.(provider.Snapshotter); ok {
		snapshot, err := snapshotter.Snapshot(ctx, workUnit.ID)
		if err == nil {
			sourceInfo := storage.SourceInfo{
				Type:    snapshot.Type,
				Ref:     snapshot.Ref,
				ReadAt:  time.Now(),
				Content: snapshot.Content,
			}
			for _, f := range snapshot.Files {
				sourceInfo.Files = append(sourceInfo.Files, storage.SourceFile{
					Path:    f.Path,
					Content: f.Content,
				})
			}
			return sourceInfo
		}
	}

	// Fallback: store reference only
	return storage.SourceInfo{
		Type:   workUnit.Provider,
		Ref:    reference,
		ReadAt: time.Now(),
	}
}

// registerTask creates the work directory and active task reference
func (c *Conductor) registerTask(taskID, reference string, workUnit *provider.WorkUnit, sourceInfo storage.SourceInfo, gi *gitInfo, ni *namingInfo) error {
	// Resolve agent for this task (uses priority: CLI > task > workspace > auto)
	agentInst, agentSource, err := c.resolveAgentForTask()
	if err != nil {
		return fmt.Errorf("resolve agent: %w", err)
	}
	c.activeAgent = agentInst

	// Create work directory
	work, err := c.workspace.CreateWork(taskID, sourceInfo)
	if err != nil {
		return fmt.Errorf("create work: %w", err)
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

// Resume loads an existing active task
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

// Plan enters the planning phase to create specifications
func (c *Conductor) Plan(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.activeTask == nil {
		return fmt.Errorf("no active task")
	}

	// Update state
	c.activeTask.State = "planning"
	if err := c.workspace.SaveActiveTask(c.activeTask); err != nil {
		return fmt.Errorf("save active task: %w", err)
	}

	// Dispatch planning event
	if err := c.machine.Dispatch(ctx, workflow.EventPlan); err != nil {
		return fmt.Errorf("enter planning: %w", err)
	}

	return nil
}

// Talk enters dialogue mode to add notes
func (c *Conductor) Talk(ctx context.Context, message string, opts TalkOptions) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.activeTask == nil {
		return fmt.Errorf("no active task")
	}

	taskID := c.activeTask.ID

	// Dispatch dialogue start
	if err := c.machine.Dispatch(ctx, workflow.EventDialogueStart); err != nil {
		return fmt.Errorf("enter talk mode: %w", err)
	}

	// Get agent for dialogue step
	dialogueAgent, err := c.GetAgentForStep(workflow.StepDialogue)
	if err != nil {
		_ = c.machine.Dispatch(ctx, workflow.EventDialogueEnd)
		return fmt.Errorf("get dialogue agent: %w", err)
	}

	// Build context-aware prompt for talk mode
	// This ensures Claude has full awareness of the task when answering
	sourceContent, notes, specs, pendingQ := c.readOptionalWorkspaceData(taskID)

	prompt := buildTalkPrompt(c.taskWork.Metadata.Title, sourceContent, notes, specs, pendingQ, message)

	// Run agent with context-aware prompt
	response, err := dialogueAgent.Run(ctx, prompt)
	if err != nil {
		// End dialogue even on error
		_ = c.machine.Dispatch(ctx, workflow.EventDialogueEnd)
		return fmt.Errorf("agent run: %w", err)
	}

	// Save response as note
	noteContent := response.Summary
	if noteContent == "" && len(response.Messages) > 0 {
		noteContent = response.Messages[0]
	}
	if noteContent != "" {
		if err := c.workspace.AppendNote(taskID, noteContent, c.activeTask.State); err != nil {
			c.logError(fmt.Errorf("append note: %w", err))
		}
	}

	// Apply file changes if not dry-run
	if !c.opts.DryRun && len(response.Files) > 0 {
		if err := c.applyFileChanges(ctx, response.Files); err != nil {
			c.logError(fmt.Errorf("apply changes: %w", err))
		}
	}

	// Clear pending question if it existed (user has answered via talk)
	if c.workspace.HasPendingQuestion(taskID) {
		_ = c.workspace.ClearPendingQuestion(taskID)
	}

	// Return to previous state
	if err := c.machine.Dispatch(ctx, workflow.EventDialogueEnd); err != nil {
		return fmt.Errorf("exit talk mode: %w", err)
	}

	return nil
}

// Implement enters the implementation phase
func (c *Conductor) Implement(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.activeTask == nil {
		return fmt.Errorf("no active task")
	}

	// Check for specifications
	specifications, err := c.workspace.ListSpecifications(c.activeTask.ID)
	if err != nil {
		return fmt.Errorf("list specifications: %w", err)
	}
	if len(specifications) == 0 {
		return fmt.Errorf("no specifications found - run 'task plan' first")
	}

	// Update machine with specifications
	wu := c.machine.WorkUnit()
	if wu != nil {
		wu.Specifications = make([]string, len(specifications))
		for i, num := range specifications {
			wu.Specifications[i] = fmt.Sprintf("specification-%d.md", num)
		}
	}

	// Update state
	c.activeTask.State = "implementing"
	if err := c.workspace.SaveActiveTask(c.activeTask); err != nil {
		return fmt.Errorf("save active task: %w", err)
	}

	// Dispatch implement event
	if err := c.machine.Dispatch(ctx, workflow.EventImplement); err != nil {
		return fmt.Errorf("enter implementation: %w", err)
	}

	return nil
}

// Review enters the review phase
func (c *Conductor) Review(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.activeTask == nil {
		return fmt.Errorf("no active task")
	}

	// Update state
	c.activeTask.State = "reviewing"
	if err := c.workspace.SaveActiveTask(c.activeTask); err != nil {
		return fmt.Errorf("save active task: %w", err)
	}

	// Dispatch review event
	if err := c.machine.Dispatch(ctx, workflow.EventReview); err != nil {
		return fmt.Errorf("enter review: %w", err)
	}

	return nil
}

// Undo reverts to the previous checkpoint
func (c *Conductor) Undo(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.activeTask == nil {
		return fmt.Errorf("no active task")
	}

	if c.git == nil {
		return fmt.Errorf("git not available")
	}

	taskID := c.activeTask.ID

	// Check if undo is possible
	can, err := c.git.CanUndo(taskID)
	if err != nil {
		return err
	}
	if !can {
		return fmt.Errorf("nothing to undo")
	}

	// Dispatch undo event
	if err := c.machine.Dispatch(ctx, workflow.EventUndo); err != nil {
		return fmt.Errorf("undo workflow: %w", err)
	}

	// Perform git undo
	checkpoint, err := c.git.Undo(taskID)
	if err != nil {
		return fmt.Errorf("git undo: %w", err)
	}

	// Publish event
	c.eventBus.PublishRaw(events.Event{
		Type: events.TypeCheckpoint,
		Data: map[string]any{
			"action":     "undo",
			"checkpoint": checkpoint.Number,
			"commit":     checkpoint.ID,
		},
	})

	// Complete undo
	_ = c.machine.Dispatch(ctx, workflow.EventUndoDone)

	return nil
}

// Redo moves forward to the next checkpoint
func (c *Conductor) Redo(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.activeTask == nil {
		return fmt.Errorf("no active task")
	}

	if c.git == nil {
		return fmt.Errorf("git not available")
	}

	taskID := c.activeTask.ID

	// Check if redo is possible
	can, err := c.git.CanRedo(taskID)
	if err != nil {
		return err
	}
	if !can {
		return fmt.Errorf("nothing to redo")
	}

	// Dispatch redo event
	if err := c.machine.Dispatch(ctx, workflow.EventRedo); err != nil {
		return fmt.Errorf("redo workflow: %w", err)
	}

	// Perform git redo
	checkpoint, err := c.git.Redo(taskID)
	if err != nil {
		return fmt.Errorf("git redo: %w", err)
	}

	// Publish event
	c.eventBus.PublishRaw(events.Event{
		Type: events.TypeCheckpoint,
		Data: map[string]any{
			"action":     "redo",
			"checkpoint": checkpoint.Number,
			"commit":     checkpoint.ID,
		},
	})

	// Complete redo
	_ = c.machine.Dispatch(ctx, workflow.EventRedoDone)

	return nil
}

// Finish completes the task
func (c *Conductor) Finish(ctx context.Context, opts FinishOptions) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.activeTask == nil {
		return fmt.Errorf("no active task")
	}

	// Handle PR creation if requested
	if opts.CreatePR {
		prResult, err := c.finishWithPR(ctx, opts)
		if err != nil {
			return err
		}
		// Store PR info for later reference
		if prResult != nil {
			c.logVerbose("Created PR #%d: %s", prResult.Number, prResult.URL)
		}
	} else if c.git != nil && c.activeTask.UseGit && c.activeTask.Branch != "" {
		// Handle git merge operations if applicable
		if err := c.performMerge(opts); err != nil {
			return err
		}

		// Push if requested
		if opts.PushAfter {
			targetBranch := c.resolveTargetBranch(opts.TargetBranch)
			if err := c.git.PushBranch(targetBranch, "origin", false); err != nil {
				return fmt.Errorf("push: %w", err)
			}
		}

		// Cleanup branch and worktree
		c.cleanupAfterMerge(opts)
	}

	// Update state
	c.activeTask.State = "done"
	if err := c.workspace.SaveActiveTask(c.activeTask); err != nil {
		c.logError(fmt.Errorf("save active task: %w", err))
	}

	// Dispatch finish event
	if err := c.machine.Dispatch(ctx, workflow.EventFinish); err != nil {
		return fmt.Errorf("finish workflow: %w", err)
	}

	// Clear active task
	if err := c.workspace.ClearActiveTask(); err != nil {
		c.logError(fmt.Errorf("clear active task: %w", err))
	}

	c.activeTask = nil
	c.taskWork = nil

	return nil
}

// resolveTargetBranch determines the target branch for merging
func (c *Conductor) resolveTargetBranch(requested string) string {
	if requested != "" {
		return requested
	}

	// Use the stored base branch from when task was started
	if c.taskWork != nil && c.taskWork.Git.BaseBranch != "" {
		return c.taskWork.Git.BaseBranch
	}

	// Fallback to detecting base branch
	baseBranch, _ := c.git.GetBaseBranch()
	return baseBranch
}

// performMerge handles the merge operation (squash or regular)
func (c *Conductor) performMerge(opts FinishOptions) error {
	targetBranch := c.resolveTargetBranch(opts.TargetBranch)
	currentBranch := c.activeTask.Branch
	taskID := c.activeTask.ID

	// Checkout target branch
	if err := c.git.Checkout(targetBranch); err != nil {
		return fmt.Errorf("checkout target: %w", err)
	}

	// Merge (squash or regular)
	if opts.SquashMerge {
		if err := c.git.MergeSquash(currentBranch); err != nil {
			_ = c.git.Checkout(currentBranch)
			return fmt.Errorf("squash merge: %w", err)
		}
		// Use stored commit prefix, fallback to task ID if not set
		prefix := c.taskWork.Git.CommitPrefix
		if prefix == "" {
			prefix = fmt.Sprintf("(%s)", taskID)
		}
		msg := fmt.Sprintf("%s merged from %s", prefix, currentBranch)
		if _, err := c.git.Commit(msg); err != nil {
			_ = c.git.Checkout(currentBranch)
			return fmt.Errorf("commit merge: %w", err)
		}
	} else {
		if err := c.git.MergeBranch(currentBranch, true); err != nil {
			_ = c.git.Checkout(currentBranch)
			return fmt.Errorf("merge: %w", err)
		}
	}

	return nil
}

// cleanupAfterMerge removes the branch and worktree after successful merge
// NOTE: Errors are logged but not returned intentionally.
// The merge succeeded, so cleanup failures should not undo the user's work.
func (c *Conductor) cleanupAfterMerge(opts FinishOptions) {
	currentBranch := c.activeTask.Branch
	targetBranch := c.resolveTargetBranch(opts.TargetBranch)
	taskID := c.activeTask.ID

	if !opts.DeleteBranch || currentBranch == targetBranch {
		return
	}

	// Checkpoint deletion is best-effort; ignore errors
	_ = c.git.DeleteAllCheckpoints(taskID)

	// If using worktree, remove it first
	if worktreePath := c.activeTask.WorktreePath; worktreePath != "" {
		if err := c.git.RemoveWorktree(worktreePath, true); err != nil {
			c.logError(fmt.Errorf("remove worktree: %w", err))
		}
	}

	if err := c.git.DeleteBranch(currentBranch, true); err != nil {
		c.logError(fmt.Errorf("delete branch: %w", err))
	}
}

// finishWithPR creates a pull request instead of merging locally
func (c *Conductor) finishWithPR(ctx context.Context, opts FinishOptions) (*provider.PullRequest, error) {
	if c.git == nil {
		return nil, fmt.Errorf("git not available for PR creation")
	}

	if c.activeTask.Branch == "" {
		return nil, fmt.Errorf("no branch associated with task for PR creation")
	}

	taskID := c.activeTask.ID
	sourceBranch := c.activeTask.Branch

	// Push the branch to remote first
	if err := c.git.PushBranch(sourceBranch, "origin", true); err != nil {
		return nil, fmt.Errorf("push branch: %w", err)
	}

	// Resolve the provider from the original reference
	p, err := c.resolveTaskProvider(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve provider: %w", err)
	}

	// Check if provider supports PR creation
	prCreator, ok := p.(provider.PRCreator)
	if !ok {
		return nil, fmt.Errorf("provider does not support PR creation")
	}

	// Load specifications for PR body
	specs, err := c.loadSpecificationsForPR()
	if err != nil {
		c.logError(fmt.Errorf("load specifications for PR: %w", err))
		// Continue without specs, not fatal
	}

	// Get diff stats
	diffStat := c.getDiffStats()

	// Determine target branch
	targetBranch := opts.TargetBranch
	if targetBranch == "" {
		targetBranch = c.resolveTargetBranch("")
	}

	// Build PR options
	prOpts := provider.PullRequestOptions{
		Title:        opts.PRTitle,
		Body:         opts.PRBody,
		SourceBranch: sourceBranch,
		TargetBranch: targetBranch,
		Draft:        opts.DraftPR,
	}

	// Generate title if not provided
	if prOpts.Title == "" {
		prOpts.Title = c.generatePRTitle()
	}

	// Generate body if not provided
	if prOpts.Body == "" {
		prOpts.Body = c.generatePRBody(specs, diffStat)
	}

	// Create the PR
	pr, err := prCreator.CreatePullRequest(ctx, prOpts)
	if err != nil {
		return nil, fmt.Errorf("create pull request: %w", err)
	}

	// Publish PRCreatedEvent
	c.eventBus.Publish(events.PRCreatedEvent{
		TaskID:   taskID,
		PRNumber: pr.Number,
		PRURL:    pr.URL,
	})

	// Post comment to issue if configured (GitHub-specific)
	if commenter, ok := p.(provider.Commenter); ok {
		issueID := c.taskWork.Metadata.ExternalKey
		if issueID != "" {
			comment := fmt.Sprintf("Pull request created: #%d\n%s\n\nThe PR includes all changes from branch `%s`.",
				pr.Number, pr.URL, sourceBranch)
			if _, err := commenter.AddComment(ctx, issueID, comment); err != nil {
				c.logError(fmt.Errorf("add PR comment to issue: %w", err))
				// Don't fail the PR creation
			}
		}
	}

	return pr, nil
}

// resolveTaskProvider resolves the provider from the task's original reference
func (c *Conductor) resolveTaskProvider(ctx context.Context) (any, error) {
	if c.activeTask == nil || c.activeTask.Ref == "" {
		return nil, fmt.Errorf("no task reference available")
	}

	// Resolve provider from the stored reference
	resolveOpts := provider.ResolveOptions{
		DefaultProvider: c.opts.DefaultProvider,
	}
	p, _, err := c.providers.Resolve(ctx, c.activeTask.Ref, provider.Config{}, resolveOpts)
	if err != nil {
		return nil, err
	}

	return p, nil
}

// loadSpecificationsForPR loads all specifications for PR body generation
func (c *Conductor) loadSpecificationsForPR() ([]*storage.Specification, error) {
	if c.workspace == nil || c.activeTask == nil {
		return nil, nil
	}

	taskID := c.activeTask.ID
	specNums, err := c.workspace.ListSpecifications(taskID)
	if err != nil {
		return nil, err
	}

	var specs []*storage.Specification
	for _, num := range specNums {
		content, err := c.workspace.LoadSpecification(taskID, num)
		if err != nil {
			continue
		}
		specs = append(specs, &storage.Specification{
			Number:  num,
			Content: content,
		})
	}

	return specs, nil
}

// getDiffStats returns git diff stats for the current branch vs base
func (c *Conductor) getDiffStats() string {
	if c.git == nil || c.taskWork == nil {
		return ""
	}

	baseBranch := c.taskWork.Git.BaseBranch
	if baseBranch == "" {
		baseBranch, _ = c.git.GetBaseBranch()
	}

	if baseBranch == "" {
		return ""
	}

	// git diff --stat base..HEAD
	stat, err := c.git.Diff("--stat", baseBranch+"..HEAD")
	if err != nil {
		return ""
	}

	return stat
}

// generatePRTitle generates a PR title from task metadata
func (c *Conductor) generatePRTitle() string {
	if c.taskWork == nil {
		return "Implementation"
	}

	var title string
	if c.taskWork.Metadata.ExternalKey != "" {
		title = fmt.Sprintf("[#%s] ", c.taskWork.Metadata.ExternalKey)
	}

	if c.taskWork.Metadata.Title != "" {
		title += c.taskWork.Metadata.Title
	} else {
		title += "Implementation"
	}

	return title
}

// generatePRBody generates a PR body with implementation summary
func (c *Conductor) generatePRBody(specs []*storage.Specification, diffStat string) string {
	var parts []string

	// Summary section
	parts = append(parts, "## Summary\n")

	if c.taskWork != nil && c.taskWork.Metadata.Title != "" {
		parts = append(parts, fmt.Sprintf("Implementation for: %s\n", c.taskWork.Metadata.Title))
	}

	// Link to issue if this is a GitHub issue task
	if c.taskWork != nil && c.taskWork.Source.Type == "github" && c.taskWork.Metadata.ExternalKey != "" {
		parts = append(parts, fmt.Sprintf("Closes #%s\n", c.taskWork.Metadata.ExternalKey))
	}

	// Specifications section
	if len(specs) > 0 {
		parts = append(parts, "\n## Implementation Details\n")
		for _, spec := range specs {
			if spec.Title != "" {
				parts = append(parts, fmt.Sprintf("### %s\n", spec.Title))
			}
			// Include first 500 chars of spec content as summary
			content := spec.Content
			if len(content) > 500 {
				content = content[:500] + "..."
			}
			parts = append(parts, content+"\n")
		}
	}

	// Changes section
	if diffStat != "" {
		parts = append(parts, "\n## Changes\n")
		parts = append(parts, "```\n"+diffStat+"\n```\n")
	}

	// Test plan section
	parts = append(parts, "\n## Test Plan\n")
	parts = append(parts, "- [ ] Manual testing\n")
	parts = append(parts, "- [ ] Unit tests pass\n")
	parts = append(parts, "- [ ] Code review\n")

	// Footer
	parts = append(parts, "\n---\n")
	parts = append(parts, "*Generated by [Mehrhof](https://github.com/valksor/go-mehrhof)*\n")

	var result string
	for _, p := range parts {
		result += p
	}
	return result
}

// logVerbose logs a message if verbose mode is enabled
func (c *Conductor) logVerbose(format string, args ...any) {
	if c.opts.Verbose && c.opts.Stdout != nil {
		_, _ = fmt.Fprintf(c.opts.Stdout, format+"\n", args...)
	}
}

// Delete abandons the current task without merging
func (c *Conductor) Delete(ctx context.Context, opts DeleteOptions) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.activeTask == nil {
		return fmt.Errorf("no active task")
	}

	taskID := c.activeTask.ID

	// Handle git operations if applicable
	if c.git != nil && c.activeTask.UseGit && c.activeTask.Branch != "" && !opts.KeepBranch {
		currentBranch, _ := c.git.CurrentBranch()
		taskBranch := c.activeTask.Branch
		worktreePath := c.activeTask.WorktreePath

		// NOTE: Cleanup errors below are logged but not returned intentionally.
		// Delete operation should succeed even if cleanup partially fails.
		// This is best-effort cleanup that should not block task deletion.

		// If using worktree, remove it first
		if worktreePath != "" {
			if err := c.git.RemoveWorktree(worktreePath, true); err != nil {
				c.logError(fmt.Errorf("remove worktree: %w", err))
			}
		} else if currentBranch == taskBranch {
			// If we're on the task branch (not worktree), switch to base branch first
			baseBranch := ""
			if c.taskWork != nil && c.taskWork.Git.BaseBranch != "" {
				baseBranch = c.taskWork.Git.BaseBranch
			} else {
				var err error
				baseBranch, err = c.git.GetBaseBranch()
				if err != nil {
					return fmt.Errorf("get base branch: %w", err)
				}
			}

			if err := c.git.Checkout(baseBranch); err != nil {
				return fmt.Errorf("checkout base branch: %w", err)
			}
		}

		// Checkpoint deletion is best-effort; ignore errors
		_ = c.git.DeleteAllCheckpoints(taskID)

		// Delete the branch
		if err := c.git.DeleteBranch(taskBranch, true); err != nil {
			c.logError(fmt.Errorf("delete branch: %w", err))
		}
	}

	// Delete work directory
	if !opts.KeepWorkDir {
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

// Status returns the current task status
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

// TaskStatus represents the current task state
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

// GetProviderRegistry returns the provider registry
func (c *Conductor) GetProviderRegistry() *provider.Registry {
	return c.providers
}

// GetAgentRegistry returns the agent registry
func (c *Conductor) GetAgentRegistry() *agent.Registry {
	return c.agents
}

// GetEventBus returns the event bus
func (c *Conductor) GetEventBus() *events.Bus {
	return c.eventBus
}

// GetWorkspace returns the workspace
func (c *Conductor) GetWorkspace() *storage.Workspace {
	return c.workspace
}

// GetGit returns the git instance
func (c *Conductor) GetGit() *vcs.Git {
	return c.git
}

// GetActiveTask returns the current active task
func (c *Conductor) GetActiveTask() *storage.ActiveTask {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.activeTask
}

// GetTaskWork returns the current task work
func (c *Conductor) GetTaskWork() *storage.TaskWork {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.taskWork
}

// GetActiveAgent returns the active agent
func (c *Conductor) GetActiveAgent() agent.Agent {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.activeAgent
}

// GetMachine returns the state machine
func (c *Conductor) GetMachine() *workflow.Machine {
	return c.machine
}

// GetStdout returns the configured stdout writer
func (c *Conductor) GetStdout() io.Writer {
	return c.opts.Stdout
}

// GetStderr returns the configured stderr writer
func (c *Conductor) GetStderr() io.Writer {
	return c.opts.Stderr
}

// buildWorkUnit creates a workflow.WorkUnit from current state
func (c *Conductor) buildWorkUnit() *workflow.WorkUnit {
	if c.taskWork == nil {
		return nil
	}

	wu := &workflow.WorkUnit{
		ID:         c.taskWork.Metadata.ID,
		ExternalID: c.taskWork.Source.Ref,
		Title:      c.taskWork.Metadata.Title,
		Source: &workflow.Source{
			Reference: c.taskWork.Source.Ref,
			Content:   c.taskWork.Source.Content,
		},
	}

	// Add specifications if any - errors ignored; empty list is acceptable for WorkUnit
	specifications, _ := c.workspace.ListSpecifications(c.taskWork.Metadata.ID)
	for _, num := range specifications {
		wu.Specifications = append(wu.Specifications, fmt.Sprintf("specification-%d.md", num))
	}

	return wu
}

// onStateChanged handles state change events
func (c *Conductor) onStateChanged(e events.Event) {
	if c.opts.OnStateChange == nil {
		return
	}

	from, ok := e.Data["from"].(string)
	if !ok {
		from = ""
	}
	to, ok := e.Data["to"].(string)
	if !ok {
		to = ""
	}
	c.opts.OnStateChange(from, to)
}

// applyFileChanges applies agent file changes to disk
func (c *Conductor) applyFileChanges(ctx context.Context, files []agent.FileChange) error {
	return applyFiles(ctx, c, files)
}

// logError logs an error using the callback if configured
func (c *Conductor) logError(err error) {
	if c.opts.OnError != nil {
		c.opts.OnError(err)
	}
}

// countCheckpoints returns the number of checkpoints for current task
func (c *Conductor) countCheckpoints() int {
	if c.activeTask == nil || c.git == nil {
		return 0
	}
	checkpoints, err := c.git.ListCheckpoints(c.activeTask.ID)
	if err != nil {
		return 0
	}
	return len(checkpoints)
}

// publishProgress publishes a progress event
func (c *Conductor) publishProgress(message string, percent int) {
	c.eventBus.PublishRaw(events.Event{
		Type: events.TypeProgress,
		Data: map[string]any{
			"message": message,
			"percent": percent,
		},
	})

	if c.opts.OnProgress != nil {
		c.opts.OnProgress(message, percent)
	}
}

// resolveAgentForTask resolves the agent based on priority:
// CLI flag > Task config > Workspace default > Auto-detect
// Returns the resolved agent, the source identifier, and any error.
func (c *Conductor) resolveAgentForTask() (agent.Agent, string, error) {
	var agentName string
	var source string

	// Priority 1: CLI flag (opts.AgentName)
	if c.opts.AgentName != "" {
		agentName = c.opts.AgentName
		source = "cli"
	} else if c.taskAgentConfig != nil && c.taskAgentConfig.Name != "" {
		// Priority 2: Task frontmatter agent config
		agentName = c.taskAgentConfig.Name
		source = "task"
	} else {
		// Priority 3: Workspace default or auto-detect
		if cfg, err := c.workspace.LoadConfig(); err == nil && cfg.Agent.Default != "" {
			agentName = cfg.Agent.Default
			source = "workspace"
		} else {
			// Priority 4: Auto-detect
			agentInst, err := c.agents.Detect()
			if err != nil {
				return nil, "", fmt.Errorf("detect agent: %w", err)
			}
			return agentInst, "auto", nil
		}
	}

	// Get the agent by name
	agentInst, err := c.agents.Get(agentName)
	if err != nil {
		return nil, "", fmt.Errorf("get agent %s: %w", agentName, err)
	}

	// Apply inline env vars and args from task if source is "task"
	if source == "task" && c.taskAgentConfig != nil {
		agentInst = applyAgentEnv(agentInst, c.taskAgentConfig.Env)
		if len(c.taskAgentConfig.Args) > 0 {
			agentInst = agentInst.WithArgs(c.taskAgentConfig.Args...)
		}
	}

	return agentInst, source, nil
}

// AgentResolution holds the result of agent resolution for a specific step
type AgentResolution struct {
	Agent     agent.Agent
	Source    string            // Where it was resolved from
	StepName  string            // Which step this is for
	InlineEnv map[string]string // Resolved inline env vars
	Args      []string          // CLI args for this step
}

// resolveAgentForStep resolves the agent for a specific workflow step.
// Priority: CLI step-specific > CLI global > Task step > Task default > Workspace step > Workspace default > Auto
func (c *Conductor) resolveAgentForStep(step workflow.Step) (*AgentResolution, error) {
	var agentName string
	var source string
	var inlineEnv map[string]string
	var args []string

	stepStr := step.String()

	// Priority 1: CLI step-specific flag
	if name, ok := c.opts.StepAgents[stepStr]; ok && name != "" {
		agentName = name
		source = "cli-step"
	} else if c.opts.AgentName != "" {
		// Priority 2: CLI global flag
		agentName = c.opts.AgentName
		source = "cli"
	} else if c.taskAgentConfig != nil {
		// Priority 3: Task frontmatter step-specific
		if stepCfg, ok := c.taskAgentConfig.Steps[stepStr]; ok && stepCfg.Name != "" {
			agentName = stepCfg.Name
			source = "task-step"
			inlineEnv = stepCfg.Env
			args = stepCfg.Args
		} else if c.taskAgentConfig.Name != "" {
			// Priority 4: Task frontmatter default
			agentName = c.taskAgentConfig.Name
			source = "task"
			inlineEnv = c.taskAgentConfig.Env
			args = c.taskAgentConfig.Args
		}
	}

	// Priority 5 & 6: Workspace config
	if agentName == "" {
		if cfg, err := c.workspace.LoadConfig(); err == nil {
			if stepCfg, ok := cfg.Agent.Steps[stepStr]; ok && stepCfg.Name != "" {
				// Priority 5: Workspace step-specific
				agentName = stepCfg.Name
				source = "workspace-step"
				inlineEnv = stepCfg.Env
				args = stepCfg.Args
			} else if cfg.Agent.Default != "" {
				// Priority 6: Workspace default
				agentName = cfg.Agent.Default
				source = "workspace"
			}
		}
	}

	// Priority 7: Auto-detect
	if agentName == "" {
		agentInst, err := c.agents.Detect()
		if err != nil {
			return nil, fmt.Errorf("detect agent for step %s: %w", step, err)
		}
		return &AgentResolution{
			Agent:    agentInst,
			Source:   "auto",
			StepName: stepStr,
		}, nil
	}

	// Get the agent by name
	agentInst, err := c.agents.Get(agentName)
	if err != nil {
		return nil, fmt.Errorf("get agent %s for step %s: %w", agentName, step, err)
	}

	// Apply inline env vars
	agentInst = applyAgentEnv(agentInst, inlineEnv)

	// Apply args
	if len(args) > 0 {
		agentInst = agentInst.WithArgs(args...)
	}

	return &AgentResolution{
		Agent:     agentInst,
		Source:    source,
		StepName:  stepStr,
		InlineEnv: inlineEnv,
		Args:      args,
	}, nil
}

// GetAgentForStep returns the resolved agent for a step, using cached resolution if available.
// It also persists the resolution in taskWork for task resumption.
func (c *Conductor) GetAgentForStep(step workflow.Step) (agent.Agent, error) {
	stepStr := step.String()

	// Check if we have a cached resolution for this step in taskWork
	if c.taskWork != nil && c.taskWork.Agent.Steps != nil {
		if stepInfo, ok := c.taskWork.Agent.Steps[stepStr]; ok && stepInfo.Name != "" {
			// Restore from persisted config
			agentInst, err := c.agents.Get(stepInfo.Name)
			if err == nil {
				// Re-apply inline env
				agentInst = applyAgentEnv(agentInst, stepInfo.InlineEnv)
				// Re-apply args
				if len(stepInfo.Args) > 0 {
					agentInst = agentInst.WithArgs(stepInfo.Args...)
				}
				return agentInst, nil
			}
			// Fall through to re-resolve if stored agent not found
		}
	}

	// Resolve fresh
	resolution, err := c.resolveAgentForStep(step)
	if err != nil {
		return nil, err
	}

	// Cache the resolution in taskWork for persistence
	if c.taskWork != nil {
		if c.taskWork.Agent.Steps == nil {
			c.taskWork.Agent.Steps = make(map[string]storage.StepAgentInfo)
		}
		c.taskWork.Agent.Steps[stepStr] = storage.StepAgentInfo{
			Name:      resolution.Agent.Name(),
			Source:    resolution.Source,
			InlineEnv: resolution.InlineEnv,
			Args:      resolution.Args,
		}
		// Save updated work.yaml
		_ = c.workspace.SaveWork(c.taskWork)
	}

	return resolution.Agent, nil
}

// registerAliasAgents registers user-defined agent aliases from workspace config.
// Aliases can extend built-in agents or other aliases (chained).
func (c *Conductor) registerAliasAgents(cfg *storage.WorkspaceConfig) error {
	if len(cfg.Agents) == 0 {
		return nil
	}

	// Track resolved aliases to handle chained aliases via topological sort
	resolved := make(map[string]bool)
	// Track aliases currently being resolved to detect circular dependencies
	resolving := make(map[string]bool)

	var resolve func(name string) error
	resolve = func(name string) error {
		if resolved[name] {
			return nil
		}

		if resolving[name] {
			return fmt.Errorf("circular alias dependency detected: %s", name)
		}

		alias, ok := cfg.Agents[name]
		if !ok {
			return nil // Not an alias, skip
		}

		resolving[name] = true

		// Check if base agent exists in registry
		if _, err := c.agents.Get(alias.Extends); err != nil {
			// Base agent not found - might be another alias, try to resolve it first
			if _, isAlias := cfg.Agents[alias.Extends]; isAlias {
				if err := resolve(alias.Extends); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("alias %q extends unknown agent %q", name, alias.Extends)
			}
		}

		// Get the base agent (now guaranteed to exist)
		base, err := c.agents.Get(alias.Extends)
		if err != nil {
			return fmt.Errorf("get base agent for alias %q: %w", name, err)
		}

		// Resolve environment variable references
		env := agent.ResolveEnvReferences(alias.Env)

		// Create and register the alias agent
		aliasAgent := agent.NewAlias(name, base, env, alias.Args, alias.Description)
		if err := c.agents.Register(aliasAgent); err != nil {
			return fmt.Errorf("register alias %q: %w", name, err)
		}

		resolved[name] = true
		resolving[name] = false
		return nil
	}

	// Resolve all aliases
	for name := range cfg.Agents {
		if err := resolve(name); err != nil {
			return err
		}
	}

	return nil
}

// loadPlugins discovers and loads enabled plugins
func (c *Conductor) loadPlugins(ctx context.Context, cfg *storage.WorkspaceConfig) error {
	// Skip if no plugins are enabled
	if len(cfg.Plugins.Enabled) == 0 {
		return nil
	}

	// Get plugin directories
	globalDir, err := plugin.DefaultGlobalDir()
	if err != nil {
		return fmt.Errorf("get global plugins dir: %w", err)
	}
	projectDir := plugin.DefaultProjectDir(c.workspace.Root())

	// Create plugin discovery and registry
	discovery := plugin.NewDiscovery(globalDir, projectDir)
	c.plugins = plugin.NewRegistry(discovery)

	// Configure enabled plugins
	c.plugins.SetEnabled(cfg.Plugins.Enabled)
	c.plugins.SetConfig(cfg.Plugins.Config)

	// Discover and load plugins
	if err := c.plugins.DiscoverAndLoad(ctx); err != nil {
		return fmt.Errorf("discover and load plugins: %w", err)
	}

	// Register provider plugins
	for _, info := range c.plugins.Providers() {
		if info.Process == nil {
			continue
		}

		adapter := plugin.NewProviderAdapter(info.Manifest, info.Process)
		providerInfo := provider.ProviderInfo{
			Name:         info.Manifest.Provider.Name,
			Description:  info.Manifest.Description,
			Schemes:      info.Manifest.Provider.Schemes,
			Priority:     info.Manifest.Provider.Priority,
			Capabilities: adapter.Capabilities(),
		}

		// Register the provider
		if err := c.providers.Register(providerInfo, func(ctx context.Context, cfg provider.Config) (any, error) {
			return adapter, nil
		}); err != nil {
			// Log but continue - don't fail if one plugin can't register
			continue
		}
	}

	// Register agent plugins
	for _, info := range c.plugins.Agents() {
		if info.Process == nil {
			continue
		}

		adapter := plugin.NewAgentAdapter(info.Manifest, info.Process)
		if err := c.agents.Register(adapter); err != nil {
			// Log but continue
			continue
		}
	}

	// Register workflow plugins (phases, guards, effects)
	workflowPlugins := c.plugins.Workflows()
	if len(workflowPlugins) > 0 {
		// Build a new machine with plugin extensions
		builder := workflow.NewMachineBuilder()

		for _, info := range workflowPlugins {
			if info.Process == nil {
				continue
			}

			adapter := plugin.NewWorkflowAdapter(info.Manifest, info.Process)

			// Initialize adapter with plugin-specific config
			pluginCfg := cfg.Plugins.Config[info.Manifest.Name]
			if err := adapter.Initialize(ctx, pluginCfg); err != nil {
				// Log warning but continue - don't fail if one plugin can't initialize
				continue
			}

			// Store adapter for lifecycle management
			c.workflowAdapters = append(c.workflowAdapters, adapter)

			// Register phases with the machine builder
			for _, phase := range adapter.BuildPhaseDefinitions() {
				if err := builder.RegisterPhase(phase); err != nil {
					// Log warning but continue
					continue
				}
			}
		}

		// Replace the default machine with the configured one
		c.machine = builder.Build(c.eventBus)
	}

	return nil
}

// GetPluginRegistry returns the plugin registry
func (c *Conductor) GetPluginRegistry() *plugin.Registry {
	return c.plugins
}
