package provider

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
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

// UpdateStatus updates the state of a GitHub issue.
func (p *GitHubProvider) UpdateStatus(ctx context.Context, id string, status string) error {
	owner, repo, number, err := parseGitHubIDFull(id)
	if err != nil {
		return err
	}

	// Map status to GitHub state
	var state string
	switch status {
	case stateOpen, "pending", "in_progress":
		state = stateOpen
	case stateClosed, "done", "completed":
		state = stateClosed
	default:
		return fmt.Errorf("unsupported status: %s", status)
	}

	issueRequest := &github.IssueRequest{
		State: &state,
	}

	_, _, err = p.client.Issues.Edit(ctx, owner, repo, number, issueRequest)
	if err != nil {
		return fmt.Errorf("update issue state: %w", err)
	}

	return nil
}

// CreatePR creates a pull request on GitHub.
func (p *GitHubProvider) CreatePR(ctx context.Context, opts PROptions) (*PRResult, error) {
	// Extract owner/repo from task ID or head branch.
	// Head may be in format "owner/repo:branch" or just "branch".
	parts := strings.SplitN(opts.Head, ":", 2)
	var repoPath, head string
	if len(parts) == 2 {
		repoPath = parts[0]
		head = parts[1]
	} else {
		// Derive repo from task ID.
		if opts.TaskID != "" {
			repoParts := strings.SplitN(opts.TaskID, "#", 2)
			if len(repoParts) >= 1 {
				repoPath = repoParts[0]
			}
		}
		head = opts.Head
	}

	if repoPath == "" {
		return nil, errors.New("cannot determine repository from options")
	}

	repoParts := strings.SplitN(repoPath, "/", 2)
	if len(repoParts) != 2 {
		return nil, fmt.Errorf("invalid repository path: %s", repoPath)
	}
	owner, repo := repoParts[0], repoParts[1]

	base := opts.Base
	if base == "" {
		// Detect the repository's default branch
		repoInfo, _, err := p.client.Repositories.Get(ctx, owner, repo)
		if err != nil {
			slog.Warn("failed to detect default branch, using 'main'", "error", err, "owner", owner, "repo", repo)
			base = "main" // Fallback
		} else {
			base = repoInfo.GetDefaultBranch()
		}
	}

	// Build PR body with task link.
	body := opts.Body
	if opts.TaskURL != "" {
		body = fmt.Sprintf("%s\n\n---\nRelated: %s", body, opts.TaskURL)
	}

	// Create the PR
	newPR := &github.NewPullRequest{
		Title: &opts.Title,
		Body:  &body,
		Head:  &head,
		Base:  &base,
		Draft: &opts.Draft,
	}

	pr, _, err := p.client.PullRequests.Create(ctx, owner, repo, newPR)
	if err != nil {
		return nil, fmt.Errorf("create pull request: %w", err)
	}

	state := pr.GetState()
	if pr.GetDraft() {
		state = stateDraft
	}

	// Request reviewers if specified (best-effort).
	if len(opts.Reviewers) > 0 {
		reviewers := github.ReviewersRequest{
			Reviewers: opts.Reviewers,
		}
		_, _, _ = p.client.PullRequests.RequestReviewers(ctx, owner, repo, pr.GetNumber(), reviewers)
	}

	// Add labels if specified (best-effort).
	if len(opts.Labels) > 0 {
		_, _, _ = p.client.Issues.AddLabelsToIssue(ctx, owner, repo, pr.GetNumber(), opts.Labels)
	}

	return &PRResult{
		ID:     fmt.Sprintf("%s/%s#%d", owner, repo, pr.GetNumber()),
		Number: pr.GetNumber(),
		URL:    pr.GetHTMLURL(),
		State:  state,
	}, nil
}

// AddComment adds a comment to an issue or PR.
func (p *GitHubProvider) AddComment(ctx context.Context, id string, comment string) error {
	owner, repo, number, err := parseGitHubIDFull(id)
	if err != nil {
		return err
	}

	issueComment := &github.IssueComment{
		Body: &comment,
	}

	_, _, err = p.client.Issues.CreateComment(ctx, owner, repo, number, issueComment)
	if err != nil {
		return fmt.Errorf("create comment: %w", err)
	}

	return nil
}

