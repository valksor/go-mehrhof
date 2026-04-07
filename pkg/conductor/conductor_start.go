package conductor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/valksor/kvelmo/pkg/provider"
	"github.com/valksor/kvelmo/pkg/provision"
	"github.com/valksor/kvelmo/pkg/settings"
)

// getStatusFromLabels extracts a status value from labels with "status:" prefix.
// Returns empty string if no status label is found.
func getStatusFromLabels(labels []string) string {
	for _, label := range labels {
		if after, ok := strings.CutPrefix(label, "status:"); ok {
			return after
		}
	}

	return ""
}

// Start loads a task from a source reference and begins the workflow.
// This is the "start" transition from None -> Loaded.
func (c *Conductor) Start(ctx context.Context, sourceRef string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.machine.State() != StateNone {
		return fmt.Errorf("cannot start: current state is %s (expected none)", c.machine.State())
	}

	// Parse source reference
	providerName, sourceID, err := c.providers.Parse(sourceRef)
	if err != nil {
		return fmt.Errorf("parse source: %w", err)
	}

	// Get effective settings (cached for reuse across phases)
	effectiveSettings := c.getEffectiveSettings()

	// Build hierarchy options from settings; currently Wrike-specific.
	hierarchyOpts := provider.HierarchyOptions{}
	if providerName == provider.NameWrike {
		hierarchyOpts.IncludeParent = effectiveSettings.Providers.Wrike.IncludeParentContext
		hierarchyOpts.IncludeSiblings = effectiveSettings.Providers.Wrike.IncludeSiblingContext
	}

	// Fetch task from provider, enriching with hierarchy context when supported.
	task, err := c.providers.FetchWithHierarchy(ctx, providerName, sourceID, hierarchyOpts)
	if err != nil {
		return fmt.Errorf("fetch task: %w", err)
	}

	// Build hierarchy context for the work unit from the fetched task.
	var hierarchyCtx *HierarchyContext
	if task.ParentTask != nil || len(task.SiblingTasks) > 0 {
		hierarchyCtx = &HierarchyContext{}
		if task.ParentTask != nil {
			hierarchyCtx.Parent = &TaskSummary{
				ID:          task.ParentTask.ID,
				Title:       task.ParentTask.Title,
				Description: task.ParentTask.Description,
				URL:         task.ParentTask.URL,
				Status:      getStatusFromLabels(task.ParentTask.Labels),
			}
		}
		for _, sibling := range task.SiblingTasks {
			hierarchyCtx.Siblings = append(hierarchyCtx.Siblings, TaskSummary{
				ID:          sibling.ID,
				Title:       sibling.Title,
				Description: sibling.Description,
				URL:         sibling.URL,
				Status:      getStatusFromLabels(sibling.Labels),
			})
		}
	}

	// Create work unit
	c.workUnit = &WorkUnit{
		ID:          "task-" + uuid.New().String(),
		ExternalID:  task.ID,
		Title:       task.Title,
		Description: task.Description,
		Source: &Source{
			Provider:  providerName,
			Reference: sourceRef,
			Content:   task.Description,
		},
		Hierarchy:   hierarchyCtx,
		TaskTraceID: "ttrace-" + uuid.New().String(),
		Metadata:    make(map[string]string),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Create branch (and optionally an isolated worktree) if git is available.
	if c.git != nil && settings.BoolValue(effectiveSettings.Git.CreateBranch, true) {
		branchName := c.generateBranchName(c.workUnit)
		if validationPattern := effectiveSettings.Git.BranchValidationPattern; validationPattern != "" {
			if err := validateBranchName(branchName, validationPattern); err != nil {
				return fmt.Errorf("branch name validation: %w", err)
			}
		}

		// Resolve local base branch to use as start point for new branches/worktrees.
		baseBranch, baseErr := c.getBaseBranch(ctx)
		if baseErr != nil {
			slog.Warn("could not detect base branch, new branches will fork from HEAD", "error", baseErr)
		}

		useWorktree := settings.BoolValue(effectiveSettings.Workflow.UseWorktreeIsolation, true)
		if useWorktree {
			// Create isolated git worktree from the local base branch.
			wtBasePath := filepath.Join(filepath.Dir(c.git.Path()), ".kvelmo-worktrees")
			wt, wtErr := c.git.CreateTaskWorktree(ctx, c.workUnit.ID, wtBasePath, baseBranch)
			if wtErr != nil {
				slog.Warn("worktree isolation failed, falling back to branch", "error", wtErr)
				useWorktree = false
			} else {
				c.workUnit.Branch = wt.Branch
				c.workUnit.WorktreePath = wt.Path
				if baseBranch != "" {
					c.logVerbosef("Created isolated worktree: %s (branch: %s, base: %s)", wt.Path, wt.Branch, baseBranch)
				} else {
					c.logVerbosef("Created isolated worktree: %s (branch: %s)", wt.Path, wt.Branch)
				}
			}
		}

		// Provision worktree with config files and dependency symlinks.
		if useWorktree && c.workUnit.WorktreePath != "" {
			c.provisionWorktree(ctx, effectiveSettings, c.git.Path(), c.workUnit.WorktreePath)
		}

		// Fallback: just create/switch branch on the main repo
		if !useWorktree {
			if c.git.BranchExists(ctx, branchName) {
				if err := c.git.SwitchBranch(ctx, branchName); err != nil {
					c.logVerbosef("Warning: could not switch to branch %s: %v", branchName, err)
				} else {
					c.workUnit.Branch = branchName
					c.logVerbosef("Switched to existing branch: %s", branchName)
				}
			} else {
				if err := c.git.CreateBranch(ctx, branchName, baseBranch); err != nil {
					c.logVerbosef("Warning: could not create branch: %v", err)
				} else {
					c.workUnit.Branch = branchName
					if baseBranch != "" {
						c.logVerbosef("Created branch: %s (base: %s)", branchName, baseBranch)
					} else {
						c.logVerbosef("Created branch: %s", branchName)
					}
				}
			}
		}
	}

	// Set work unit in machine (needed for guard validation) and dispatch start event
	c.machine.SetWorkUnit(c.workUnit)
	if err := c.machine.Dispatch(ctx, EventStart); err != nil {
		return fmt.Errorf("dispatch start: %w", err)
	}

	c.populateStandardVars()
	c.persistState()

	c.emit(ConductorEvent{
		Type:    "task_started",
		State:   c.machine.State(),
		Message: "Task started: " + c.workUnit.Title,
	})

	return nil
}

// provisionWorktree copies config files and creates dependency symlinks into
// the newly created worktree. Settings overrides are merged with auto-detected
// defaults. The result is emitted as a "worktree_provisioned" event.
func (c *Conductor) provisionWorktree(ctx context.Context, cfg *settings.Settings, srcDir, worktreeDir string) {
	if !settings.BoolValue(cfg.Git.Provision.Enabled, true) {
		return
	}

	defaults := provision.DefaultOptions(srcDir)
	userOpts := provision.Options{
		CopyPatterns:    cfg.Git.Provision.CopyPatterns,
		SymlinkPatterns: cfg.Git.Provision.SymlinkPatterns,
		SetupCommands:   cfg.Git.Provision.SetupCommands,
	}
	merged := provision.MergeOptions(defaults, userOpts)

	result, err := provision.Provision(ctx, srcDir, worktreeDir, merged)
	if err != nil {
		slog.Warn("worktree provisioning failed", "error", err)
		c.emit(ConductorEvent{
			Type:    "warning",
			Message: fmt.Sprintf("Worktree provisioning partially failed: %v", err),
		})

		return
	}

	if result.Empty() {
		return
	}

	data, jsonErr := json.Marshal(result)
	if jsonErr != nil {
		slog.Warn("failed to marshal provision result", "error", jsonErr)
	}
	c.logVerbosef("Provisioned worktree: %d files copied, %d symlinks created, %d commands run",
		len(result.FilesCopied), len(result.SymlinksCreated), len(result.CommandsRun))

	c.emit(ConductorEvent{
		Type:    "worktree_provisioned",
		Message: fmt.Sprintf("Worktree provisioned: %d files, %d symlinks", len(result.FilesCopied), len(result.SymlinksCreated)),
		Data:    data,
	})
}
