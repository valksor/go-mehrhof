package conductor

import (
	"context"
	"fmt"
	"strings"

	"github.com/valksor/go-mehrhof/internal/events"
	"github.com/valksor/go-mehrhof/internal/provider"
	"github.com/valksor/go-mehrhof/internal/storage"
)

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

	// Use strings.Builder for efficient concatenation
	var sb strings.Builder
	for _, p := range parts {
		sb.WriteString(p)
	}
	return sb.String()
}
