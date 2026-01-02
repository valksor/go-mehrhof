package asana

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// ProviderName is the canonical name for this provider.
const ProviderName = "asana"

// Provider implements the Asana task provider.
type Provider struct {
	client *Client
	config *Config
}

// Config holds Asana provider configuration.
type Config struct {
	Token          string
	WorkspaceGID   string
	DefaultProject string // Default project GID for list operations
	BranchPattern  string
	CommitPrefix   string
}

// Info returns provider metadata.
func Info() provider.ProviderInfo {
	return provider.ProviderInfo{
		Name:        ProviderName,
		Description: "Load tasks from Asana projects",
		Schemes:     []string{"asana", "as"},
		Capabilities: provider.CapabilitySet{
			provider.CapRead:          true,
			provider.CapList:          true,
			provider.CapFetchComments: true,
			provider.CapComment:       true,
			provider.CapUpdateStatus:  true,
			provider.CapManageLabels:  true,
			provider.CapSnapshot:      true,
			provider.CapFetchSubtasks: true,
		},
	}
}

// New creates a new Asana provider instance.
func New(_ context.Context, cfg provider.Config) (any, error) {
	config := &Config{
		Token:          cfg.GetString("token"),
		WorkspaceGID:   cfg.GetString("workspace_gid"),
		DefaultProject: cfg.GetString("default_project"),
		BranchPattern:  cfg.GetString("branch_pattern"),
		CommitPrefix:   cfg.GetString("commit_prefix"),
	}

	// Resolve token
	token, err := ResolveToken(config.Token)
	if err != nil {
		return nil, err
	}

	client := NewClient(token, config.WorkspaceGID)

	return &Provider{
		client: client,
		config: config,
	}, nil
}

// Match checks if the input looks like an Asana reference.
func (p *Provider) Match(input string) bool {
	// Check for asana: or as: prefix
	if strings.HasPrefix(input, "asana:") || strings.HasPrefix(input, "as:") {
		return true
	}

	// Check for Asana URL pattern
	if strings.Contains(input, "app.asana.com") {
		return true
	}

	// Check for bare GID pattern (long numeric string)
	_, err := ParseReference(input)

	return err == nil
}

// Parse parses an Asana reference and returns a canonical ID.
func (p *Provider) Parse(input string) (string, error) {
	ref, err := ParseReference(input)
	if err != nil {
		return "", err
	}

	return ref.TaskGID, nil
}

// Fetch retrieves a task by its GID.
func (p *Provider) Fetch(ctx context.Context, id string) (*provider.WorkUnit, error) {
	task, err := p.client.GetTask(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("fetch task %s: %w", id, err)
	}

	return p.taskToWorkUnit(task), nil
}

// Snapshot creates a snapshot of the task's current state.
func (p *Provider) Snapshot(ctx context.Context, id string) (*provider.Snapshot, error) {
	task, err := p.client.GetTask(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("snapshot task %s: %w", id, err)
	}

	// Build markdown content
	content := buildSnapshotContent(task)

	return &provider.Snapshot{
		Type:    ProviderName,
		Ref:     "asana:" + id,
		Content: content,
	}, nil
}

// List retrieves tasks from a project.
func (p *Provider) List(ctx context.Context, opts provider.ListOptions) ([]*provider.WorkUnit, error) {
	projectGID := p.config.DefaultProject
	if projectGID == "" {
		return nil, ErrProjectRequired
	}

	// Filter by completion status
	var completedSince *time.Time
	if opts.Status == provider.StatusOpen || opts.Status == "" {
		// Show only incomplete tasks
		t := time.Now().Add(-365 * 24 * time.Hour) // Last year's completed tasks only
		completedSince = &t
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}

	tasks, err := p.client.ListProjectTasks(ctx, projectGID, completedSince, limit)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	var units []*provider.WorkUnit
	for _, task := range tasks {
		// Filter completed tasks if looking for open
		if opts.Status == provider.StatusOpen && task.Completed {
			continue
		}

		// Filter by labels if specified
		if len(opts.Labels) > 0 && !hasAnyTag(task.Tags, opts.Labels) {
			continue
		}

		units = append(units, p.taskToWorkUnit(&task))
	}

	return units, nil
}

