package bitbucket

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/valksor/go-mehrhof/internal/naming"
	"github.com/valksor/go-mehrhof/internal/provider"
)

// ProviderName is the registered name for this provider.
const ProviderName = "bitbucket"

// Provider handles Bitbucket issue tasks.
type Provider struct {
	client *Client
	config *Config
}

// Config holds Bitbucket provider configuration.
type Config struct {
	Username          string
	AppPassword       string
	Workspace         string
	RepoSlug          string
	BranchPattern     string // Default: "issue/{key}-{slug}"
	CommitPrefix      string // Default: "[#{key}]"
	TargetBranch      string // Target branch for PRs
	CloseSourceBranch bool   // Close source branch when PR is merged
}

// Info returns provider metadata.
func Info() provider.ProviderInfo {
	return provider.ProviderInfo{
		Name:        ProviderName,
		Description: "Bitbucket Issues task source",
		Schemes:     []string{"bitbucket", "bb"},
		Priority:    20,
		Capabilities: provider.CapabilitySet{
			provider.CapRead:           true,
			provider.CapList:           true,
			provider.CapFetchComments:  true,
			provider.CapComment:        true,
			provider.CapUpdateStatus:   true,
			provider.CapSnapshot:       true,
			provider.CapCreatePR:       true,
			provider.CapCreateWorkUnit: true,
			provider.CapFetchSubtasks:  true,
		},
	}
}

// New creates a Bitbucket provider.
func New(ctx context.Context, cfg provider.Config) (any, error) {
	// Extract config values
	configUsername := cfg.GetString("username")
	configAppPassword := cfg.GetString("app_password")
	workspace := cfg.GetString("workspace")
	repoSlug := cfg.GetString("repo")

	// Resolve credentials
	username, appPassword, err := ResolveCredentials(configUsername, configAppPassword)
	if err != nil {
		return nil, err
	}

	// Set defaults for branch/commit patterns
	branchPattern := cfg.GetString("branch_pattern")
	if branchPattern == "" {
		branchPattern = "issue/{key}-{slug}"
	}
	commitPrefix := cfg.GetString("commit_prefix")
	if commitPrefix == "" {
		commitPrefix = "[#{key}]"
	}

	// PR config
	targetBranch := cfg.GetString("target_branch")
	closeSourceBranch := cfg.GetBool("close_source_branch")

	config := &Config{
		Username:          username,
		AppPassword:       appPassword,
		Workspace:         workspace,
		RepoSlug:          repoSlug,
		BranchPattern:     branchPattern,
		CommitPrefix:      commitPrefix,
		TargetBranch:      targetBranch,
		CloseSourceBranch: closeSourceBranch,
	}

	return &Provider{
		client: NewClient(username, appPassword, workspace, repoSlug),
		config: config,
	}, nil
}

// Match checks if input has the bitbucket: or bb: scheme prefix.
func (p *Provider) Match(input string) bool {
	return strings.HasPrefix(input, "bitbucket:") || strings.HasPrefix(input, "bb:")
}

// Parse extracts the issue reference from input.
func (p *Provider) Parse(input string) (string, error) {
	ref, err := ParseReference(input)
	if err != nil {
		return "", err
	}

	// If explicit workspace/repo provided, use it
	if ref.IsExplicit {
		return fmt.Sprintf("%s/%s#%d", ref.Workspace, ref.RepoSlug, ref.IssueID), nil
	}

	// Otherwise, check if we have workspace/repo configured
	workspace := p.config.Workspace
	repoSlug := p.config.RepoSlug

	if workspace == "" || repoSlug == "" {
		return "", fmt.Errorf("%w: use bitbucket:workspace/repo#N format or configure bitbucket.workspace and bitbucket.repo", ErrRepoNotConfigured)
	}

	return fmt.Sprintf("%s/%s#%d", workspace, repoSlug, ref.IssueID), nil
}

