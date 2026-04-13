package provider

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/go-github/v67/github"
)

// GitHub issue/PR state constants.
const (
	stateOpen   = "open"
	stateClosed = "closed"
	stateDraft  = "draft"
)

// GitHubProvider implements the Provider interface for GitHub issues and PRs.
type GitHubProvider struct {
	client *github.Client
	host   string
}

// NewGitHubProvider creates a new GitHub provider.
// Token should come from Settings (settings.Providers.GitHub.Token).
func NewGitHubProvider(token string) *GitHubProvider {
	return &GitHubProvider{
		client: newGitHubClient(token, ""),
	}
}

// NewGitHubProviderWithHost creates a new GitHub provider for GitHub Enterprise.
// Token should come from Settings (settings.Providers.GitHub.Token).
func NewGitHubProviderWithHost(token, host string) *GitHubProvider {
	return &GitHubProvider{
		client: newGitHubClient(token, host),
		host:   host,
	}
}

func (p *GitHubProvider) Name() string {
	return NameGitHub
}

// FetchTask fetches an issue or PR from GitHub by ID (owner/repo#number).
func (p *GitHubProvider) FetchTask(ctx context.Context, id string) (*Task, error) {
	owner, repo, number, err := parseGitHubIDFull(id)
	if err != nil {
		return nil, err
	}

	// Try issues first (PRs also appear as issues but with limited data)
	issue, _, err := p.client.Issues.Get(ctx, owner, repo, number)
	if err == nil {
		// Check if this is actually a PR (PRs appear as issues but need full PR data)
		if issue.IsPullRequest() {
			pr, _, prErr := p.client.PullRequests.Get(ctx, owner, repo, number)
			if prErr == nil {
				return p.prToTask(owner, repo, pr), nil
			}
			// Fall through to issue if PR fetch fails
			slog.Debug("failed to fetch PR details, using issue data", "number", number, "error", prErr)
		}

		return p.issueToTask(owner, repo, issue), nil
	}

	// If not found as issue, try as PR directly
	pr, _, err := p.client.PullRequests.Get(ctx, owner, repo, number)
	if err == nil {
		return p.prToTask(owner, repo, pr), nil
	}

	return nil, fmt.Errorf("not found: %s", id)
}

// issueToTask converts a GitHub issue to a Task.
func (p *GitHubProvider) issueToTask(owner, repo string, issue *github.Issue) *Task {
	labels := make([]string, len(issue.Labels))
	for i, l := range issue.Labels {
		labels[i] = l.GetName()
	}

	task := &Task{
		ID:          fmt.Sprintf("%s/%s#%d", owner, repo, issue.GetNumber()),
		Title:       issue.GetTitle(),
		Description: issue.GetBody(),
		URL:         issue.GetHTMLURL(),
		Labels:      labels,
		Source:      NameGitHub,
	}

	// Inference
	task.Priority, task.Type, task.Slug = InferAll(task.Title, labels)

	// Subtasks
	task.Subtasks = ParseSubtasks(task.ID, task.Description)

	// Metadata (set before resolveDependencies so shorthand refs can use owner/repo)
	task.SetMetadata("github_state", issue.GetState())
	task.SetMetadata("github_owner", owner)
	task.SetMetadata("github_repo", repo)

	// Dependencies
	task.Dependencies = p.resolveDependencies(task)

	// Store assignees
	if len(issue.Assignees) > 0 {
		assigneeLogins := make([]string, len(issue.Assignees))
		for i, a := range issue.Assignees {
			assigneeLogins[i] = a.GetLogin()
		}
		task.SetMetadata("github_assignees", strings.Join(assigneeLogins, ","))
	}

	// Store milestone
	if issue.Milestone != nil && issue.Milestone.GetTitle() != "" {
		task.SetMetadata("github_milestone", issue.Milestone.GetTitle())
		task.SetMetadata("github_milestone_number", milestoneNumber(issue))
	}

	return task
}

// prToTask converts a GitHub pull request to a Task.
func (p *GitHubProvider) prToTask(owner, repo string, pr *github.PullRequest) *Task {
	labels := make([]string, len(pr.Labels))
	for i, l := range pr.Labels {
		labels[i] = l.GetName()
	}

	task := &Task{
		ID:          fmt.Sprintf("%s/%s#%d", owner, repo, pr.GetNumber()),
		Title:       pr.GetTitle(),
		Description: pr.GetBody(),
		URL:         pr.GetHTMLURL(),
		Labels:      labels,
		Source:      NameGitHub,
	}

	// Inference
	task.Priority, task.Type, task.Slug = InferAll(task.Title, labels)

	// Subtasks
	task.Subtasks = ParseSubtasks(task.ID, task.Description)

	// Metadata (set before resolveDependencies so shorthand refs can use owner/repo)
	state := pr.GetState()
	if pr.GetDraft() {
		state = stateDraft
	}
	task.SetMetadata("github_state", state)
	task.SetMetadata("github_owner", owner)
	task.SetMetadata("github_repo", repo)
	task.SetMetadata("github_is_pr", "true")

	// Dependencies
	task.Dependencies = p.resolveDependencies(task)

	// Store assignees
	if len(pr.Assignees) > 0 {
		assigneeLogins := make([]string, len(pr.Assignees))
		for i, a := range pr.Assignees {
			assigneeLogins[i] = a.GetLogin()
		}
		task.SetMetadata("github_assignees", strings.Join(assigneeLogins, ","))
	}

	// Store milestone
	if pr.Milestone != nil && pr.Milestone.GetTitle() != "" {
		task.SetMetadata("github_milestone", pr.Milestone.GetTitle())
		task.SetMetadata("github_milestone_number", milestoneNumberFromPR(pr))
	}

	return task
}

// resolveDependencies parses dependency references and creates stub Task objects.
// Full resolution would require additional API calls; this provides the references.
func (p *GitHubProvider) resolveDependencies(task *Task) []*Task {
	refs := ParseDependencies(task.Description)
	if len(refs) == 0 {
		return nil
	}

	deps := make([]*Task, 0, len(refs))
	for _, ref := range refs {
		// Handle both full (owner/repo#num) and shorthand (#num) refs
		depID := ref
		if strings.HasPrefix(ref, "#") {
			// Shorthand ref - prepend owner/repo from task
			owner := task.Metadata("github_owner")
			repo := task.Metadata("github_repo")
			if owner != "" && repo != "" {
				depID = fmt.Sprintf("%s/%s%s", owner, repo, ref)
			}
		}
		deps = append(deps, &Task{
			ID:     depID,
			Source: NameGitHub,
		})
	}

	return deps
}
