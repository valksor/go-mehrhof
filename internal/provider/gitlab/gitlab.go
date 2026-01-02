package gitlab

import (
	"context"
	"fmt"
	"strings"
	"time"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/valksor/go-mehrhof/internal/naming"
	"github.com/valksor/go-mehrhof/internal/provider"
)

// ProviderName is the registered name for this provider.
const ProviderName = "gitlab"

// Provider handles GitLab issue tasks.
type Provider struct {
	client      *Client
	projectPath string
	config      *Config
}

// Config holds GitLab provider configuration.
type Config struct {
	Token              string
	Host               string // e.g., "https://gitlab.com" or custom host
	ProjectPath        string // e.g., "group/project" or "12345" (project ID)
	ProjectID          int64  // Numeric project ID (alternative to path)
	BranchPattern      string // Default: "issue/{key}-{slug}"
	CommitPrefix       string // Default: "[#{key}]"
	TargetBranch       string // Target branch for MRs (default: repo default branch)
	DraftMR            bool   // Create MRs as draft by default
	RemoveSourceBranch bool   // Remove source branch when MR is merged
}

// Info returns provider metadata.
func Info() provider.ProviderInfo {
	return provider.ProviderInfo{
		Name:        ProviderName,
		Description: "GitLab Issues task source",
		Schemes:     []string{"gitlab", "gl"},
		Priority:    20, // Higher than file/directory
		Capabilities: provider.CapabilitySet{
			provider.CapRead:               true,
			provider.CapList:               true,
			provider.CapFetchComments:      true,
			provider.CapComment:            true,
			provider.CapUpdateStatus:       true,
			provider.CapManageLabels:       true,
			provider.CapCreateWorkUnit:     true,
			provider.CapDownloadAttachment: true,
			provider.CapSnapshot:           true,
			provider.CapCreatePR:           true, // MR creation
			provider.CapFetchSubtasks:      true,
		},
	}
}

// New creates a GitLab provider.
func New(ctx context.Context, cfg provider.Config) (any, error) {
	// Extract config values
	token := cfg.GetString("token")
	host := cfg.GetString("host")
	projectPath := cfg.GetString("project_path")

	// Resolve token
	resolvedToken, err := ResolveToken(token)
	if err != nil {
		return nil, err
	}

	// Set defaults
	if host == "" {
		host = "https://gitlab.com"
	}
	// Strip trailing slash from host
	host = strings.TrimSuffix(host, "/")

	// Set defaults for branch/commit patterns
	branchPattern := cfg.GetString("branch_pattern")
	if branchPattern == "" {
		branchPattern = "issue/{key}-{slug}"
	}
	commitPrefix := cfg.GetString("commit_prefix")
	if commitPrefix == "" {
		commitPrefix = "[#{key}]"
	}

	// MR-related config
	targetBranch := cfg.GetString("target_branch")
	draftMR := cfg.GetBool("draft_mr")
	removeSourceBranch := cfg.GetBool("remove_source_branch")

	config := &Config{
		Token:              resolvedToken,
		Host:               host,
		ProjectPath:        projectPath,
		BranchPattern:      branchPattern,
		CommitPrefix:       commitPrefix,
		TargetBranch:       targetBranch,
		DraftMR:            draftMR,
		RemoveSourceBranch: removeSourceBranch,
	}

	return &Provider{
		client: NewClient(resolvedToken, host, projectPath, 0),
		config: config,
	}, nil
}

// Match checks if input has the gitlab: or gl: scheme prefix.
func (p *Provider) Match(input string) bool {
	return strings.HasPrefix(input, "gitlab:") || strings.HasPrefix(input, "gl:")
}

