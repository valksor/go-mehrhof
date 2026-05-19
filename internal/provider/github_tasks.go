package provider

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v67/github"
)

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
	case stateClosed, "done", statusCompleted:
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
//
// Retries up to 3 times with exponential backoff when GitHub returns HTTP 405
// "Base branch was modified" — a well-known transient response where the API's
// merge-readiness view lags behind actual branch state.
func (p *GitHubProvider) MergePR(ctx context.Context, taskID string, method string) error {
	owner, repo, number, err := parseGitHubIDFull(taskID)
	if err != nil {
		return err
	}

	options := &github.PullRequestOptions{
		MergeMethod: cmp.Or(method, "rebase"),
	}

	const maxAttempts = 3
	backoff := 2 * time.Second
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		_, _, err = p.client.PullRequests.Merge(ctx, owner, repo, number, "", options)
		if err == nil {
			return nil
		}
		if !isTransientMergeError(err) || attempt == maxAttempts {
			return fmt.Errorf("merge pull request: %w", err)
		}
		slog.Warn("merge PR transient failure, retrying",
			"attempt", attempt, "max", maxAttempts, "backoff", backoff, "error", err)
		select {
		case <-ctx.Done():
			return fmt.Errorf("merge pull request: %w", ctx.Err())
		case <-time.After(backoff):
		}
		backoff *= 2
	}

	return fmt.Errorf("merge pull request: %w", err)
}

// isTransientMergeError reports whether a GitHub merge error is transient and
// worth retrying. GitHub returns HTTP 405 with "Base branch was modified" when
// its internal merge-readiness state is stale, even though nothing was actually
// modified. Retrying after a short delay typically succeeds.
func isTransientMergeError(err error) bool {
	if err == nil {
		return false
	}
	var ghErr *github.ErrorResponse
	if errors.As(err, &ghErr) && ghErr.Response != nil {
		if ghErr.Response.StatusCode == http.StatusMethodNotAllowed {
			return strings.Contains(ghErr.Message, "Base branch was modified")
		}
	}

	return false
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

	listOpts := &github.IssueListByRepoOptions{
		State: cmp.Or(opts.Status, stateOpen),
		ListOptions: github.ListOptions{
			PerPage: cmp.Or(opts.Limit, 30),
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