// FetchComments retrieves comments (stories) for a task.
func (p *Provider) FetchComments(ctx context.Context, id string) ([]*provider.Comment, error) {
	stories, err := p.client.GetTaskStories(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("fetch comments for %s: %w", id, err)
	}

	var comments []*provider.Comment
	for _, story := range stories {
		// Only include comment-type stories
		if story.ResourceSubtype != "comment_added" {
			continue
		}

		author := provider.Person{}
		if story.CreatedBy != nil {
			author = provider.Person{
				Name:  story.CreatedBy.Name,
				Email: story.CreatedBy.Email,
			}
		}

		comments = append(comments, &provider.Comment{
			ID:        story.GID,
			Author:    author,
			Body:      story.Text,
			CreatedAt: story.CreatedAt,
		})
	}

	return comments, nil
}

// AddComment adds a comment to a task.
func (p *Provider) AddComment(ctx context.Context, id string, body string) error {
	_, err := p.client.AddTaskComment(ctx, id, body)
	if err != nil {
		return fmt.Errorf("add comment to %s: %w", id, err)
	}

	return nil
}

// UpdateStatus updates the task status (completes the task or moves to section).
func (p *Provider) UpdateStatus(ctx context.Context, id string, status provider.Status) error {
	switch status {
	case provider.StatusClosed, provider.StatusDone:
		_, err := p.client.CompleteTask(ctx, id)
		if err != nil {
			return fmt.Errorf("complete task %s: %w", id, err)
		}
	case provider.StatusOpen, provider.StatusInProgress, provider.StatusReview:
		// For these statuses, we could potentially move to sections
		// but this requires project context
		return nil
	}

	return nil
}

// --- Helper functions ---

func (p *Provider) taskToWorkUnit(task *Task) *provider.WorkUnit {
	unit := &provider.WorkUnit{
		ID:          task.GID,
		ExternalID:  task.GID,
		ExternalKey: task.GID,
		Provider:    ProviderName,
		Title:       task.Name,
		Description: task.Notes,
		Status:      mapAsanaStatus(task),
		Priority:    provider.PriorityNormal, // Asana doesn't have built-in priority
		TaskType:    mapTaskType(task),
		Labels:      extractTagNames(task.Tags),
		CreatedAt:   task.CreatedAt,
		UpdatedAt:   task.ModifiedAt,
		Source: provider.SourceInfo{
			Type:      ProviderName,
			Reference: "asana:" + task.GID,
			SyncedAt:  time.Now(),
		},
		Metadata: map[string]any{
			"permalink_url": task.PermalinkURL,
			"projects":      extractProjectNames(task.Projects),
		},
	}

	// Set assignees
	if task.Assignee != nil {
		unit.Assignees = []provider.Person{
			{
				Name:  task.Assignee.Name,
				Email: task.Assignee.Email,
			},
		}
	}

	// Set due date in metadata
	if task.DueOn != "" {
		unit.Metadata["due_on"] = task.DueOn
	}

	return unit
}

func mapAsanaStatus(task *Task) provider.Status {
	if task.Completed {
		return provider.StatusClosed
	}

	// Check approval status for approval tasks
	if task.ResourceSubtype == "approval" {
		switch task.ApprovalStatus {
		case "approved":
			return provider.StatusClosed
		case "rejected":
			return provider.StatusOpen
		case "pending":
			return provider.StatusInProgress
		}
	}

	// Check section for status (common Asana pattern)
	for _, membership := range task.Memberships {
		if membership.Section != nil {
			sectionName := strings.ToLower(membership.Section.Name)
			switch {
			case contains(sectionName, "done") || contains(sectionName, "complete"):
				return provider.StatusClosed
			case contains(sectionName, "progress") || contains(sectionName, "doing"):
				return provider.StatusInProgress
			case contains(sectionName, "review"):
				return provider.StatusReview
			case contains(sectionName, "blocked") || contains(sectionName, "hold"):
				return provider.StatusOpen // Blocked tasks are still open
			}
		}
	}

	return provider.StatusOpen
}

