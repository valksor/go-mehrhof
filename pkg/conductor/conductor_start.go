package conductor

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/valksor/kvelmo/pkg/provider"
	"github.com/valksor/kvelmo/pkg/settings"
)

// getStatusFromLabels extracts a status value from labels with "status:" prefix.
// Returns empty string if no status label is found.
func getStatusFromLabels(labels []string) string {
	for _, label := range labels {
		if strings.HasPrefix(label, "status:") {
			return strings.TrimPrefix(label, "status:")
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
	if providerName == "wrike" {
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

		useWorktree := settings.BoolValue(effectiveSettings.Workflow.UseWorktreeIsolation, true)
		if useWorktree {
			// Create isolated git worktree so the main branch stays clean.
			wtBasePath := filepath.Join(c.git.Path(), ".kvelmo", "worktrees")
			wt, wtErr := c.git.CreateTaskWorktree(ctx, c.workUnit.ID, wtBasePath)
			if wtErr != nil {
				slog.Warn("worktree isolation failed, falling back to branch", "error", wtErr)
				useWorktree = false
			} else {
				c.workUnit.Branch = wt.Branch
				c.workUnit.WorktreePath = wt.Path
				c.logVerbosef("Created isolated worktree: %s (branch: %s)", wt.Path, wt.Branch)
			}
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
				if err := c.git.CreateBranch(ctx, branchName); err != nil {
					c.logVerbosef("Warning: could not create branch: %v", err)
				} else {
					c.workUnit.Branch = branchName
					c.logVerbosef("Created branch: %s", branchName)
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
