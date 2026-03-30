package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const (
	maxJiraSiblings = 5

	priorityCritical = "critical"
	priorityHigh     = "high"
	priorityNormal   = "normal"
	priorityLow      = "low"

	statusDone     = "done"
	statusClosed   = "closed"
	statusResolved = "resolved"
)

// JiraProvider implements Provider, HierarchyProvider, CommentProvider, and
// SubmitProvider for Jira issues via the REST API v3.
type JiraProvider struct {
	client *JiraClient
}

// NewJiraProvider creates a new Jira provider.
func NewJiraProvider(baseURL, email, token string) *JiraProvider {
	return &JiraProvider{
		client: NewJiraClient(baseURL, email, token),
	}
}

func (p *JiraProvider) Name() string {
	return NameJira
}

// --- Provider interface ---

// FetchTask fetches a Jira issue by key (e.g., "PROJ-123").
func (p *JiraProvider) FetchTask(ctx context.Context, ref string) (*Task, error) {
	if p.client.token == "" {
		return nil, errors.New("JIRA_TOKEN not set")
	}

	key := normalizeJiraKey(ref)

	issue, err := p.client.GetIssue(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("jira: fetch task: %w", err)
	}

	return p.issueToTask(issue), nil
}

// UpdateStatus updates the status of a Jira issue by performing a transition.
// The status string is matched against available transition names (case-insensitive).
func (p *JiraProvider) UpdateStatus(ctx context.Context, ref string, status string) error {
	if p.client.token == "" {
		return errors.New("JIRA_TOKEN not set")
	}

	key := normalizeJiraKey(ref)

	transitions, err := p.client.GetIssueTransitions(ctx, key)
	if err != nil {
		return fmt.Errorf("jira: get transitions: %w", err)
	}

	statusLower := strings.ToLower(status)
	for _, t := range transitions {
		if strings.ToLower(t.Name) == statusLower {
			return p.client.TransitionIssue(ctx, key, t.ID)
		}
	}

	return fmt.Errorf("jira: no matching transition for status %q on %s", status, key)
}

// --- HierarchyProvider interface ---

// FetchParent returns the parent issue if this issue has one.
func (p *JiraProvider) FetchParent(ctx context.Context, task *Task) (*Task, error) {
	if p.client.token == "" {
		return nil, errors.New("JIRA_TOKEN not set")
	}

	parentKey := task.Metadata("jira_parent_key")
	if parentKey == "" {
		return nil, nil //nolint:nilnil // nil, nil signals "no parent" (not an error)
	}

	issue, err := p.client.GetIssue(ctx, parentKey)
	if err != nil {
		return nil, fmt.Errorf("jira: fetch parent: %w", err)
	}

	return p.issueToTask(issue), nil
}

// FetchSiblings returns sibling issues (subtasks of the same parent).
func (p *JiraProvider) FetchSiblings(ctx context.Context, task *Task) ([]*Task, error) {
	if p.client.token == "" {
		return nil, errors.New("JIRA_TOKEN not set")
	}

	parentKey := task.Metadata("jira_parent_key")
	if parentKey == "" {
		return nil, nil
	}

	// Fetch the parent to get its subtasks
	parentIssue, err := p.client.GetIssue(ctx, parentKey)
	if err != nil {
		return nil, fmt.Errorf("jira: fetch parent for siblings: %w", err)
	}

	siblings := make([]*Task, 0, maxJiraSiblings)
	for _, sub := range parentIssue.Fields.Subtasks {
		if sub.Key == task.ID {
			continue // Skip self
		}
		siblings = append(siblings, p.issueToTask(&sub))
		if len(siblings) >= maxJiraSiblings {
			break
		}
	}

	return siblings, nil
}

// --- SubmitProvider interface ---

// AddComment posts a comment on a Jira issue.
func (p *JiraProvider) AddComment(ctx context.Context, ref string, body string) error {
	if p.client.token == "" {
		return errors.New("JIRA_TOKEN not set")
	}

	key := normalizeJiraKey(ref)

	return p.client.AddComment(ctx, key, body)
}

// CreatePR is not supported by Jira (Jira is an issue tracker, not a code host).
// Returns an error indicating this.
func (p *JiraProvider) CreatePR(_ context.Context, _ PROptions) (*PRResult, error) {
	return nil, errors.New("jira: CreatePR not supported (Jira is not a code host)")
}

