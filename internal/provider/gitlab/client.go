package gitlab

import (
	"context"
	"fmt"
	"os"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

// ptr is a helper to create a pointer to a value.
func ptr[T any](v T) *T {
	return &v
}

// Client wraps the GitLab API client.
type Client struct {
	gl          *gitlab.Client
	projectID   int64  // Numeric project ID (cached)
	projectPath string // Project path (e.g., "group/project")
	host        string // GitLab host (e.g., "gitlab.com" or custom)
}

// NewClient creates a new GitLab API client.
func NewClient(token, host, projectPath string, projectID int64) *Client {
	var options []gitlab.ClientOptionFunc

	// For self-hosted GitLab, set the base URL
	if host != "" && host != "https://gitlab.com" && host != "gitlab.com" {
		baseURL := strings.TrimSuffix(host, "/") + "/api/v4"
		options = append(options, gitlab.WithBaseURL(baseURL))
	}

	client, err := gitlab.NewClient(token, options...)
	if err != nil {
		panic(fmt.Sprintf("failed to create GitLab client: %v", err))
	}

	return &Client{
		gl:          client,
		projectPath: projectPath,
		projectID:   projectID,
		host:        host,
	}
}

// ResolveToken finds the GitLab token from multiple sources
// Priority order:
//  1. MEHR_GITLAB_TOKEN env var
//  2. GITLAB_TOKEN env var
//  3. configToken (from config.yaml)
func ResolveToken(configToken string) (string, error) {
	// 1. Check MEHR_GITLAB_TOKEN
	if token := os.Getenv("MEHR_GITLAB_TOKEN"); token != "" {
		return token, nil
	}

	// 2. Check GITLAB_TOKEN
	if token := os.Getenv("GITLAB_TOKEN"); token != "" {
		return token, nil
	}

	// 3. Check config token
	if configToken != "" {
		return configToken, nil
	}

	return "", ErrNoToken
}

// getProjectID retrieves the numeric project ID from the project path.
func (c *Client) getProjectID(ctx context.Context) (int64, error) {
	if c.projectID > 0 {
		return c.projectID, nil
	}

	if c.projectPath == "" {
		return 0, ErrProjectNotConfigured
	}

	// Get project by path
	project, _, err := c.gl.Projects.GetProject(c.projectPath, nil, gitlab.WithContext(ctx))
	if err != nil {
		return 0, wrapAPIError(err)
	}

	c.projectID = project.ID

	return c.projectID, nil
}

// GetIssue fetches an issue by IID (internal issue number).
func (c *Client) GetIssue(ctx context.Context, iid int64) (*gitlab.Issue, error) {
	pid, err := c.getProjectID(ctx)
	if err != nil {
		return nil, err
	}

	issue, _, err := c.gl.Issues.GetIssue(pid, iid, gitlab.WithContext(ctx))
	if err != nil {
		return nil, wrapAPIError(err)
	}

	return issue, nil
}

// GetIssueNotes fetches all notes (comments) on an issue.
func (c *Client) GetIssueNotes(ctx context.Context, iid int64) ([]*gitlab.Note, error) {
	pid, err := c.getProjectID(ctx)
	if err != nil {
		return nil, err
	}

	var allNotes []*gitlab.Note
	opts := &gitlab.ListIssueNotesOptions{}
	opts.Page = 1
	opts.PerPage = 100

	for {
		notes, resp, err := c.gl.Notes.ListIssueNotes(pid, iid, opts, gitlab.WithContext(ctx))
		if err != nil {
			return nil, wrapAPIError(err)
		}
		allNotes = append(allNotes, notes...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allNotes, nil
}

// AddNote adds a note (comment) to an issue.
func (c *Client) AddNote(ctx context.Context, iid int64, body string) (*gitlab.Note, error) {
	pid, err := c.getProjectID(ctx)
	if err != nil {
		return nil, err
	}

	note, _, err := c.gl.Notes.CreateIssueNote(pid, iid, &gitlab.CreateIssueNoteOptions{
		Body: ptr(body),
	}, gitlab.WithContext(ctx))
	if err != nil {
		return nil, wrapAPIError(err)
	}

	return note, nil
}

// UpdateIssue updates an issue.
func (c *Client) UpdateIssue(ctx context.Context, iid int64, opts *gitlab.UpdateIssueOptions) (*gitlab.Issue, error) {
	pid, err := c.getProjectID(ctx)
	if err != nil {
		return nil, err
	}

	issue, _, err := c.gl.Issues.UpdateIssue(pid, iid, opts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, wrapAPIError(err)
	}

	return issue, nil
}

// ListIssues lists issues with filters.
func (c *Client) ListIssues(ctx context.Context, opts *gitlab.ListProjectIssuesOptions) ([]*gitlab.Issue, error) {
	pid, err := c.getProjectID(ctx)
	if err != nil {
		return nil, err
	}

	// Set default pagination
	if opts.PerPage == 0 {
		opts.PerPage = 100
	}

	var allIssues []*gitlab.Issue
	for {
		issues, resp, err := c.gl.Issues.ListProjectIssues(pid, opts, gitlab.WithContext(ctx))
		if err != nil {
			return nil, wrapAPIError(err)
		}
		allIssues = append(allIssues, issues...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allIssues, nil
}

// CreateIssue creates a new issue.
func (c *Client) CreateIssue(ctx context.Context, opts *gitlab.CreateIssueOptions) (*gitlab.Issue, error) {
	pid, err := c.getProjectID(ctx)
	if err != nil {
		return nil, err
	}

	issue, _, err := c.gl.Issues.CreateIssue(pid, opts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, wrapAPIError(err)
	}

	return issue, nil
}

// SetLabels sets labels on an issue (by updating the issue).
func (c *Client) SetLabels(ctx context.Context, iid int64, labels []string) error {
	pid, err := c.getProjectID(ctx)
	if err != nil {
		return err
	}

	// Update labels requires setting the full label list
	labelOpts := gitlab.LabelOptions(labels)
	_, _, err = c.gl.Issues.UpdateIssue(pid, iid, &gitlab.UpdateIssueOptions{
		Labels: &labelOpts,
	}, gitlab.WithContext(ctx))

	return err
}

// AddLabels adds labels to an issue.
func (c *Client) AddLabels(ctx context.Context, iid int64, labels []string) error {
	pid, err := c.getProjectID(ctx)
	if err != nil {
		return err
	}

	// Get current issue to check existing labels
	issue, err := c.GetIssue(ctx, iid)
	if err != nil {
		return err
	}

	// Merge labels
	existingLabels := make(map[string]bool)
	for _, l := range issue.Labels {
		existingLabels[l] = true
	}

	for _, l := range labels {
		if !existingLabels[l] {
			issue.Labels = append(issue.Labels, l)
		}
	}

	// Update with new label set
	labelOpts := gitlab.LabelOptions(issue.Labels)
	_, _, err = c.gl.Issues.UpdateIssue(pid, iid, &gitlab.UpdateIssueOptions{
		Labels: &labelOpts,
	}, gitlab.WithContext(ctx))

	return err
}

// RemoveLabel removes a label from an issue.
func (c *Client) RemoveLabel(ctx context.Context, iid int64, label string) error {
	pid, err := c.getProjectID(ctx)
	if err != nil {
		return err
	}

	// Get current issue to check existing labels
	issue, err := c.GetIssue(ctx, iid)
	if err != nil {
		return err
	}

	// Remove the label from the list
	newLabels := make([]string, 0, len(issue.Labels))
	for _, l := range issue.Labels {
		if l != label {
			newLabels = append(newLabels, l)
		}
	}

	// Update with new label set
	labelOpts := gitlab.LabelOptions(newLabels)
	_, _, err = c.gl.Issues.UpdateIssue(pid, iid, &gitlab.UpdateIssueOptions{
		Labels: &labelOpts,
	}, gitlab.WithContext(ctx))

	return err
}

// SetProjectPath updates the project path for the client.
func (c *Client) SetProjectPath(projectPath string) {
	c.projectPath = projectPath
	c.projectID = 0 // Reset cached ID
}

// SetProjectID sets the numeric project ID directly.
func (c *Client) SetProjectID(projectID int64) {
	c.projectID = projectID
}

// ProjectPath returns the current project path.
func (c *Client) ProjectPath() string {
	return c.projectPath
}

// ProjectID returns the cached project ID (0 if not cached).
func (c *Client) ProjectID() int64 {
	return c.projectID
}

// Host returns the GitLab host.
func (c *Client) Host() string {
	if c.host != "" {
		return c.host
	}

	return "gitlab.com"
}

// CreateMergeRequest creates a new merge request.
func (c *Client) CreateMergeRequest(ctx context.Context, title, description, sourceBranch, targetBranch string, removeSourceBranch bool) (*gitlab.MergeRequest, error) {
	pid, err := c.getProjectID(ctx)
	if err != nil {
		return nil, err
	}

	mr, _, err := c.gl.MergeRequests.CreateMergeRequest(pid, &gitlab.CreateMergeRequestOptions{
		Title:              ptr(title),
		Description:        ptr(description),
		SourceBranch:       ptr(sourceBranch),
		TargetBranch:       ptr(targetBranch),
		RemoveSourceBranch: ptr(removeSourceBranch),
	}, gitlab.WithContext(ctx))
	if err != nil {
		return nil, wrapAPIError(err)
	}

	return mr, nil
}

// GetDefaultBranch returns the project's default branch.
func (c *Client) GetDefaultBranch(ctx context.Context) (string, error) {
	pid, err := c.getProjectID(ctx)
	if err != nil {
		return "", err
	}

	project, _, err := c.gl.Projects.GetProject(pid, nil, gitlab.WithContext(ctx))
	if err != nil {
		return "", wrapAPIError(err)
	}

	return project.DefaultBranch, nil
}
