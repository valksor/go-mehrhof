package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	linearAPIURL      = "https://api.linear.app/graphql"
	maxLinearSiblings = 5
)

// linearAllowedAttachmentHosts defines hosts from which attachments can be downloaded.
// Linear stores attachments on these CDN domains.
var linearAllowedAttachmentHosts = map[string]bool{
	"uploads.linear.app": true,
	"cdn.linear.app":     true,
}

// linearAllowedGCSPrefixes defines allowed GCS bucket path prefixes for Linear attachments.
var linearAllowedGCSPrefixes = []string{
	"/uploads.linear.app",
	"/public.linear.app",
	"/imports.linear.app",
	"/linear-uploads-europe-west1",
	"/linear-imports-europe-west1",
}

// isAllowedLinearAttachmentURL validates that a URL is from an allowed Linear attachment host.
func isAllowedLinearAttachmentURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	host := strings.ToLower(parsed.Hostname())

	// Direct Linear CDN hosts
	if linearAllowedAttachmentHosts[host] {
		return nil
	}

	// Google Cloud Storage with allowed prefixes
	if host == "storage.googleapis.com" {
		for _, prefix := range linearAllowedGCSPrefixes {
			if strings.HasPrefix(parsed.Path, prefix) {
				return nil
			}
		}
	}

	return fmt.Errorf("attachment host not allowed: %s", host)
}

// LinearProvider implements Provider, HierarchyProvider, CommentProvider,
// LabelProvider, ListProvider, CreateProvider, and AttachmentProvider
// for Linear.app issues.
type LinearProvider struct {
	token string
	team  string // default team key (optional)
}

// NewLinearProvider creates a new Linear provider.
// Token should come from Settings (settings.Providers.Linear.Token).
func NewLinearProvider(token, team string) *LinearProvider {
	return &LinearProvider{
		token: token,
		team:  team,
	}
}

func (p *LinearProvider) Name() string {
	return NameLinear
}

// --- internal types for GraphQL responses ---

type linearIssue struct {
	ID          string          `json:"id"`
	Identifier  string          `json:"identifier"` // "ENG-123"
	Title       string          `json:"title"`
	Description string          `json:"description"`
	URL         string          `json:"url"`
	Priority    int             `json:"priority"`
	State       *linearState    `json:"state"`
	Team        *linearTeam     `json:"team"`
	Parent      *linearParent   `json:"parent"`
	Labels      *linearLabels   `json:"labels"`
	Assignee    *linearUser     `json:"assignee"`
	Children    *linearChildren `json:"children"`
}

type linearState struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"` // backlog, unstarted, started, completed, canceled
}

type linearTeam struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

type linearParent struct {
	ID         string `json:"id"`
	Identifier string `json:"identifier"`
}

type linearLabels struct {
	Nodes []linearLabel `json:"nodes"`
}

type linearLabel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type linearUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type linearChildren struct {
	Nodes []linearIssue `json:"nodes"`
}

type linearComment struct {
	ID        string      `json:"id"`
	Body      string      `json:"body"`
	User      *linearUser `json:"user"`
	CreatedAt string      `json:"createdAt"`
}

type linearComments struct {
	Nodes []linearComment `json:"nodes"`
}