func mapTaskType(task *Task) string {
	// Check tags for type hints
	for _, tag := range task.Tags {
		tagLower := strings.ToLower(tag.Name)
		switch {
		case contains(tagLower, "bug") || contains(tagLower, "fix"):
			return "fix"
		case contains(tagLower, "feature") || contains(tagLower, "enhancement"):
			return "feature"
		case contains(tagLower, "chore") || contains(tagLower, "task"):
			return "task"
		case contains(tagLower, "doc"):
			return "docs"
		}
	}

	// Check subtask type
	if task.ResourceSubtype == "milestone" {
		return "milestone"
	}
	if task.ResourceSubtype == "approval" {
		return "approval"
	}

	return "task"
}

func extractTagNames(tags []Tag) []string {
	var names []string
	for _, tag := range tags {
		names = append(names, tag.Name)
	}

	return names
}

func extractProjectNames(projects []Project) []string {
	var names []string
	for _, proj := range projects {
		names = append(names, proj.Name)
	}

	return names
}

func hasAnyTag(tags []Tag, tagNames []string) bool {
	for _, tag := range tags {
		tagLower := strings.ToLower(tag.Name)
		for _, name := range tagNames {
			if strings.ToLower(name) == tagLower {
				return true
			}
		}
	}

	return false
}

func buildSnapshotContent(task *Task) string {
	var sb strings.Builder

	// Title
	sb.WriteString("# ")
	sb.WriteString(task.Name)
	sb.WriteString("\n\n")

	// Metadata
	sb.WriteString("**GID:** ")
	sb.WriteString(task.GID)
	sb.WriteString("\n")

	if task.Assignee != nil {
		sb.WriteString("**Assignee:** ")
		sb.WriteString(task.Assignee.Name)
		sb.WriteString("\n")
	}

	if task.DueOn != "" {
		sb.WriteString("**Due:** ")
		sb.WriteString(task.DueOn)
		sb.WriteString("\n")
	}

	if len(task.Tags) > 0 {
		sb.WriteString("**Tags:** ")
		sb.WriteString(strings.Join(extractTagNames(task.Tags), ", "))
		sb.WriteString("\n")
	}

	if len(task.Projects) > 0 {
		sb.WriteString("**Projects:** ")
		sb.WriteString(strings.Join(extractProjectNames(task.Projects), ", "))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")

	// Description
	if task.Notes != "" {
		sb.WriteString("## Description\n\n")
		sb.WriteString(task.Notes)
		sb.WriteString("\n")
	}

	return sb.String()
}

// GetBranchSuggestion returns a suggested branch name for the task.
func (p *Provider) GetBranchSuggestion(task *provider.WorkUnit) string {
	if p.config.BranchPattern == "" {
		// Default pattern
		return "task/" + task.ID
	}

	// Simple template replacement
	result := p.config.BranchPattern
	result = strings.ReplaceAll(result, "{key}", task.ExternalKey)
	result = strings.ReplaceAll(result, "{id}", task.ID)

	// Slugify title
	slug := slugify(task.Title)
	result = strings.ReplaceAll(result, "{slug}", slug)

	return result
}

func slugify(s string) string {
	// Simple slugification
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		if r == ' ' || r == '-' || r == '_' {
			return '-'
		}

		return -1
	}, s)

	// Remove consecutive hyphens
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")

	// Truncate
	if len(s) > 50 {
		s = s[:50]
		s = strings.TrimRight(s, "-")
	}

	return s
}
