package youtrack

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/valksor/go-mehrhof/internal/naming"
	"github.com/valksor/go-mehrhof/internal/provider"
)

// Provider implements the YouTrack task provider
type Provider struct {
	client *Client
	config *Config
}

// New creates a new YouTrack provider
func New(_ context.Context, cfg provider.Config) (any, error) {
	token := cfg.GetString("token")
	host := cfg.GetString("host")

	resolvedToken, err := ResolveToken(token)
	if err != nil {
		return nil, err
	}

	config := &Config{
		Token: resolvedToken,
		Host:  host,
	}

	return &Provider{
		client: NewClient(resolvedToken, host),
		config: config,
	}, nil
}

// Match checks if input matches a YouTrack reference
func (p *Provider) Match(input string) bool {
	input = strings.TrimSpace(input)
	return strings.HasPrefix(input, "youtrack:") ||
		strings.HasPrefix(input, "yt:") ||
		urlPattern.MatchString(input) ||
		readableIDPattern.MatchString(input)
}

// Parse extracts the issue ID from input
func (p *Provider) Parse(input string) (string, error) {
	ref, err := ParseReference(input)
	if err != nil {
		return "", err
	}
	return ref.ID, nil
}

// Fetch retrieves a YouTrack issue and converts it to a WorkUnit
func (p *Provider) Fetch(ctx context.Context, id string) (*provider.WorkUnit, error) {
	issue, err := p.client.GetIssue(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("fetch issue: %w", err)
	}

	// Fetch comments
	comments, _ := p.client.GetComments(ctx, id)

	// Fetch attachments
	attachments, _ := p.client.GetAttachments(ctx, id)

	return p.issueToWorkUnit(issue, comments, attachments), nil
}

// Snapshot captures the issue content from YouTrack
func (p *Provider) Snapshot(ctx context.Context, id string) (*provider.Snapshot, error) {
	issue, err := p.client.GetIssue(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("fetch issue for snapshot: %w", err)
	}

	comments, _ := p.client.GetComments(ctx, id)

	return &provider.Snapshot{
		Type: ProviderName,
		Ref:  id,
		Files: []provider.SnapshotFile{
			{
				Path:    "issue.md",
				Content: formatIssueMarkdown(issue, comments),
			},
		},
	}, nil
}

// issueToWorkUnit converts YouTrack Issue to provider WorkUnit
func (p *Provider) issueToWorkUnit(issue *Issue, comments []Comment, attachments []Attachment) *provider.WorkUnit {
	// Extract tag names
	tagNames := make([]string, len(issue.Tags))
	for i, tag := range issue.Tags {
		tagNames[i] = tag.Name
	}

	// Map assignees from custom fields
	assignees := p.extractAssignees(issue)

	// Extract priority from custom fields
	priority := p.mapPriority(issue)

	// Extract status from custom fields
	status := p.mapStatus(issue)

	// Convert attachments
	providerAttachments := make([]provider.Attachment, len(attachments))
	for i, att := range attachments {
		providerAttachments[i] = provider.Attachment{
			ID:          att.ID,
			Name:        att.Name,
			URL:         att.URL,
			ContentType: att.MimeType,
			Size:        att.Size,
			CreatedAt:   timeFromMillis(att.Created),
		}
	}

	// Convert comments
	providerComments := make([]provider.Comment, len(comments))
	for i, c := range comments {
		providerComments[i] = provider.Comment{
			ID:        c.ID,
			Author:    provider.Person{ID: c.Author.ID, Name: c.Author.FullName},
			Body:      c.Text,
			CreatedAt: timeFromMillis(c.Created),
			UpdatedAt: timeFromMillis(c.Updated),
		}
	}

	// Extract subtask IDs
	subtasks := make([]string, len(issue.Subtasks))
	for i, st := range issue.Subtasks {
		subtasks[i] = st.IDReadable
	}

	// Build URL
	issueURL := ""
	if issue.Project.ShortName != "" {
		issueURL = fmt.Sprintf("https://youtrack.cloud/issue/%s", issue.IDReadable)
	}

	return &provider.WorkUnit{
		ID:          issue.IDReadable,
		ExternalID:  issue.ID,
		Provider:    ProviderName,
		Title:       issue.Summary,
		Description: issue.Description,
		Status:      status,
		Priority:    priority,
		Labels:      tagNames,
		Assignees:   assignees,
		Comments:    providerComments,
		Attachments: providerAttachments,
		Subtasks:    subtasks,
		CreatedAt:   timeFromMillis(issue.Created),
		UpdatedAt:   timeFromMillis(issue.Updated),
		Source: provider.SourceInfo{
			Type:      ProviderName,
			Reference: issue.IDReadable,
			SyncedAt:  time.Now(),
		},
		ExternalKey: issue.IDReadable,
		TaskType:    p.inferTaskType(issue),
		Slug:        naming.Slugify(issue.Summary, 50),
		Metadata: map[string]any{
			"yt_id":       issue.ID,
			"yt_project":  issue.Project.ShortName,
			"yt_resolved": issue.Resolved > 0,
			"yt_url":      issueURL,
			"custom_fields": issue.CustomFields,
		},
	}
}

// extractAssignees extracts assignees from custom fields
func (p *Provider) extractAssignees(issue *Issue) []provider.Person {
	for _, cf := range issue.CustomFields {
		if cf.Name == "Assignee" && cf.Value != nil {
			return mapAssigneeValue(cf.Value)
		}
	}
	return []provider.Person{}
}