// issueToTask converts a Linear issue to a Task.
func (p *LinearProvider) issueToTask(issue *linearIssue) *Task {
	labels := make([]string, 0)
	if issue.Labels != nil {
		for _, l := range issue.Labels.Nodes {
			labels = append(labels, l.Name)
		}
	}

	// Add state as a label
	if issue.State != nil {
		labels = append(labels, issue.State.Name)
	}

	task := &Task{
		ID:          issue.Identifier,
		Title:       issue.Title,
		Description: issue.Description,
		URL:         issue.URL,
		Labels:      labels,
		Source:      NameLinear,
	}

	// Inference
	task.Priority, task.Type, task.Slug = InferAll(task.Title, labels)

	// Override priority from Linear if set
	if issue.Priority > 0 {
		task.Priority = linearPriorityToString(issue.Priority)
	}

	// Subtasks from children
	if issue.Children != nil {
		for i, child := range issue.Children.Nodes {
			completed := false
			if child.State != nil {
				completed = child.State.Type == "completed" || child.State.Type == "canceled"
			}
			task.Subtasks = append(task.Subtasks, &Subtask{
				ID:        child.Identifier,
				Text:      child.Title,
				Completed: completed,
				Index:     i,
			})
		}
	}

	// Dependencies (parsed from description)
	task.Dependencies = p.resolveDependencies(task)

	// Metadata
	task.SetMetadata("linear_id", issue.ID)
	task.SetMetadata("linear_identifier", issue.Identifier)
	if issue.State != nil {
		task.SetMetadata("linear_state_id", issue.State.ID)
		task.SetMetadata("linear_state_type", issue.State.Type)
	}
	if issue.Team != nil {
		task.SetMetadata("linear_team_key", issue.Team.Key)
		task.SetMetadata("linear_team_id", issue.Team.ID)
	}
	if issue.Parent != nil {
		task.SetMetadata("linear_parent_id", issue.Parent.ID)
		task.SetMetadata("linear_parent_identifier", issue.Parent.Identifier)
	}
	if issue.Assignee != nil {
		task.SetMetadata("linear_assignee", issue.Assignee.Name)
	}

	return task
}

// resolveDependencies parses dependency references from description.
func (p *LinearProvider) resolveDependencies(task *Task) []*Task {
	refs := ParseDependencies(task.Description)
	if len(refs) == 0 {
		return nil
	}

	deps := make([]*Task, 0, len(refs))
	for _, ref := range refs {
		depID := ref
		// Handle shorthand refs (e.g., "ENG-123" without prefix)
		if !strings.Contains(ref, "-") {
			continue // Not a valid Linear reference
		}
		deps = append(deps, &Task{
			ID:     depID,
			Source: NameLinear,
		})
	}

	return deps
}

// Priority conversion helpers

func linearPriorityToString(p int) string {
	switch p {
	case 1:
		return priorityCritical
	case 2:
		return priorityHigh
	case 3:
		return priorityNormal
	case 4:
		return priorityLow
	default:
		return priorityNormal
	}
}

// DownloadAttachment downloads an attachment from Linear.
// Linear stores attachments on approved CDN hosts; this validates the URL and adds auth.
func (p *LinearProvider) DownloadAttachment(ctx context.Context, attachmentURL string) ([]byte, error) {
	if p.token == "" {
		return nil, errors.New("LINEAR_TOKEN not set")
	}

	// Validate URL is from an allowed Linear attachment host
	if err := isAllowedLinearAttachmentURL(attachmentURL); err != nil {
		return nil, fmt.Errorf("validate attachment URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, attachmentURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Linear API: Personal API keys (lin_api_*) are used directly without prefix.
	// OAuth tokens should include "Bearer " prefix in the settings configuration.
	req.Header.Set("Authorization", p.token)

	resp, err := DoWithRetry(httpClient, req, DefaultRetryConfig)
	if err != nil {
		return nil, fmt.Errorf("download attachment: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download attachment: status %d", resp.StatusCode)
	}

	// Limit attachment size to prevent OOM on very large files (100MB)
	const maxAttachmentSize = 100 * 1024 * 1024
	limitedReader := io.LimitReader(resp.Body, maxAttachmentSize+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("read attachment: %w", err)
	}
	if len(data) > maxAttachmentSize {
		return nil, fmt.Errorf("attachment exceeds max size (%d bytes)", maxAttachmentSize)
	}

	return data, nil
}

// Priority conversion helpers

func linearPriorityFromString(s string) int {
	switch strings.ToLower(s) {
	case "critical", "urgent":
		return 1
	case "high":
		return 2
	case "normal", "medium":
		return 3
	case "low":
		return 4
	default:
		return 0 // No priority
	}
}