// --- internal helpers ---

// issueToTask converts a Jira issue to a Task.
func (p *JiraProvider) issueToTask(issue *jiraIssue) *Task {
	labels := make([]string, 0, len(issue.Fields.Labels)+1)
	labels = append(labels, issue.Fields.Labels...)

	// Add status as a label
	if issue.Fields.Status != nil {
		labels = append(labels, issue.Fields.Status.Name)
	}

	description := extractADFText(issue.Fields.Description)

	task := &Task{
		ID:          issue.Key,
		Title:       issue.Fields.Summary,
		Description: description,
		URL:         fmt.Sprintf("%s/browse/%s", p.client.baseURL, issue.Key),
		Labels:      labels,
		Source:      NameJira,
	}

	// Inference
	task.Priority, task.Type, task.Slug = InferAll(task.Title, labels)

	// Override priority from Jira if set
	if issue.Fields.Priority != nil {
		task.Priority = jiraPriorityToString(issue.Fields.Priority.Name)
	}

	// Subtasks
	for i, sub := range issue.Fields.Subtasks {
		completed := false
		if sub.Fields.Status != nil {
			name := strings.ToLower(sub.Fields.Status.Name)
			completed = name == statusDone || name == statusClosed || name == statusResolved
		}
		task.Subtasks = append(task.Subtasks, &Subtask{
			ID:        sub.Key,
			Text:      sub.Fields.Summary,
			Completed: completed,
			Index:     i,
		})
	}

	// Metadata
	task.SetMetadata("jira_id", issue.ID)
	task.SetMetadata("jira_key", issue.Key)
	if issue.Fields.Status != nil {
		task.SetMetadata("jira_status", issue.Fields.Status.Name)
	}
	if issue.Fields.IssueType != nil {
		task.SetMetadata("jira_issue_type", issue.Fields.IssueType.Name)
	}
	if issue.Fields.Parent != nil {
		task.SetMetadata("jira_parent_key", issue.Fields.Parent.Key)
		task.SetMetadata("jira_parent_id", issue.Fields.Parent.ID)
	}

	return task
}

// extractADFText extracts plain text from an Atlassian Document Format (ADF)
// document. If the description is already a plain string, it returns it directly.
// ADF is a JSON structure with nested content nodes; we extract text from
// paragraph, heading, and other block-level nodes.
func extractADFText(desc any) string {
	if desc == nil {
		return ""
	}

	// Plain string — return directly
	if s, ok := desc.(string); ok {
		return s
	}

	// ADF document (map)
	doc, ok := desc.(map[string]any)
	if !ok {
		return ""
	}

	content, ok := doc["content"]
	if !ok {
		return ""
	}

	contentSlice, ok := content.([]any)
	if !ok {
		return ""
	}

	var b strings.Builder
	extractADFContent(&b, contentSlice)

	return strings.TrimSpace(b.String())
}

// extractADFContent recursively extracts text from ADF content nodes.
func extractADFContent(b *strings.Builder, nodes []any) {
	for _, node := range nodes {
		nodeMap, ok := node.(map[string]any)
		if !ok {
			continue
		}

		// If this node has a "text" field, write it
		if text, ok := nodeMap["text"].(string); ok {
			b.WriteString(text)
		}

		// Recurse into child content
		if childContent, ok := nodeMap["content"].([]any); ok {
			extractADFContent(b, childContent)
		}

		// Add newline after block-level nodes
		nodeType, _ := nodeMap["type"].(string)
		switch nodeType {
		case "paragraph", "heading", "bulletList", "orderedList", "blockquote",
			"codeBlock", "rule", "listItem":
			b.WriteString("\n")
		}
	}
}

// normalizeJiraKey cleans up a Jira issue key reference.
func normalizeJiraKey(ref string) string {
	ref = strings.TrimPrefix(ref, "jira:")

	return strings.ToUpper(strings.TrimSpace(ref))
}

// jiraPriorityToString maps Jira priority names to normalized priority strings.
func jiraPriorityToString(name string) string {
	switch strings.ToLower(name) {
	case "highest", "blocker", priorityCritical:
		return priorityCritical
	case priorityHigh:
		return priorityHigh
	case "medium":
		return priorityNormal
	case priorityLow:
		return priorityLow
	case "lowest":
		return priorityLow
	default:
		return priorityNormal
	}
}
