package provider

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

// GitLabProvider implements the Provider interface for GitLab issues and MRs.
type GitLabProvider struct {
	client *gitlab.Client
	host   string
}

// NewGitLabProvider creates a new GitLab provider.
// Token should come from Settings (settings.Providers.GitLab.Token).
func NewGitLabProvider(token string) (*GitLabProvider, error) {
	client, err := newGitLabClient(token, "")
	if err != nil {
		return nil, fmt.Errorf("create gitlab client: %w", err)
	}

	return &GitLabProvider{
		client: client,
	}, nil
}

// NewGitLabProviderWithHost creates a new GitLab provider for a custom GitLab instance.
// Token should come from Settings (settings.Providers.GitLab.Token).
func NewGitLabProviderWithHost(token, host string) (*GitLabProvider, error) {
	client, err := newGitLabClient(token, host)
	if err != nil {
		return nil, fmt.Errorf("create gitlab client: %w", err)
	}

	return &GitLabProvider{
		client: client,
		host:   host,
	}, nil
}

func (p *GitLabProvider) Name() string {
	return "gitlab"
}

// FetchTask fetches an issue or MR from GitLab by ID (project#number or project!number).
func (p *GitLabProvider) FetchTask(ctx context.Context, id string) (*Task, error) {
	project, number, isMR, err := parseGitLabID(id)
	if err != nil {
		return nil, err
	}

	if isMR {
		return p.fetchMR(ctx, project, number)
	}

	return p.fetchIssue(ctx, project, number)
}

// fetchIssue fetches a GitLab issue and converts it to a Task.
func (p *GitLabProvider) fetchIssue(ctx context.Context, project string, number int) (*Task, error) {
	issue, _, err := p.client.Issues.GetIssue(project, int64(number), gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("get issue: %w", err)
	}

	return p.issueToTask(project, issue), nil
}

// fetchMR fetches a GitLab merge request and converts it to a Task.
func (p *GitLabProvider) fetchMR(ctx context.Context, project string, number int) (*Task, error) {
	mr, _, err := p.client.MergeRequests.GetMergeRequest(project, int64(number), nil, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("get merge request: %w", err)
	}

	return p.mrToTask(project, mr), nil
}

// issueToTask converts a GitLab issue to a Task.
func (p *GitLabProvider) issueToTask(project string, issue *gitlab.Issue) *Task {
	labels := make([]string, len(issue.Labels))
	copy(labels, issue.Labels)

	task := &Task{
		ID:          fmt.Sprintf("%s#%d", project, issue.IID),
		Title:       issue.Title,
		Description: issue.Description,
		URL:         issue.WebURL,
		Labels:      labels,
		Source:      "gitlab",
	}

	// Inference
	task.Priority, task.Type, task.Slug = InferAll(task.Title, labels)

	// Subtasks
	task.Subtasks = ParseSubtasks(task.ID, task.Description)

	// Metadata (set before resolveDependencies so shorthand refs can use project)
	task.SetMetadata("gitlab_state", issue.State)
	task.SetMetadata("gitlab_project", project)

	// Dependencies
	task.Dependencies = p.resolveDependencies(task)

	// Store assignees
	if len(issue.Assignees) > 0 {
		assigneeLogins := make([]string, 0, len(issue.Assignees))
		for _, a := range issue.Assignees {
			if a.Username != "" {
				assigneeLogins = append(assigneeLogins, a.Username)
			}
		}
		if len(assigneeLogins) > 0 {
			task.SetMetadata("gitlab_assignees", strings.Join(assigneeLogins, ","))
		}
	}

	// Store milestone
	if issue.Milestone != nil && issue.Milestone.Title != "" {
		task.SetMetadata("gitlab_milestone", issue.Milestone.Title)
		task.SetMetadata("gitlab_milestone_id", strconv.FormatInt(issue.Milestone.ID, 10))
	}

	return task
}

// mrToTask converts a GitLab merge request to a Task.
func (p *GitLabProvider) mrToTask(project string, mr *gitlab.MergeRequest) *Task {
	labels := make([]string, len(mr.Labels))
	copy(labels, mr.Labels)

	task := &Task{
		ID:          fmt.Sprintf("%s!%d", project, mr.IID),
		Title:       mr.Title,
		Description: mr.Description,
		URL:         mr.WebURL,
		Labels:      labels,
		Source:      "gitlab",
	}

	// Inference
	task.Priority, task.Type, task.Slug = InferAll(task.Title, labels)

	// Subtasks
	task.Subtasks = ParseSubtasks(task.ID, task.Description)

	// Metadata (set before resolveDependencies so shorthand refs can use project)
	state := mr.State
	if mr.Draft {
		state = "draft"
	}
	task.SetMetadata("gitlab_state", state)
	task.SetMetadata("gitlab_project", project)
	task.SetMetadata("gitlab_is_mr", "true")

	// Dependencies
	task.Dependencies = p.resolveDependencies(task)

	// Store assignees
	if len(mr.Assignees) > 0 {
		assigneeLogins := make([]string, 0, len(mr.Assignees))
		for _, a := range mr.Assignees {
			if a.Username != "" {
				assigneeLogins = append(assigneeLogins, a.Username)
			}
		}
		if len(assigneeLogins) > 0 {
			task.SetMetadata("gitlab_assignees", strings.Join(assigneeLogins, ","))
		}
	}

	// Store milestone
	if mr.Milestone != nil && mr.Milestone.Title != "" {
		task.SetMetadata("gitlab_milestone", mr.Milestone.Title)
		task.SetMetadata("gitlab_milestone_id", strconv.FormatInt(mr.Milestone.ID, 10))
	}

	return task
}