// Parse extracts the issue reference from input.
func (p *Provider) Parse(input string) (string, error) {
	ref, err := ParseReference(input)
	if err != nil {
		return "", err
	}

	// If explicit project provided, use it
	if ref.IsExplicit {
		if ref.ProjectPath != "" {
			return fmt.Sprintf("%s#%d", ref.ProjectPath, ref.IssueIID), nil
		}
		if ref.ProjectID > 0 {
			return fmt.Sprintf("%d#%d", ref.ProjectID, ref.IssueIID), nil
		}
	}

	// Otherwise, check if we have project configured
	projectPath := p.projectPath
	if projectPath == "" {
		projectPath = p.config.ProjectPath
	}

	if projectPath == "" {
		return "", fmt.Errorf("%w: use gitlab:group/project#N format or configure gitlab.project_path", ErrProjectNotConfigured)
	}

	return fmt.Sprintf("%s#%d", projectPath, ref.IssueIID), nil
}

// Fetch reads a GitLab issue and creates a WorkUnit.
func (p *Provider) Fetch(ctx context.Context, id string) (*provider.WorkUnit, error) {
	ref, err := ParseReference(id)
	if err != nil {
		return nil, err
	}

	// Determine project
	projectPath := ref.ProjectPath
	if projectPath == "" {
		projectPath = p.projectPath
		if projectPath == "" {
			projectPath = p.config.ProjectPath
		}
	}

	// If explicit project ID in reference, use it
	if ref.ProjectID > 0 {
		p.client.SetProjectID(ref.ProjectID)
	} else if projectPath != "" {
		p.client.SetProjectPath(projectPath)
	} else {
		return nil, ErrProjectNotConfigured
	}

	// Fetch issue
	issue, err := p.client.GetIssue(ctx, ref.IssueIID)
	if err != nil {
		return nil, err
	}

	// Determine the project path for ExternalID
	displayProject := projectPath
	if displayProject == "" && ref.ProjectID > 0 {
		displayProject = fmt.Sprintf("%d", ref.ProjectID)
	} else if displayProject == "" && issue.ProjectID != 0 {
		displayProject = fmt.Sprintf("%d", issue.ProjectID)
	}

	// Map to WorkUnit
	wu := &provider.WorkUnit{
		ID:          fmt.Sprintf("%d", issue.IID),
		ExternalID:  fmt.Sprintf("%s#%d", displayProject, issue.IID),
		Provider:    ProviderName,
		Title:       issue.Title,
		Description: issue.Description,
		Status:      mapGitLabState(issue.State),
		Priority:    inferPriorityFromLabels(issue.Labels),
		Labels:      issue.Labels,
		Assignees:   mapAssignees(issue.Assignees),
		CreatedAt:   *issue.CreatedAt,
		UpdatedAt:   *issue.UpdatedAt,
		Source: provider.SourceInfo{
			Type:      ProviderName,
			Reference: id,
			SyncedAt:  time.Now(),
		},

		// Naming fields for branch/commit customization
		ExternalKey: fmt.Sprintf("%d", issue.IID),
		TaskType:    inferTypeFromLabels(issue.Labels),
		Slug:        naming.Slugify(issue.Title, 50),

		Metadata: map[string]any{
			"web_url":        issue.WebURL,
			"project_path":   displayProject,
			"project_id":     issue.ProjectID,
			"issue_iid":      issue.IID,
			"branch_pattern": p.config.BranchPattern,
			"commit_prefix":  p.config.CommitPrefix,
			"host":           p.client.Host(),
		},
	}

	// Fetch notes (comments) if available
	notes, err := p.client.GetIssueNotes(ctx, ref.IssueIID)
	if err == nil && len(notes) > 0 {
		wu.Comments = mapNotes(notes)
	}

	// Extract linked issues
	linkedIIDs := ExtractLinkedIssues(issue.Description)
	if len(linkedIIDs) > 0 {
		wu.Metadata["linked_issues"] = linkedIIDs
	}

	// Extract image URLs
	imageURLs := ExtractImageURLs(issue.Description)
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

	return wu, nil
}

