package clickup

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/valksor/go-mehrhof/internal/provider"
)

// ProviderName is the canonical name for this provider.
const ProviderName = "clickup"

// Provider implements the ClickUp task provider.
type Provider struct {
	client *Client
	config *Config
}

// Config holds ClickUp provider configuration.
type Config struct {
	Token         string
	TeamID        string // Team/Workspace ID
	DefaultList   string // Default list ID for list operations
	BranchPattern string
	CommitPrefix  string
}

// Info returns provider metadata.
func Info() provider.ProviderInfo {
	return provider.ProviderInfo{
		Name:        ProviderName,
		Description: "Load tasks from ClickUp",
		Schemes:     []string{"clickup", "cu"},
		Capabilities: provider.CapabilitySet{
			provider.CapRead:           true,
			provider.CapList:           true,
			provider.CapFetchComments:  true,
			provider.CapComment:        true,
			provider.CapUpdateStatus:   true,
			provider.CapManageLabels:   true,
			provider.CapSnapshot:       true,
			provider.CapCreateWorkUnit: true,
			provider.CapFetchSubtasks:  true,
		},
	}
}

// New creates a new ClickUp provider instance.
func New(_ context.Context, cfg provider.Config) (any, error) {
	config := &Config{
		Token:         cfg.GetString("token"),
		TeamID:        cfg.GetString("team_id"),
		DefaultList:   cfg.GetString("default_list"),
		BranchPattern: cfg.GetString("branch_pattern"),
		CommitPrefix:  cfg.GetString("commit_prefix"),
	}

	// Resolve token
	token, err := ResolveToken(config.Token)
	if err != nil {
		return nil, err
	}

	client := NewClient(token)

	return &Provider{
		client: client,
		config: config,
	}, nil
}

// Match checks if the input looks like a ClickUp reference.
func (p *Provider) Match(input string) bool {
	// Check for clickup: or cu: prefix
	if strings.HasPrefix(input, "clickup:") || strings.HasPrefix(input, "cu:") {
		return true
	}

	// Check for ClickUp URL patterns
	if strings.Contains(input, "app.clickup.com") || strings.Contains(input, "sharing.clickup.com") {
		return true
	}

	// Check for bare task ID or custom ID pattern
	_, err := ParseReference(input)

	return err == nil
}

// Parse parses a ClickUp reference and returns a canonical ID.
func (p *Provider) Parse(input string) (string, error) {
	ref, err := ParseReference(input)
	if err != nil {
		return "", err
	}
	if ref.CustomID != "" {
		return ref.CustomID, nil
	}

	return ref.TaskID, nil
}

// Fetch retrieves a task by its ID.
func (p *Provider) Fetch(ctx context.Context, id string) (*provider.WorkUnit, error) {
	// Determine if this is a custom ID or task ID
	ref, err := ParseReference(id)
	if err != nil {
		return nil, fmt.Errorf("parse reference: %w", err)
	}

	var task *Task
	if ref.CustomID != "" && p.config.TeamID != "" {
		task, err = p.client.GetTaskByCustomID(ctx, p.config.TeamID, ref.CustomID)
	} else {
		taskID := ref.TaskID
		if taskID == "" {
			taskID = ref.CustomID
		}
		task, err = p.client.GetTask(ctx, taskID)
	}

	if err != nil {
		return nil, fmt.Errorf("fetch task %s: %w", id, err)
	}

	return p.taskToWorkUnit(task), nil
}

// Snapshot creates a snapshot of the task's current state.
func (p *Provider) Snapshot(ctx context.Context, id string) (*provider.Snapshot, error) {
	ref, err := ParseReference(id)
	if err != nil {
		return nil, fmt.Errorf("parse reference: %w", err)
	}

	var task *Task
	if ref.CustomID != "" && p.config.TeamID != "" {
		task, err = p.client.GetTaskByCustomID(ctx, p.config.TeamID, ref.CustomID)
	} else {
		taskID := ref.TaskID
		if taskID == "" {
			taskID = ref.CustomID
		}
		task, err = p.client.GetTask(ctx, taskID)
	}

	if err != nil {
		return nil, fmt.Errorf("snapshot task %s: %w", id, err)
	}

	// Build markdown content
	content := buildSnapshotContent(task)

	return &provider.Snapshot{
		Type:    ProviderName,
		Ref:     "clickup:" + task.ID,
		Content: content,
	}, nil
}

// List retrieves tasks from a list.
func (p *Provider) List(ctx context.Context, opts provider.ListOptions) ([]*provider.WorkUnit, error) {
	listID := p.config.DefaultList
	if listID == "" {
		return nil, ErrListRequired
	}

	// Include closed tasks based on status filter
	includeClosed := opts.Status == "" || opts.Status == provider.StatusClosed || opts.Status == provider.StatusDone

	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}

	tasks, err := p.client.ListTasks(ctx, listID, false, limit)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	var units []*provider.WorkUnit
	for _, task := range tasks {
		// Filter by status
		if opts.Status != "" {
			taskStatus := mapClickUpStatus(&task)
			if taskStatus != opts.Status && !statusMatches(taskStatus, opts.Status) {
				continue
			}
		}

		// Filter closed tasks if only looking for open
		if opts.Status == provider.StatusOpen && !includeClosed {
			if task.Status != nil && task.Status.Type == "closed" {
				continue
			}
		}

		// Filter by labels if specified
		if len(opts.Labels) > 0 && !hasAnyTag(task.Tags, opts.Labels) {
			continue
		}

		units = append(units, p.taskToWorkUnit(&task))
	}

	return units, nil
}

