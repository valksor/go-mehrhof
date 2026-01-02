package jira

import (
	"context"
	"strings"
	"time"

	"github.com/valksor/go-mehrhof/internal/naming"
	"github.com/valksor/go-mehrhof/internal/provider"
)

// ProviderName is the registered name for this provider.
const ProviderName = "jira"

// Provider handles Jira issues.
type Provider struct {
	client         *Client
	defaultProject string // Default project key
	baseURL        string // Base URL for API requests
}

// Config holds Jira provider configuration.
type Config struct {
	Token   string // API token
	Email   string // Email for Cloud auth
	BaseURL string // Base URL (optional, auto-detected)
	Project string // Default project key
}

// Info returns provider metadata.
func Info() provider.ProviderInfo {
	return provider.ProviderInfo{
		Name:        ProviderName,
		Description: "Jira issue source",
		Schemes:     []string{"jira", "j"},
		Priority:    20, // Same as GitHub and Linear
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
			provider.CapFetchSubtasks:      true,
		},
	}
}

// New creates a Jira provider.
func New(_ context.Context, cfg provider.Config) (any, error) {
	token := cfg.GetString("token")
	email := cfg.GetString("email")
	baseURL := cfg.GetString("base_url")
	project := cfg.GetString("project")

	// Try to resolve token from env if not provided
	if token == "" {
		resolvedToken, err := ResolveToken("")
		if err != nil {
			return nil, err
		}
		token = resolvedToken
	}

	return &Provider{
		client:         NewClient(token, email, baseURL),
		defaultProject: project,
		baseURL:        baseURL,
	}, nil
}

// Match checks if input has the jira: or j: scheme prefix.
func (p *Provider) Match(input string) bool {
	return strings.HasPrefix(input, "jira:") || strings.HasPrefix(input, "j:")
}

// Parse extracts the issue key from input.
func (p *Provider) Parse(input string) (string, error) {
	ref, err := ParseReference(input)
	if err != nil {
		return "", err
	}

	// Update provider's base URL if detected from reference
	if ref.BaseURL != "" && p.baseURL == "" {
		p.baseURL = ref.BaseURL
		p.client.SetBaseURL(ref.BaseURL)
	}

	return ref.IssueKey, nil
}

// Fetch reads a Jira issue and creates a WorkUnit.
func (p *Provider) Fetch(ctx context.Context, id string) (*provider.WorkUnit, error) {
	ref, err := ParseReference(id)
	if err != nil {
		return nil, err
	}

	// Update base URL if detected from reference
	if ref.BaseURL != "" && p.baseURL == "" {
		p.baseURL = ref.BaseURL
		p.client.SetBaseURL(ref.BaseURL)
	}

	// Fetch issue from Jira
	issue, err := p.client.GetIssue(ctx, ref.IssueKey)
	if err != nil {
		return nil, err
	}

	// Map to WorkUnit
	wu := &provider.WorkUnit{
		ID:          issue.ID,
		ExternalID:  issue.Key,
		Provider:    ProviderName,
		Title:       issue.Fields.Summary,
		Description: issue.Fields.Description,
		Status:      mapJiraStatus(issue.Fields.Status.Name),
		Priority:    mapJiraPriority(issue.Fields.Priority),
		Labels:      issue.Fields.Labels,
		Assignees:   mapAssignees(issue.Fields.Assignee),
		CreatedAt:   issue.Fields.Created,
		UpdatedAt:   issue.Fields.Updated,
		Source: provider.SourceInfo{
			Type:      ProviderName,
			Reference: issue.Key,
			SyncedAt:  time.Now(),
		},
		// Naming fields for branch/commit customization
		ExternalKey: issue.Key,
		TaskType:    inferTaskTypeFromLabels(issue.Fields.Labels),
		Slug:        naming.Slugify(issue.Fields.Summary, 50),
		Metadata:    buildMetadata(issue),
	}

	// Fetch comments if available
	comments, err := p.client.GetComments(ctx, ref.IssueKey)
	if err == nil && len(comments) > 0 {
		wu.Comments = mapComments(comments)
	}

	// Fetch attachments
	attachments, err := p.client.GetAttachments(ctx, ref.IssueKey)
	if err == nil && len(attachments) > 0 {
		wu.Attachments = mapAttachments(attachments)
	}

	return wu, nil
}

// GetClient returns the Jira API client.
func (p *Provider) GetClient() *Client {
	return p.client
}

// GetDefaultProject returns the default project key.
func (p *Provider) GetDefaultProject() string {
	return p.defaultProject
}

// GetBaseURL returns the base URL.
func (p *Provider) GetBaseURL() string {
	return p.baseURL
}

// ──────────────────────────────────────────────────────────────────────────────
// Helper functions
// ──────────────────────────────────────────────────────────────────────────────