// GetPRStatus returns the status of a pull request.
// The taskID should be in format "owner/repo#number".
func (p *GitHubProvider) GetPRStatus(ctx context.Context, taskID string) (*PRStatus, error) {
	owner, repo, number, err := parseGitHubIDFull(taskID)
	if err != nil {
		return nil, err
	}

	// Try to get as PR first
	pr, _, err := p.client.PullRequests.Get(ctx, owner, repo, number)
	if err == nil {
		return &PRStatus{
			Number: pr.GetNumber(),
			State:  pr.GetState(),
			Merged: pr.GetMerged(),
			URL:    pr.GetHTMLURL(),
		}, nil
	}

	// If not a PR, check if it's an issue (issues don't have merged status)
	issue, _, err := p.client.Issues.Get(ctx, owner, repo, number)
	if err == nil {
		return &PRStatus{
			Number: issue.GetNumber(),
			State:  issue.GetState(),
			Merged: false,
			URL:    issue.GetHTMLURL(),
		}, nil
	}

	return nil, fmt.Errorf("could not find issue or PR: %s", taskID)
}

// ApprovePR approves a pull request with an optional comment.
// The taskID should be in format "owner/repo#number".
func (p *GitHubProvider) ApprovePR(ctx context.Context, taskID string, comment string) error {
	owner, repo, number, err := parseGitHubIDFull(taskID)
	if err != nil {
		return err
	}

	event := "APPROVE"
	review := &github.PullRequestReviewRequest{
		Event: &event,
	}

	// Only set body if non-empty
	if comment != "" {
		review.Body = &comment
	}

	_, _, err = p.client.PullRequests.CreateReview(ctx, owner, repo, number, review)
	if err != nil {
		return fmt.Errorf("approve pull request: %w", err)
	}

	return nil
}

// MergePR merges a pull request using the specified method.
// The taskID should be in format "owner/repo#number".
// Method should be one of: "merge", "squash", "rebase" (default: "rebase").
func (p *GitHubProvider) MergePR(ctx context.Context, taskID string, method string) error {
	owner, repo, number, err := parseGitHubIDFull(taskID)
	if err != nil {
		return err
	}

	// Default to rebase
	if method == "" {
		method = "rebase"
	}

	options := &github.PullRequestOptions{
		MergeMethod: method,
	}

	_, _, err = p.client.PullRequests.Merge(ctx, owner, repo, number, "", options)
	if err != nil {
		return fmt.Errorf("merge pull request: %w", err)
	}

	return nil
}

// GetBranchProtection returns GitHub branch protection rules.
func (p *GitHubProvider) GetBranchProtection(ctx context.Context, owner, repo, branch string) (*BranchProtection, error) {
	protection, resp, err := p.client.Repositories.GetBranchProtection(ctx, owner, repo, branch)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return new(BranchProtection), nil // No protection rules
		}

		return nil, fmt.Errorf("get branch protection: %w", err)
	}

	bp := &BranchProtection{}

	if protection.RequiredStatusChecks != nil && protection.RequiredStatusChecks.Checks != nil {
		for _, check := range *protection.RequiredStatusChecks.Checks {
			bp.RequiredChecks = append(bp.RequiredChecks, check.Context)
		}
	}

	if protection.RequiredPullRequestReviews != nil {
		bp.RequireReviews = true
		bp.MinReviewers = protection.RequiredPullRequestReviews.RequiredApprovingReviewCount
		bp.DismissStaleReviews = protection.RequiredPullRequestReviews.DismissStaleReviews
	}

	if protection.EnforceAdmins != nil {
		bp.EnforceAdmins = protection.EnforceAdmins.Enabled
	}

	return bp, nil
}

// ListTasks lists issues for a GitHub repository.
// ListOptions.Team is used as "owner/repo" to identify the repository.
// ListOptions.Status maps to GitHub issue state ("open", "closed", "all").
func (p *GitHubProvider) ListTasks(ctx context.Context, opts ListOptions) (*ListResult, error) {
	if opts.Team == "" {
		return nil, errors.New("team must be set to owner/repo for GitHub")
	}

	parts := strings.SplitN(opts.Team, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repository format %q, expected owner/repo", opts.Team)
	}
	owner, repo := parts[0], parts[1]

	limit := opts.Limit
	if limit == 0 {
		limit = 30
	}

	state := opts.Status
	if state == "" {
		state = stateOpen
	}

	listOpts := &github.IssueListByRepoOptions{
		State: state,
		ListOptions: github.ListOptions{
			PerPage: limit,
		},
	}

	// Parse cursor as page number.
	if opts.Cursor != "" {
		page, err := strconv.Atoi(opts.Cursor)
		if err != nil {
			return nil, fmt.Errorf("invalid cursor %q: %w", opts.Cursor, err)
		}
		listOpts.Page = page
	}

	issues, resp, err := p.client.Issues.ListByRepo(ctx, owner, repo, listOpts)
	if err != nil {
		return nil, fmt.Errorf("list issues: %w", err)
	}

	tasks := make([]*Task, 0, len(issues))
	for _, issue := range issues {
		// Skip pull requests (GitHub API returns PRs mixed in with issues).
		if issue.IsPullRequest() {
			continue
		}
		tasks = append(tasks, p.issueToTask(owner, repo, issue))
	}

	result := &ListResult{
		Tasks: tasks,
	}

	if resp.NextPage > 0 {
		result.HasMore = true
		result.NextCursor = strconv.Itoa(resp.NextPage)
	}

	return result, nil
}