// resolveDependencies parses dependency references and creates stub Task objects.
func (p *GitLabProvider) resolveDependencies(task *Task) []*Task {
	refs := ParseDependencies(task.Description)
	if len(refs) == 0 {
		return nil
	}

	project := task.Metadata("gitlab_project")
	deps := make([]*Task, 0, len(refs))
	for _, ref := range refs {
		depID := ref
		// Handle shorthand refs (#num or !num)
		if strings.HasPrefix(ref, "#") || strings.HasPrefix(ref, "!") {
			if project != "" {
				depID = project + ref
			}
		}
		deps = append(deps, &Task{
			ID:     depID,
			Source: "gitlab",
		})
	}

	return deps
}

// UpdateStatus updates the state of a GitLab issue.
func (p *GitLabProvider) UpdateStatus(ctx context.Context, id string, status string) error {
	project, number, isMR, err := parseGitLabID(id)
	if err != nil {
		return err
	}

	// Map status to GitLab state event
	var stateEvent string
	switch status {
	case stateOpen, "pending", "in_progress":
		stateEvent = "reopen"
	case stateClosed, "done", "completed":
		stateEvent = "close"
	default:
		return fmt.Errorf("unsupported status: %s", status)
	}

	if isMR {
		opts := &gitlab.UpdateMergeRequestOptions{
			StateEvent: new(stateEvent),
		}
		_, _, err = p.client.MergeRequests.UpdateMergeRequest(project, int64(number), opts, gitlab.WithContext(ctx))
	} else {
		opts := &gitlab.UpdateIssueOptions{
			StateEvent: new(stateEvent),
		}
		_, _, err = p.client.Issues.UpdateIssue(project, int64(number), opts, gitlab.WithContext(ctx))
	}

	if err != nil {
		return fmt.Errorf("update state: %w", err)
	}

	return nil
}

// CreatePR creates a merge request on GitLab.
func (p *GitLabProvider) CreatePR(ctx context.Context, opts PROptions) (*PRResult, error) {
	// Extract project from task ID or head branch.
	var project, sourceBranch string
	parts := strings.SplitN(opts.Head, ":", 2)
	if len(parts) == 2 {
		project = parts[0]
		sourceBranch = parts[1]
	} else {
		// Get project from task ID.
		if opts.TaskID != "" {
			projectParts := strings.SplitN(opts.TaskID, "#", 2)
			if len(projectParts) < 2 {
				projectParts = strings.SplitN(opts.TaskID, "!", 2)
			}
			if len(projectParts) >= 1 {
				project = projectParts[0]
			}
		}
		sourceBranch = opts.Head
	}

	if project == "" {
		return nil, errors.New("cannot determine project from options")
	}

	targetBranch := opts.Base
	if targetBranch == "" {
		// Detect the project's default branch.
		defaultBranch, err := p.getDefaultBranch(ctx, project)
		if err != nil {
			// Fall back to "main" if detection fails.
			defaultBranch = "main"
		}
		targetBranch = defaultBranch
	}

	// Build MR description with task link.
	description := opts.Body
	if opts.TaskURL != "" {
		description = fmt.Sprintf("%s\n\n---\nRelated: %s", description, opts.TaskURL)
	}

	// Build the MR request.
	// Set draft title upfront to avoid race window where MR exists non-draft.
	title := opts.Title
	if opts.Draft {
		title = "Draft: " + title
	}

	mrOpts := &gitlab.CreateMergeRequestOptions{
		Title:              new(title),
		Description:        new(description),
		SourceBranch:       new(sourceBranch),
		TargetBranch:       new(targetBranch),
		RemoveSourceBranch: new(true), // Best practice: clean up merged branches
	}

	mr, _, err := p.client.MergeRequests.CreateMergeRequest(project, mrOpts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("create merge request: %w", err)
	}

	state := mr.State
	if mr.Draft {
		state = "draft"
	}

	return &PRResult{
		ID:     fmt.Sprintf("%s!%d", project, mr.IID),
		Number: int(mr.IID),
		URL:    mr.WebURL,
		State:  state,
	}, nil
}

// AddComment adds a comment to an issue or merge request.
func (p *GitLabProvider) AddComment(ctx context.Context, id string, comment string) error {
	project, number, isMR, err := parseGitLabID(id)
	if err != nil {
		return err
	}

	if isMR {
		noteOpts := &gitlab.CreateMergeRequestNoteOptions{
			Body: new(comment),
		}
		_, _, err = p.client.Notes.CreateMergeRequestNote(project, int64(number), noteOpts, gitlab.WithContext(ctx))
	} else {
		noteOpts := &gitlab.CreateIssueNoteOptions{
			Body: new(comment),
		}
		_, _, err = p.client.Notes.CreateIssueNote(project, int64(number), noteOpts, gitlab.WithContext(ctx))
	}

	if err != nil {
		return fmt.Errorf("create note: %w", err)
	}

	return nil
}

// --- internal helpers ---

// getDefaultBranch fetches the default branch for a GitLab project.
func (p *GitLabProvider) getDefaultBranch(ctx context.Context, project string) (string, error) {
	proj, _, err := p.client.Projects.GetProject(project, nil, gitlab.WithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("get project: %w", err)
	}

	return proj.DefaultBranch, nil
}