// Fetch reads a Bitbucket issue and creates a WorkUnit.
func (p *Provider) Fetch(ctx context.Context, id string) (*provider.WorkUnit, error) {
	ref, err := ParseReference(id)
	if err != nil {
		return nil, err
	}

	// Determine workspace/repo
	workspace := ref.Workspace
	repoSlug := ref.RepoSlug
	if workspace == "" {
		workspace = p.config.Workspace
	}
	if repoSlug == "" {
		repoSlug = p.config.RepoSlug
	}

	if workspace == "" || repoSlug == "" {
		return nil, ErrRepoNotConfigured
	}

	p.client.SetWorkspaceRepo(workspace, repoSlug)

	// Fetch issue
	issue, err := p.client.GetIssue(ctx, ref.IssueID)
	if err != nil {
		return nil, err
	}

	// Map to WorkUnit
	description := ""
	if issue.Content != nil {
		description = issue.Content.Raw
	}

	webURL := ""
	if issue.Links.HTML != nil {
		webURL = issue.Links.HTML.Href
	}

	wu := &provider.WorkUnit{
		ID:          strconv.Itoa(issue.ID),
		ExternalID:  fmt.Sprintf("%s/%s#%d", workspace, repoSlug, issue.ID),
		Provider:    ProviderName,
		Title:       issue.Title,
		Description: description,
		Status:      mapBitbucketState(issue.State),
		Priority:    mapBitbucketPriority(issue.Priority),
		Labels:      []string{}, // Bitbucket uses components instead of labels
		Assignees:   mapAssignee(issue.Assignee),
		CreatedAt:   issue.CreatedOn,
		UpdatedAt:   issue.UpdatedOn,
		Source: provider.SourceInfo{
			Type:      ProviderName,
			Reference: id,
			SyncedAt:  time.Now(),
		},

		// Naming fields
		ExternalKey: strconv.Itoa(issue.ID),
		TaskType:    mapBitbucketKind(issue.Kind),
		Slug:        naming.Slugify(issue.Title, 50),

		Metadata: map[string]any{
			"web_url":        webURL,
			"workspace":      workspace,
			"repo_slug":      repoSlug,
			"issue_id":       issue.ID,
			"kind":           issue.Kind,
			"branch_pattern": p.config.BranchPattern,
			"commit_prefix":  p.config.CommitPrefix,
		},
	}

	// Add component as label if present
	if issue.Component != nil && issue.Component.Name != "" {
		wu.Labels = append(wu.Labels, "component:"+issue.Component.Name)
	}

	// Fetch comments
	comments, err := p.client.GetIssueComments(ctx, ref.IssueID)
	if err == nil && len(comments) > 0 {
		wu.Comments = mapComments(comments)
	}

	// Extract linked issues
	if issue.Content != nil {
		linkedIDs := ExtractLinkedIssues(issue.Content.Raw)
		if len(linkedIDs) > 0 {
			wu.Metadata["linked_issues"] = linkedIDs
		}

		// Extract image URLs
		imageURLs := ExtractImageURLs(issue.Content.Raw)
		if len(imageURLs) > 0 {
			wu.Attachments = make([]provider.Attachment, len(imageURLs))
			for i, url := range imageURLs {
				wu.Attachments[i] = provider.Attachment{
					ID:   fmt.Sprintf("img-%d", i),
					Name: fmt.Sprintf("image-%d", i),
					URL:  url,
				}
			}
		}
	}

	return wu, nil
}

// Snapshot captures the issue content for storage.
func (p *Provider) Snapshot(ctx context.Context, id string) (*provider.Snapshot, error) {
	ref, err := ParseReference(id)
	if err != nil {
		return nil, err
	}

	workspace := ref.Workspace
	repoSlug := ref.RepoSlug
	if workspace == "" {
		workspace = p.config.Workspace
	}
	if repoSlug == "" {
		repoSlug = p.config.RepoSlug
	}

	if workspace == "" || repoSlug == "" {
		return nil, ErrRepoNotConfigured
	}

	p.client.SetWorkspaceRepo(workspace, repoSlug)

	// Fetch issue
	issue, err := p.client.GetIssue(ctx, ref.IssueID)
	if err != nil {
		return nil, err
	}

	snapshot := &provider.Snapshot{
		Type: ProviderName,
		Ref:  id,
		Files: []provider.SnapshotFile{
			{
				Path:    "issue.md",
				Content: formatIssueMarkdown(issue),
			},
		},
	}

	// Fetch and include comments
	comments, err := p.client.GetIssueComments(ctx, ref.IssueID)
	if err == nil && len(comments) > 0 {
		snapshot.Files = append(snapshot.Files, provider.SnapshotFile{
			Path:    "comments.md",
			Content: formatCommentsMarkdown(comments),
		})
	}

	return snapshot, nil
}