// FetchComments retrieves comments for a task.
func (p *Provider) FetchComments(ctx context.Context, id string) ([]*provider.Comment, error) {
	ref, err := ParseReference(id)
	if err != nil {
		return nil, fmt.Errorf("parse reference: %w", err)
	}

	taskID := ref.TaskID
	if taskID == "" {
		taskID = ref.CustomID
	}

	comments, err := p.client.GetTaskComments(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("fetch comments for %s: %w", id, err)
	}

	var result []*provider.Comment
	for _, comment := range comments {
		// Parse date
		createdAt := parseTimestamp(comment.Date)

		author := provider.Person{
			ID:    strconv.Itoa(comment.User.ID),
			Name:  comment.User.Username,
			Email: comment.User.Email,
		}

		result = append(result, &provider.Comment{
			ID:        comment.ID,
			Author:    author,
			Body:      comment.CommentText,
			CreatedAt: createdAt,
		})
	}

	return result, nil
}

// AddComment adds a comment to a task.
func (p *Provider) AddComment(ctx context.Context, id string, body string) error {
	ref, err := ParseReference(id)
	if err != nil {
		return fmt.Errorf("parse reference: %w", err)
	}

	taskID := ref.TaskID
	if taskID == "" {
		taskID = ref.CustomID
	}

	_, err = p.client.AddTaskComment(ctx, taskID, body)
	if err != nil {
		return fmt.Errorf("add comment to %s: %w", id, err)
	}

	return nil
}

// UpdateStatus updates the task status.
func (p *Provider) UpdateStatus(ctx context.Context, id string, status provider.Status) error {
	ref, err := ParseReference(id)
	if err != nil {
		return fmt.Errorf("parse reference: %w", err)
	}

	taskID := ref.TaskID
	if taskID == "" {
		taskID = ref.CustomID
	}

	// Map provider status to ClickUp status
	clickUpStatus := mapToClickUpStatus(status)
	if clickUpStatus == "" {
		return nil // No mapping for this status
	}

	_, err = p.client.UpdateTaskStatus(ctx, taskID, clickUpStatus)
	if err != nil {
		return fmt.Errorf("update task status %s: %w", id, err)
	}

	return nil
}

// --- Helper functions ---

func (p *Provider) taskToWorkUnit(task *Task) *provider.WorkUnit {
	unit := &provider.WorkUnit{
		ID:          task.ID,
		ExternalID:  task.ID,
		ExternalKey: task.ID,
		Provider:    ProviderName,
		Title:       task.Name,
		Description: task.Description,
		Status:      mapClickUpStatus(task),
		Priority:    mapClickUpPriority(task.Priority),
		TaskType:    mapTaskType(task),
		Labels:      extractTagNames(task.Tags),
		CreatedAt:   parseTimestamp(task.DateCreated),
		UpdatedAt:   parseTimestamp(task.DateUpdated),
		Source: provider.SourceInfo{
			Type:      ProviderName,
			Reference: "clickup:" + task.ID,
			SyncedAt:  time.Now(),
		},
		Metadata: map[string]any{
			"url":       task.URL,
			"team_id":   task.TeamID,
			"list_id":   getListID(task),
			"folder_id": getFolderID(task),
			"space_id":  getSpaceID(task),
		},
	}

	// Use custom ID if available
	if task.CustomID != "" {
		unit.ExternalKey = task.CustomID
	}

	// Set assignees
	for _, assignee := range task.Assignees {
		unit.Assignees = append(unit.Assignees, provider.Person{
			ID:    strconv.Itoa(assignee.ID),
			Name:  assignee.Username,
			Email: assignee.Email,
		})
	}

	// Set due date in metadata
	if task.DueDate != nil {
		unit.Metadata["due_date"] = time.UnixMilli(*task.DueDate).Format(time.RFC3339)
	}

	// Set time estimate in metadata
	if task.TimeEstimate != nil {
		unit.Metadata["time_estimate_ms"] = *task.TimeEstimate
	}

	// Set points in metadata
	if task.Points != nil {
		unit.Metadata["points"] = *task.Points
	}

	return unit
}