// FetchComments returns all comments on a GitHub issue or PR.
func (p *GitHubProvider) FetchComments(ctx context.Context, id string) ([]Comment, error) {
	owner, repo, number, err := parseGitHubIDFull(id)
	if err != nil {
		return nil, err
	}

	var allGhComments []*github.IssueComment
	opts := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for {
		ghComments, resp, err := p.client.Issues.ListComments(ctx, owner, repo, number, opts)
		if err != nil {
			return nil, fmt.Errorf("list comments: %w", err)
		}
		allGhComments = append(allGhComments, ghComments...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	comments := make([]Comment, 0, len(allGhComments))
	for _, c := range allGhComments {
		author := ""
		if c.User != nil {
			author = c.User.GetLogin()
		}
		createdAt := ""
		if c.CreatedAt != nil {
			createdAt = c.CreatedAt.GetTime().Format("2006-01-02T15:04:05Z")
		}
		comments = append(comments, Comment{
			ID:        strconv.FormatInt(c.GetID(), 10),
			Body:      c.GetBody(),
			Author:    author,
			CreatedAt: createdAt,
		})
	}

	return comments, nil
}

// AddLabels adds labels to a GitHub issue or PR.
func (p *GitHubProvider) AddLabels(ctx context.Context, id string, labels []string) error {
	owner, repo, number, err := parseGitHubIDFull(id)
	if err != nil {
		return err
	}

	_, _, err = p.client.Issues.AddLabelsToIssue(ctx, owner, repo, number, labels)
	if err != nil {
		return fmt.Errorf("add labels: %w", err)
	}

	return nil
}

// CreateTask creates a new GitHub issue.
func (p *GitHubProvider) CreateTask(ctx context.Context, opts CreateTaskOptions) (*Task, error) {
	if opts.Team == "" {
		return nil, errors.New("team must be set to owner/repo for GitHub")
	}

	parts := strings.SplitN(opts.Team, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repository format %q, expected owner/repo", opts.Team)
	}
	owner, repo := parts[0], parts[1]

	req := &github.IssueRequest{
		Title: &opts.Title,
		Body:  &opts.Description,
	}
	if len(opts.Labels) > 0 {
		req.Labels = &opts.Labels
	}

	issue, _, err := p.client.Issues.Create(ctx, owner, repo, req)
	if err != nil {
		return nil, fmt.Errorf("create issue: %w", err)
	}

	return p.issueToTask(owner, repo, issue), nil
}

// DeleteBranch deletes a branch from a GitHub repository.
// The id should be in "owner/repo" format, and branch is the branch name.
func (p *GitHubProvider) DeleteBranch(ctx context.Context, repo string, branch string) error {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid repository format %q, expected owner/repo", repo)
	}

	_, err := p.client.Git.DeleteRef(ctx, parts[0], parts[1], "refs/heads/"+branch)
	if err != nil {
		return fmt.Errorf("delete branch: %w", err)
	}

	return nil
}

// RemoveLabels removes labels from a GitHub issue or PR.
func (p *GitHubProvider) RemoveLabels(ctx context.Context, id string, labels []string) error {
	owner, repo, number, err := parseGitHubIDFull(id)
	if err != nil {
		return err
	}

	for _, label := range labels {
		resp, err := p.client.Issues.RemoveLabelForIssue(ctx, owner, repo, number, label)
		if err != nil {
			// Ignore 404 — the label may not be on the issue.
			if resp != nil && resp.StatusCode == http.StatusNotFound {
				continue
			}

			return fmt.Errorf("remove label %q: %w", label, err)
		}
	}

	return nil
}