// List lists issues from the repository.
func (p *Provider) List(ctx context.Context, opts provider.ListOptions) ([]*provider.WorkUnit, error) {
	workspace := p.config.Workspace
	repoSlug := p.config.RepoSlug

	if workspace == "" || repoSlug == "" {
		return nil, ErrRepoNotConfigured
	}

	p.client.SetWorkspaceRepo(workspace, repoSlug)

	// Map status to Bitbucket state
	state := ""
	switch opts.Status {
	case provider.StatusOpen:
		state = "open"
	case provider.StatusClosed:
		state = "closed"
	case provider.StatusInProgress, provider.StatusReview, provider.StatusDone:
		// Bitbucket doesn't have these states, treat as open
		state = "open"
	}

	limit := opts.Limit
	if limit == 0 {
		limit = 50
	}

	issues, err := p.client.ListIssues(ctx, state, limit)
	if err != nil {
		return nil, err
	}

	result := make([]*provider.WorkUnit, len(issues))
	for i, issue := range issues {
		description := ""
		if issue.Content != nil {
			description = issue.Content.Raw
		}

		result[i] = &provider.WorkUnit{
			ID:          strconv.Itoa(issue.ID),
			ExternalID:  fmt.Sprintf("%s/%s#%d", workspace, repoSlug, issue.ID),
			Provider:    ProviderName,
			Title:       issue.Title,
			Description: description,
			Status:      mapBitbucketState(issue.State),
			Priority:    mapBitbucketPriority(issue.Priority),
			Assignees:   mapAssignee(issue.Assignee),
			CreatedAt:   issue.CreatedOn,
			UpdatedAt:   issue.UpdatedOn,
		}
	}

	return result, nil
}

// FetchComments fetches comments for a work unit.
func (p *Provider) FetchComments(ctx context.Context, workUnitID string) ([]provider.Comment, error) {
	ref, err := ParseReference(workUnitID)
	if err != nil {
		return nil, err
	}

	workspace := ref.Workspace
	repoSlug := ref.RepoSlug
	if workspace == "" {
		workspace = p.config.Workspace
	}
	if repoSlug == "" {
		repoSlug = p.config.RepoSlug
	}

	p.client.SetWorkspaceRepo(workspace, repoSlug)

	comments, err := p.client.GetIssueComments(ctx, ref.IssueID)
	if err != nil {
		return nil, err
	}

	return mapComments(comments), nil
}

// AddComment adds a comment to a work unit.
func (p *Provider) AddComment(ctx context.Context, workUnitID string, body string) (*provider.Comment, error) {
	ref, err := ParseReference(workUnitID)
	if err != nil {
		return nil, err
	}

	workspace := ref.Workspace
	repoSlug := ref.RepoSlug
	if workspace == "" {
		workspace = p.config.Workspace
	}
	if repoSlug == "" {
		repoSlug = p.config.RepoSlug
	}

	p.client.SetWorkspaceRepo(workspace, repoSlug)

	comment, err := p.client.AddIssueComment(ctx, ref.IssueID, body)
	if err != nil {
		return nil, err
	}

	content := ""
	if comment.Content != nil {
		content = comment.Content.Raw
	}

	author := provider.Person{
		ID: "unknown",
	}
	if comment.User != nil {
		author.ID = comment.User.UUID
		author.Name = comment.User.DisplayName
		if comment.User.Username != "" {
			author.Name = comment.User.Username
		}
	}

	return &provider.Comment{
		ID:        strconv.Itoa(comment.ID),
		Body:      content,
		CreatedAt: comment.CreatedOn,
		UpdatedAt: comment.UpdatedOn,
		Author:    author,
	}, nil
}

// UpdateStatus updates the status of a work unit.
func (p *Provider) UpdateStatus(ctx context.Context, workUnitID string, status provider.Status) error {
	ref, err := ParseReference(workUnitID)
	if err != nil {
		return err
	}

	workspace := ref.Workspace
	repoSlug := ref.RepoSlug
	if workspace == "" {
		workspace = p.config.Workspace
	}
	if repoSlug == "" {
		repoSlug = p.config.RepoSlug
	}

	p.client.SetWorkspaceRepo(workspace, repoSlug)

	// Map provider status to Bitbucket state
	var state string
	switch status {
	case provider.StatusOpen:
		state = "open"
	case provider.StatusClosed:
		state = "closed"
	case provider.StatusDone:
		state = "resolved"
	case provider.StatusInProgress, provider.StatusReview:
		// Bitbucket doesn't have these states, treat as open
		state = "open"
	}

	_, err = p.client.UpdateIssueState(ctx, ref.IssueID, state)

	return err
}

// GetConfig returns the provider configuration.
func (p *Provider) GetConfig() *Config {
	return p.config
}

// GetClient returns the Bitbucket API client.
func (p *Provider) GetClient() *Client {
	return p.client
}

// --- Helper functions ---