func mapClickUpStatus(task *Task) provider.Status {
	if task.Status == nil {
		return provider.StatusOpen
	}

	// Check status type
	switch task.Status.Type {
	case "closed", "done":
		return provider.StatusClosed
	case "open":
		return provider.StatusOpen
	}

	// Check status name
	statusLower := strings.ToLower(task.Status.Status)
	switch {
	case contains(statusLower, "done") || contains(statusLower, "complete") || contains(statusLower, "closed"):
		return provider.StatusClosed
	case contains(statusLower, "progress") || contains(statusLower, "doing") || contains(statusLower, "started"):
		return provider.StatusInProgress
	case contains(statusLower, "review") || contains(statusLower, "qa"):
		return provider.StatusReview
	case contains(statusLower, "todo") || contains(statusLower, "open") || contains(statusLower, "backlog"):
		return provider.StatusOpen
	}

	return provider.StatusOpen
}

func mapClickUpPriority(priority *Priority) provider.Priority {
	if priority == nil {
		return provider.PriorityNormal
	}

	switch strings.ToLower(priority.Priority) {
	case "urgent", "1":
		return provider.PriorityCritical
	case "high", "2":
		return provider.PriorityHigh
	case "normal", "3":
		return provider.PriorityNormal
	case "low", "4":
		return provider.PriorityLow
	}

	return provider.PriorityNormal
}

func mapToClickUpStatus(status provider.Status) string {
	switch status {
	case provider.StatusClosed, provider.StatusDone:
		return "complete"
	case provider.StatusInProgress:
		return "in progress"
	case provider.StatusOpen:
		return "to do"
	case provider.StatusReview:
		return "in review"
	default:
		return ""
	}
}

func statusMatches(taskStatus, filterStatus provider.Status) bool {
	// StatusDone and StatusClosed are equivalent
	if filterStatus == provider.StatusDone && taskStatus == provider.StatusClosed {
		return true
	}
	if filterStatus == provider.StatusClosed && taskStatus == provider.StatusDone {
		return true
	}

	return false
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

	return "task"
}

func extractTagNames(tags []Tag) []string {
	var names []string
	for _, tag := range tags {
		names = append(names, tag.Name)
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

func parseTimestamp(ts string) time.Time {
	if ts == "" {
		return time.Time{}
	}

	// Try millisecond timestamp
	if ms, err := strconv.ParseInt(ts, 10, 64); err == nil {
		return time.UnixMilli(ms)
	}

	// Try RFC3339
	if t, err := time.Parse(time.RFC3339, ts); err == nil {
		return t
	}

	return time.Time{}
}

func getListID(task *Task) string {
	if task.List != nil {
		return task.List.ID
	}

	return ""
}

func getFolderID(task *Task) string {
	if task.Folder != nil {
		return task.Folder.ID
	}

	return ""
}

func getSpaceID(task *Task) string {
	if task.Space != nil {
		return task.Space.ID
	}

	return ""
}

func buildSnapshotContent(task *Task) string {
	var sb strings.Builder

	// Title
	sb.WriteString("# ")
	sb.WriteString(task.Name)
	sb.WriteString("\n\n")

	// Metadata
	sb.WriteString("**ID:** ")
	sb.WriteString(task.ID)
	sb.WriteString("\n")

	if task.CustomID != "" {
		sb.WriteString("**Custom ID:** ")
		sb.WriteString(task.CustomID)
		sb.WriteString("\n")
	}

	if task.Status != nil {
		sb.WriteString("**Status:** ")
		sb.WriteString(task.Status.Status)
		sb.WriteString("\n")
	}

	if task.Priority != nil {
		sb.WriteString("**Priority:** ")
		sb.WriteString(task.Priority.Priority)
		sb.WriteString("\n")
	}

	if len(task.Assignees) > 0 {
		sb.WriteString("**Assignees:** ")
		var names []string
		for _, a := range task.Assignees {
			names = append(names, a.Username)
		}
		sb.WriteString(strings.Join(names, ", "))
		sb.WriteString("\n")
	}

	if task.DueDate != nil {
		sb.WriteString("**Due:** ")
		sb.WriteString(time.UnixMilli(*task.DueDate).Format("2006-01-02"))
		sb.WriteString("\n")
	}

	if len(task.Tags) > 0 {
		sb.WriteString("**Tags:** ")
		sb.WriteString(strings.Join(extractTagNames(task.Tags), ", "))
		sb.WriteString("\n")
	}

	if task.URL != "" {
		sb.WriteString("**URL:** ")
		sb.WriteString(task.URL)
		sb.WriteString("\n")
	}

	sb.WriteString("\n")

	// Description
	if task.Description != "" {
		sb.WriteString("## Description\n\n")
		sb.WriteString(task.Description)
		sb.WriteString("\n")
	}

	// Checklists
	if len(task.Checklists) > 0 {
		sb.WriteString("\n## Checklists\n\n")
		for _, cl := range task.Checklists {
			sb.WriteString("### ")
			sb.WriteString(cl.Name)
			sb.WriteString("\n\n")
			for _, item := range cl.Items {
				if item.Resolved {
					sb.WriteString("- [x] ")
				} else {
					sb.WriteString("- [ ] ")
				}
				sb.WriteString(item.Name)
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// GetBranchSuggestion returns a suggested branch name for the task.
func (p *Provider) GetBranchSuggestion(task *provider.WorkUnit) string {
	if p.config.BranchPattern == "" {
		// Default pattern
		return "task/" + task.ExternalKey
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