// mapJiraStatus converts Jira status to provider status.
func mapJiraStatus(status string) provider.Status {
	switch strings.ToLower(status) {
	case "to do", "backlog", "open", "new":
		return provider.StatusOpen
	case "in progress", "started", "in development":
		return provider.StatusInProgress
	case "in review", "code review", "under review", "verification":
		return provider.StatusReview
	case "done", "closed", "resolved", "complete", "finished":
		return provider.StatusDone
	case "won't fix", "cancelled", "canceled", "obsolete", "won't do":
		return provider.StatusClosed
	default:
		return provider.StatusOpen
	}
}

// mapProviderStatusToJiraTransition converts provider status to common Jira transition names
// Returns a list of possible transition names to try.
func mapProviderStatusToJiraTransitions(status provider.Status) []string {
	switch status {
	case provider.StatusOpen:
		return []string{"To Do", "Backlog", "Open", "Reopen", "New"}
	case provider.StatusInProgress:
		return []string{"In Progress", "Start Progress", "Start Development"}
	case provider.StatusReview:
		return []string{"In Review", "Code Review", "Ready for Review"}
	case provider.StatusDone:
		return []string{"Done", "Close", "Resolve", "Complete", "Mark as Done"}
	case provider.StatusClosed:
		return []string{"Closed", "Cancel", "Won't Fix", "Won't Do"}
	default:
		return []string{"To Do"}
	}
}

// mapJiraPriority converts Jira priority to provider priority.
func mapJiraPriority(p *Priority) provider.Priority {
	if p == nil {
		return provider.PriorityNormal
	}

	switch strings.ToLower(p.Name) {
	case "highest", "critical":
		return provider.PriorityCritical
	case "high":
		return provider.PriorityHigh
	case "low", "lowest":
		return provider.PriorityLow
	case "medium", "normal", "default":
		return provider.PriorityNormal
	default:
		return provider.PriorityNormal
	}
}

// mapProviderPriorityToJira converts provider priority to Jira priority name.
func mapProviderPriorityToJira(priority provider.Priority) string {
	switch priority {
	case provider.PriorityCritical:
		return "Highest"
	case provider.PriorityHigh:
		return "High"
	case provider.PriorityNormal:
		return "Medium"
	case provider.PriorityLow:
		return "Low"
	}

	return "Medium"
}

// mapAssignees converts Jira assignee to provider Person.
func mapAssignees(assignee *User) []provider.Person {
	if assignee == nil {
		return []provider.Person{}
	}

	return []provider.Person{
		{
			ID:    assignee.AccountID,
			Name:  assignee.DisplayName,
			Email: assignee.EmailAddress,
		},
	}
}

// mapComments converts Jira comments to provider comments.
func mapComments(comments []*Comment) []provider.Comment {
	if comments == nil {
		return nil
	}

	result := make([]provider.Comment, 0, len(comments))
	for _, c := range comments {
		var author provider.Person
		if c.Author != nil {
			author = provider.Person{
				ID:   c.Author.AccountID,
				Name: c.Author.DisplayName,
			}
		}
		result = append(result, provider.Comment{
			ID:        c.ID,
			Body:      c.Body,
			CreatedAt: c.Created,
			UpdatedAt: c.Updated,
			Author:    author,
		})
	}

	return result
}

// mapAttachments converts Jira attachments to provider attachments.
func mapAttachments(attachments []*Attachment) []provider.Attachment {
	if attachments == nil {
		return nil
	}

	result := make([]provider.Attachment, 0, len(attachments))
	for _, a := range attachments {
		result = append(result, provider.Attachment{
			ID:          a.ID,
			Name:        a.Filename,
			URL:         a.Content,
			ContentType: a.MimeType,
			Size:        a.Size,
			CreatedAt:   a.Created,
		})
	}

	return result
}

// inferTaskTypeFromLabels determines task type from label names.
func inferTaskTypeFromLabels(labels []string) string {
	for _, label := range labels {
		switch strings.ToLower(label) {
		case "bug", "bugfix", "fix":
			return "fix"
		case "feature", "enhancement":
			return "feature"
		case "docs", "documentation":
			return "docs"
		case "refactor":
			return "refactor"
		case "chore":
			return "chore"
		case "test":
			return "test"
		case "ci":
			return "ci"
		}
	}

	return "issue"
}

// buildMetadata creates metadata map from issue.
func buildMetadata(issue *Issue) map[string]any {
	metadata := make(map[string]any)

	metadata["key"] = issue.Key
	metadata["status"] = issue.Fields.Status.Name
	metadata["priority"] = issue.Fields.Priority.Name

	if issue.Fields.Project != nil {
		metadata["project_key"] = issue.Fields.Project.Key
		metadata["project_name"] = issue.Fields.Project.Name
	}

	if issue.Fields.Issuetype != nil {
		metadata["issue_type"] = issue.Fields.Issuetype.Name
	}

	if issue.Fields.Sprint != nil {
		metadata["sprint_name"] = issue.Fields.Sprint.Name
		metadata["sprint_id"] = issue.Fields.Sprint.ID
	}

	return metadata
}
