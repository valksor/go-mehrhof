package conductor

import (
	"fmt"

	"github.com/valksor/go-mehrhof/internal/naming"
	"github.com/valksor/go-mehrhof/internal/provider"
)

// gitInfo holds git branch/worktree information created during task start.
type gitInfo struct {
	branchName    string
	baseBranch    string
	worktreePath  string
	commitPrefix  string // Resolved commit prefix (e.g., "[FEATURE-123]")
	branchPattern string // Template used to generate branch
}

// namingInfo holds resolved naming for a task.
type namingInfo struct {
	externalKey   string // User-facing key (e.g., "FEATURE-123")
	taskType      string // Task type (e.g., "feature", "fix")
	slug          string // URL-safe title slug
	branchName    string // Resolved branch name
	commitPrefix  string // Resolved commit prefix
	branchPattern string // Template used for branch
}

// resolveNaming resolves external key, branch name, and commit prefix from workUnit and options.
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

	// Resolve title: CLI flag > workUnit
	title := workUnit.Title
	if c.opts.TitleOverride != "" {
		title = c.opts.TitleOverride
	}

	// Resolve slug: CLI flag > workUnit > generated from title
	slug := workUnit.Slug
	if slug == "" {
		slug = naming.Slugify(title, 50)
	}
	if c.opts.SlugOverride != "" {
		slug = c.opts.SlugOverride
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
		Title:  title,
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

// createBranchOrWorktree creates a git branch or worktree for the task.
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

// resolveTargetBranch determines the target branch for merging.
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

// performMerge handles the merge operation (squash or regular).
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