func mapBitbucketState(state string) provider.Status {
	switch state {
	case "new", "open":
		return provider.StatusOpen
	case "resolved", "closed":
		return provider.StatusClosed
	case "on hold":
		return provider.StatusOpen
	case "invalid", "duplicate", "wontfix":
		return provider.StatusClosed
	default:
		return provider.StatusOpen
	}
}

func mapBitbucketPriority(priority string) provider.Priority {
	switch priority {
	case "blocker", "critical":
		return provider.PriorityCritical
	case "major":
		return provider.PriorityHigh
	case "minor":
		return provider.PriorityNormal
	case "trivial":
		return provider.PriorityLow
	default:
		return provider.PriorityNormal
	}
}

func mapBitbucketKind(kind string) string {
	switch kind {
	case "bug":
		return "fix"
	case "enhancement":
		return "feature"
	case "proposal":
		return "feature"
	case "task":
		return "task"
	default:
		return "issue"
	}
}

func mapAssignee(assignee *User) []provider.Person {
	if assignee == nil {
		return nil
	}

	name := assignee.DisplayName
	if name == "" {
		name = assignee.Username
	}

	return []provider.Person{
		{
			ID:   assignee.UUID,
			Name: name,
		},
	}
}

func mapComments(comments []Comment) []provider.Comment {
	result := make([]provider.Comment, len(comments))
	for i, c := range comments {
		content := ""
		if c.Content != nil {
			content = c.Content.Raw
		}

		author := provider.Person{
			ID: "unknown",
		}
		if c.User != nil {
			author.ID = c.User.UUID
			author.Name = c.User.DisplayName
			if c.User.Username != "" {
				author.Name = c.User.Username
			}
		}

		result[i] = provider.Comment{
			ID:        strconv.Itoa(c.ID),
			Body:      content,
			CreatedAt: c.CreatedOn,
			UpdatedAt: c.UpdatedOn,
			Author:    author,
		}
	}

	return result
}

func formatIssueMarkdown(issue *Issue) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# #%d: %s\n\n", issue.ID, issue.Title))

	// Metadata
	sb.WriteString("## Metadata\n\n")
	sb.WriteString(fmt.Sprintf("- **State:** %s\n", issue.State))
	sb.WriteString(fmt.Sprintf("- **Priority:** %s\n", issue.Priority))
	sb.WriteString(fmt.Sprintf("- **Kind:** %s\n", issue.Kind))
	sb.WriteString(fmt.Sprintf("- **Created:** %s\n", issue.CreatedOn.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("- **Updated:** %s\n", issue.UpdatedOn.Format(time.RFC3339)))

	if issue.Reporter != nil {
		name := issue.Reporter.DisplayName
		if name == "" {
			name = issue.Reporter.Username
		}
		sb.WriteString(fmt.Sprintf("- **Reporter:** %s\n", name))
	}

	if issue.Assignee != nil {
		name := issue.Assignee.DisplayName
		if name == "" {
			name = issue.Assignee.Username
		}
		sb.WriteString(fmt.Sprintf("- **Assignee:** %s\n", name))
	}

	if issue.Component != nil && issue.Component.Name != "" {
		sb.WriteString(fmt.Sprintf("- **Component:** %s\n", issue.Component.Name))
	}

	if issue.Milestone != nil && issue.Milestone.Name != "" {
		sb.WriteString(fmt.Sprintf("- **Milestone:** %s\n", issue.Milestone.Name))
	}

	if issue.Links.HTML != nil {
		sb.WriteString(fmt.Sprintf("- **URL:** %s\n", issue.Links.HTML.Href))
	}

	// Body
	sb.WriteString("\n## Description\n\n")
	if issue.Content != nil && issue.Content.Raw != "" {
		sb.WriteString(issue.Content.Raw)
	} else {
		sb.WriteString("*No description*")
	}
	sb.WriteString("\n")

	return sb.String()
}

func formatCommentsMarkdown(comments []Comment) string {
	var sb strings.Builder

	sb.WriteString("# Comments\n\n")

	for _, c := range comments {
		authorName := "Unknown"
		if c.User != nil {
			authorName = c.User.DisplayName
			if authorName == "" {
				authorName = c.User.Username
			}
		}

		sb.WriteString(fmt.Sprintf("## Comment by %s\n\n", authorName))
		sb.WriteString(fmt.Sprintf("*%s*\n\n", c.CreatedOn.Format(time.RFC3339)))

		if c.Content != nil {
			sb.WriteString(c.Content.Raw)
		}
		sb.WriteString("\n\n---\n\n")
	}

	return sb.String()
}