// Snapshot captures the issue content for storage.
func (p *Provider) Snapshot(ctx context.Context, id string) (*provider.Snapshot, error) {
	ref, err := ParseReference(id)
	if err != nil {
		return nil, err
	}

	// Determine project
	projectPath := ref.ProjectPath
	if projectPath == "" {
		projectPath = p.projectPath
		if projectPath == "" {
			projectPath = p.config.ProjectPath
		}
	}

	if ref.ProjectID > 0 {
		p.client.SetProjectID(ref.ProjectID)
	} else if projectPath != "" {
		p.client.SetProjectPath(projectPath)
	} else {
		return nil, ErrProjectNotConfigured
	}

	// Fetch main issue
	issue, err := p.client.GetIssue(ctx, ref.IssueIID)
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

	// Fetch and include notes (comments)
	notes, err := p.client.GetIssueNotes(ctx, ref.IssueIID)
	if err == nil && len(notes) > 0 {
		snapshot.Files = append(snapshot.Files, provider.SnapshotFile{
			Path:    "notes.md",
			Content: formatNotesMarkdown(notes),
		})
	}

	return snapshot, nil
}

// List lists issues from the project.
func (p *Provider) List(ctx context.Context, opts provider.ListOptions) ([]*provider.WorkUnit, error) {
	// Set up project
	projectPath := p.config.ProjectPath
	if projectPath != "" {
		p.client.SetProjectPath(projectPath)
	}

	listOpts := &gitlab.ListProjectIssuesOptions{
		OrderBy: ptr("created_at"),
		Sort:    ptr("desc"),
	}

	// Map status
	if opts.Status != "" {
		switch opts.Status {
		case provider.StatusOpen:
			listOpts.State = ptr("opened")
		case provider.StatusClosed:
			listOpts.State = ptr("closed")
		}
	}

	// Map labels
	if len(opts.Labels) > 0 {
		labelOpts := gitlab.LabelOptions(opts.Labels)
		listOpts.Labels = &labelOpts
	}

	// Pagination - Page and PerPage are int64 in ListOptions
	if opts.Limit > 0 {
		listOpts.PerPage = int64(opts.Limit)
	} else {
		listOpts.PerPage = 100
	}
	if opts.Offset > 0 {
		page := (int64(opts.Offset) / 100) + 1
		listOpts.Page = page
	}

	issues, err := p.client.ListIssues(ctx, listOpts)
	if err != nil {
		return nil, err
	}

	result := make([]*provider.WorkUnit, len(issues))
	for i, issue := range issues {
		result[i] = &provider.WorkUnit{
			ID:          fmt.Sprintf("%d", issue.IID),
			ExternalID:  fmt.Sprintf("%s#%d", p.config.ProjectPath, issue.IID),
			Provider:    ProviderName,
			Title:       issue.Title,
			Description: issue.Description,
			Status:      mapGitLabState(issue.State),
			Priority:    inferPriorityFromLabels(issue.Labels),
			Labels:      issue.Labels,
			Assignees:   mapAssignees(issue.Assignees),
			CreatedAt:   *issue.CreatedAt,
			UpdatedAt:   *issue.UpdatedAt,
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

	notes, err := p.client.GetIssueNotes(ctx, ref.IssueIID)
	if err != nil {
		return nil, err
	}

	return mapNotes(notes), nil
}

// AddComment adds a comment to a work unit.
func (p *Provider) AddComment(ctx context.Context, workUnitID string, body string) (*provider.Comment, error) {
	ref, err := ParseReference(workUnitID)
	if err != nil {
		return nil, err
	}

	note, err := p.client.AddNote(ctx, ref.IssueIID, body)
	if err != nil {
		return nil, err
	}

	author := provider.Person{
		ID:   fmt.Sprintf("%d", note.Author.ID),
		Name: note.Author.Username,
	}

	// Email is now a string, not a pointer in the new API
	if note.Author.Email != "" {
		author.Email = note.Author.Email
	}

	var updatedAt time.Time
	if note.UpdatedAt != nil {
		updatedAt = *note.UpdatedAt
	} else if note.CreatedAt != nil {
		updatedAt = *note.CreatedAt
	}

	return &provider.Comment{
		ID:        fmt.Sprintf("%d", note.ID),
		Body:      note.Body,
		CreatedAt: *note.CreatedAt,
		UpdatedAt: updatedAt,
		Author:    author,
	}, nil
}

// UpdateStatus updates the status of a work unit.
func (p *Provider) UpdateStatus(ctx context.Context, workUnitID string, status provider.Status) error {
	ref, err := ParseReference(workUnitID)
	if err != nil {
		return err
	}

	var stateEvent *string
	switch status {
	case provider.StatusOpen:
		stateEvent = ptr("reopen")
	case provider.StatusClosed:
		stateEvent = ptr("close")
	}

	_, err = p.client.UpdateIssue(ctx, ref.IssueIID, &gitlab.UpdateIssueOptions{
		StateEvent: stateEvent,
	})

	return err
}

// AddLabels adds labels to a work unit.
func (p *Provider) AddLabels(ctx context.Context, workUnitID string, labels []string) error {
	ref, err := ParseReference(workUnitID)
	if err != nil {
		return err
	}
	return p.client.AddLabels(ctx, ref.IssueIID, labels)
}

// RemoveLabels removes labels from a work unit.
func (p *Provider) RemoveLabels(ctx context.Context, workUnitID string, labels []string) error {
	ref, err := ParseReference(workUnitID)
	if err != nil {
		return err
	}

	for _, label := range labels {
		if err := p.client.RemoveLabel(ctx, ref.IssueIID, label); err != nil {
			return err
		}
	}
	return nil
}

// CreateWorkUnit creates a new work unit.
func (p *Provider) CreateWorkUnit(ctx context.Context, opts provider.CreateWorkUnitOptions) (*provider.WorkUnit, error) {
	createOpts := &gitlab.CreateIssueOptions{
		Title:       ptr(opts.Title),
		Description: ptr(opts.Description),
	}

	if len(opts.Labels) > 0 {
		labelOpts := gitlab.LabelOptions(opts.Labels)
		createOpts.Labels = &labelOpts
	}

	// Note: Assignees are not yet implemented - would need user lookup to convert usernames to IDs

	issue, err := p.client.CreateIssue(ctx, createOpts)
	if err != nil {
		return nil, err
	}

	return p.Fetch(ctx, fmt.Sprintf("%d", issue.IID))
}

// GetConfig returns the provider configuration.
func (p *Provider) GetConfig() *Config {
	return p.config
}

// GetClient returns the GitLab API client.
func (p *Provider) GetClient() *Client {
	return p.client
}

// --- Helper functions ---

func mapGitLabState(state string) provider.Status {
	switch state {
	case "opened":
		return provider.StatusOpen
	case "closed":
		return provider.StatusClosed
	default:
		return provider.StatusOpen
	}
}

// labelTypeMap maps GitLab labels to task types.
var labelTypeMap = map[string]string{
	"bug":           "fix",
	"bugfix":        "fix",
	"fix":           "fix",
	"feature":       "feature",
	"enhancement":   "feature",
	"docs":          "docs",
	"documentation": "docs",
	"refactor":      "refactor",
	"chore":         "chore",
	"test":          "test",
	"ci":            "ci",
}

func inferTypeFromLabels(labels []string) string {
	for _, label := range labels {
		name := strings.ToLower(label)
		if t, ok := labelTypeMap[name]; ok {
			return t
		}
	}
	return "issue"
}

// labelPriorityMap maps GitLab labels to priorities.
var labelPriorityMap = map[string]provider.Priority{
	"critical":      provider.PriorityCritical,
	"urgent":        provider.PriorityCritical,
	"priority:high": provider.PriorityHigh,
	"high-priority": provider.PriorityHigh,
	"priority:low":  provider.PriorityLow,
	"low-priority":  provider.PriorityLow,
}

func inferPriorityFromLabels(labels []string) provider.Priority {
	for _, label := range labels {
		name := strings.ToLower(label)
		if p, ok := labelPriorityMap[name]; ok {
			return p
		}
	}
	return provider.PriorityNormal
}

func mapAssignees(assignees []*gitlab.IssueAssignee) []provider.Person {
	persons := make([]provider.Person, len(assignees))
	for i, a := range assignees {
		persons[i] = provider.Person{
			ID:   fmt.Sprintf("%d", a.ID),
			Name: a.Username,
		}
	}
	return persons
}

func mapNotes(notes []*gitlab.Note) []provider.Comment {
	result := make([]provider.Comment, len(notes))
	for i, n := range notes {
		author := provider.Person{
			ID:   fmt.Sprintf("%d", n.Author.ID),
			Name: n.Author.Username,
		}

		var updatedAt time.Time
		if n.UpdatedAt != nil {
			updatedAt = *n.UpdatedAt
		} else if n.CreatedAt != nil {
			updatedAt = *n.CreatedAt
		}

		result[i] = provider.Comment{
			ID:        fmt.Sprintf("%d", n.ID),
			Body:      n.Body,
			CreatedAt: *n.CreatedAt,
			UpdatedAt: updatedAt,
			Author:    author,
		}
	}
	return result
}

func formatIssueMarkdown(issue *gitlab.Issue) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# #%d: %s\n\n", issue.IID, issue.Title))

	// Metadata
	sb.WriteString("## Metadata\n\n")
	sb.WriteString(fmt.Sprintf("- **State:** %s\n", issue.State))
	if issue.CreatedAt != nil {
		sb.WriteString(fmt.Sprintf("- **Created:** %s\n", issue.CreatedAt.Format(time.RFC3339)))
	}
	if issue.UpdatedAt != nil {
		sb.WriteString(fmt.Sprintf("- **Updated:** %s\n", issue.UpdatedAt.Format(time.RFC3339)))
	}

	if issue.Author != nil {
		sb.WriteString(fmt.Sprintf("- **Author:** @%s\n", issue.Author.Username))
	}

	if len(issue.Labels) > 0 {
		sb.WriteString(fmt.Sprintf("- **Labels:** %s\n", strings.Join(issue.Labels, ", ")))
	}

	if len(issue.Assignees) > 0 {
		assignees := make([]string, len(issue.Assignees))
		for i, a := range issue.Assignees {
			assignees[i] = "@" + a.Username
		}
		sb.WriteString(fmt.Sprintf("- **Assignees:** %s\n", strings.Join(assignees, ", ")))
	}

	if issue.WebURL != "" {
		sb.WriteString(fmt.Sprintf("- **URL:** %s\n", issue.WebURL))
	}

	// Body
	sb.WriteString("\n## Description\n\n")
	if issue.Description != "" {
		sb.WriteString(issue.Description)
	} else {
		sb.WriteString("*No description*")
	}
	sb.WriteString("\n")

	return sb.String()
}

func formatNotesMarkdown(notes []*gitlab.Note) string {
	var sb strings.Builder

	sb.WriteString("# Notes\n\n")

	for _, n := range notes {
		authorName := "Unknown"
		if n.Author.Username != "" {
			authorName = n.Author.Username
		}

		sb.WriteString(fmt.Sprintf("## Note by @%s\n\n", authorName))
		if n.CreatedAt != nil {
			sb.WriteString(fmt.Sprintf("*%s*\n\n", n.CreatedAt.Format(time.RFC3339)))
		}
		sb.WriteString(n.Body)
		sb.WriteString("\n\n---\n\n")
	}

	return sb.String()
}