// mapPriority extracts priority from custom fields
func (p *Provider) mapPriority(issue *Issue) provider.Priority {
	for _, cf := range issue.CustomFields {
		if cf.Name == "Priority" {
			return mapPriorityValue(cf.Value)
		}
	}
	return provider.PriorityNormal
}

// mapStatus extracts status from custom fields
func (p *Provider) mapStatus(issue *Issue) provider.Status {
	// Check if resolved
	if issue.Resolved > 0 {
		return provider.StatusDone
	}

	for _, cf := range issue.CustomFields {
		if cf.Name == "State" || cf.Name == "Status" {
			return mapStatusValue(cf.Value)
		}
	}
	return provider.StatusOpen
}

// inferTaskType infers task type from custom fields
func (p *Provider) inferTaskType(issue *Issue) string {
	for _, cf := range issue.CustomFields {
		if cf.Name == "Type" {
			if name := extractNameFromValue(cf.Value); name != "" {
				return strings.ToLower(name)
			}
		}
	}
	return "issue"
}

// ──────────────────────────────────────────────────────────────────────────────
// Helper functions for mapping custom field values
// ──────────────────────────────────────────────────────────────────────────────

// mapPriorityValue converts a custom field value to Priority
func mapPriorityValue(value interface{}) provider.Priority {
	s, ok := value.(map[string]interface{})
	if !ok {
		return provider.PriorityNormal
	}
	name, _ := s["name"].(string)
	switch strings.ToLower(name) {
	case "critical", "urgent", "show-stopper":
		return provider.PriorityCritical
	case "high", "major":
		return provider.PriorityHigh
	case "low", "minor":
		return provider.PriorityLow
	default:
		return provider.PriorityNormal
	}
}

// mapStatusValue converts a custom field value to Status
func mapStatusValue(value interface{}) provider.Status {
	s, ok := value.(map[string]interface{})
	if !ok {
		return provider.StatusOpen
	}
	name, _ := s["name"].(string)
	switch strings.ToLower(name) {
	case "open", "new", "submitted", "to be done":
		return provider.StatusOpen
	case "in progress", "inprogress", "active":
		return provider.StatusInProgress
	case "review", "code review", "verification":
		return provider.StatusReview
	case "fixed", "done", "completed", "verified", "resolved":
		return provider.StatusDone
	case "closed", "won't fix", "can't reproduce", "duplicate", "obsolete":
		return provider.StatusClosed
	default:
		return provider.StatusOpen
	}
}

// mapAssigneeValue converts a custom field value to Person slice
func mapAssigneeValue(value interface{}) []provider.Person {
	var result []provider.Person

	switch v := value.(type) {
	case map[string]interface{}:
		// Single assignee
		return []provider.Person{{
			ID:   getValue(v, "id"),
			Name: getValue(v, "name"),
		}}
	case []interface{}:
		// Multiple assignees
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				result = append(result, provider.Person{
					ID:   getValue(m, "id"),
					Name: getValue(m, "name"),
				})
			}
		}
	}

	if len(result) == 0 {
		return []provider.Person{}
	}
	return result
}

// extractNameFromValue extracts the "name" field from a custom field value
func extractNameFromValue(value interface{}) string {
	switch v := value.(type) {
	case map[string]interface{}:
		return getValue(v, "name")
	case string:
		return v
	}
	return ""
}

// getValue extracts a string value from a map by key
func getValue(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// formatIssueMarkdown formats an issue as markdown
func formatIssueMarkdown(issue *Issue, comments []Comment) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# %s\n\n", issue.IDReadable))
	sb.WriteString(fmt.Sprintf("## %s\n\n", issue.Summary))

	// Metadata
	sb.WriteString("### Metadata\n\n")
	sb.WriteString(fmt.Sprintf("- **Project:** %s (%s)\n", issue.Project.Name, issue.Project.ShortName))
	sb.WriteString(fmt.Sprintf("- **Reporter:** %s\n", issue.Reporter.FullName))
	sb.WriteString(fmt.Sprintf("- **Created:** %s\n", timeFromMillis(issue.Created).Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("- **Updated:** %s\n", timeFromMillis(issue.Updated).Format(time.RFC3339)))

	// Tags
	if len(issue.Tags) > 0 {
		tags := make([]string, len(issue.Tags))
		for i, t := range issue.Tags {
			tags[i] = t.Name
		}
		sb.WriteString(fmt.Sprintf("- **Tags:** %s\n", strings.Join(tags, ", ")))
	}

	// Custom fields summary
	sb.WriteString("\n### Fields\n\n")
	for _, cf := range issue.CustomFields {
		if name := extractNameFromValue(cf.Value); name != "" {
			sb.WriteString(fmt.Sprintf("- **%s:** %s\n", cf.Name, name))
		}
	}

	// Description
	if issue.Description != "" {
		sb.WriteString("\n### Description\n\n")
		sb.WriteString(issue.Description)
		sb.WriteString("\n")
	}

	// Comments
	if len(comments) > 0 {
		sb.WriteString("\n### Comments\n\n")
		for _, c := range comments {
			if c.Deleted {
				continue
			}
			sb.WriteString(fmt.Sprintf("#### %s - %s\n\n", c.Author.FullName,
				timeFromMillis(c.Created).Format(time.RFC3339)))
			sb.WriteString(c.Text)
			sb.WriteString("\n\n")
		}
	}

	return sb.String()
}
