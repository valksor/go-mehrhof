package wrike

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/valksor/go-mehrhof/internal/naming"
	"github.com/valksor/go-mehrhof/internal/provider"
)

const (
	// ProviderName is the name of the Wrike provider
	ProviderName = "wrike"
)

// Provider implements the Wrike task provider
type Provider struct {
	client *Client
}

// New creates a new Wrike provider instance
func New(_ context.Context, cfg provider.Config) (any, error) {
	token := cfg.GetString("token")
	host := cfg.GetString("host")

	// Try to resolve token from env if not provided
	if token == "" {
		resolvedToken, err := ResolveToken("")
		if err != nil {
			return nil, err
		}
		token = resolvedToken
	}

	return &Provider{
		client: NewClient(token, host),
	}, nil
}

// Match checks if the input matches a Wrike reference
func (p *Provider) Match(input string) bool {
	input = strings.TrimSpace(input)
	return strings.HasPrefix(input, "wrike:") ||
		strings.HasPrefix(input, "wk:") ||
		permalinkPattern.MatchString(input) ||
		apiIDPattern.MatchString(input) ||
		numericIDPattern.MatchString(input)
}

// Parse extracts the task ID from a Wrike reference
func (p *Provider) Parse(input string) (string, error) {
	input = strings.TrimSpace(input)

	// Strip scheme prefix if present
	schemeStripped := strings.TrimPrefix(input, "wrike:")
	schemeStripped = strings.TrimPrefix(schemeStripped, "wk:")

	// Check for permalink - extract numeric ID
	if matches := permalinkPattern.FindStringSubmatch(input); matches != nil {
		return matches[1], nil
	}

	// Use scheme-stripped version for remaining checks
	taskID := schemeStripped

	// Validate that it's a valid ID format
	if !apiIDPattern.MatchString(taskID) && !numericIDPattern.MatchString(taskID) {
		return "", fmt.Errorf("%w: invalid Wrike reference: %s", ErrInvalidReference, input)
	}

	return taskID, nil
}

// Fetch retrieves a task from Wrike and converts it to a WorkUnit
func (p *Provider) Fetch(ctx context.Context, id string) (*provider.WorkUnit, error) {
	// First try to fetch as a direct task ID
	task, err := p.client.GetTask(ctx, id)
	if err != nil {
		// If that fails, try as permalink
		task, err = p.client.GetTaskByPermalink(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("fetch task: %w", err)
		}
	}

	// Fetch comments
	comments, err := p.client.GetComments(ctx, task.ID)
	if err != nil {
		// Log error but continue - comments are optional
		comments = nil
	}

	// Fetch attachments
	attachments, err := p.client.GetAttachments(ctx, task.ID)
	if err != nil {
		// Log error but continue - attachments are optional
		attachments = nil
	}

	// Fetch subtasks
	var subtaskInfos []SubtaskInfo
	if len(task.SubTaskIDs) > 0 {
		var err error
		subtaskInfos, _, err = p.fetchSubtasks(ctx, task.SubTaskIDs, 0)
		_ = err // Subtasks are optional, ignore fetch errors
	}

	// Extract numeric ID from permalink for ExternalKey
	numericID := ExtractNumericID(task.Permalink)
	if numericID == "" {
		numericID = task.ID
	}

	// Build WorkUnit
	wu := &provider.WorkUnit{
		ID:          numericID,
		ExternalID:  task.ID,
		Provider:    ProviderName,
		Title:       task.Title,
		Description: task.Description,
		Status:      mapStatus(task.Status),
		Priority:    mapPriority(task.Priority),
		Labels:      []string{},
		Assignees:   []provider.Person{},
		Comments:    mapComments(comments),
		Attachments: mapAttachments(attachments),
		Subtasks:    task.SubTaskIDs,
		Metadata:    buildMetadata(task, subtaskInfos),
		CreatedAt:   task.CreatedDate,
		UpdatedAt:   task.UpdatedDate,
		Source: provider.SourceInfo{
			Type:      ProviderName,
			Reference: task.Permalink,
			SyncedAt:  time.Now(),
		},
		ExternalKey: numericID,
		TaskType:    "task",
		Slug:        naming.Slugify(task.Title, 50),
	}

	return wu, nil
}

// Snapshot captures the task content from Wrike
func (p *Provider) Snapshot(ctx context.Context, id string) (*provider.Snapshot, error) {
	task, err := p.client.GetTask(ctx, id)
	if err != nil {
		task, err = p.client.GetTaskByPermalink(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("fetch task for snapshot: %w", err)
		}
	}

	comments, _ := p.client.GetComments(ctx, task.ID)

	var content strings.Builder
	content.WriteString(fmt.Sprintf("# %s\n\n", task.Title))
	content.WriteString(fmt.Sprintf("**Status:** %s\n", task.Status))
	content.WriteString(fmt.Sprintf("**Priority:** %s\n", task.Priority))
	content.WriteString(fmt.Sprintf("**Permalink:** %s\n\n", task.Permalink))

	if task.Description != "" {
		content.WriteString("## Description\n\n")
		content.WriteString(task.Description)
		content.WriteString("\n\n")
	}

	if len(comments) > 0 {
		content.WriteString("## Comments\n\n")
		for _, c := range comments {
			content.WriteString(fmt.Sprintf("### %s - %s\n\n", c.AuthorName, c.CreatedDate.Format(time.RFC3339)))
			content.WriteString(c.Text)
			content.WriteString("\n\n")
		}
	}

	return &provider.Snapshot{
		Type:    "file",
		Ref:     id,
		Content: content.String(),
	}, nil
}

// ListTasks returns tasks from a folder or space
// scopeType should be "folder" or "space"
func (p *Provider) ListTasks(ctx context.Context, scopeType, scopeID string) ([]*provider.WorkUnit, error) {
	var tasks []Task
	var err error

	switch scopeType {
	case "folder":
		tasks, err = p.client.GetTasksInFolder(ctx, scopeID)
	case "space":
		tasks, err = p.client.GetTasksInSpace(ctx, scopeID)
	default:
		return nil, fmt.Errorf("invalid scope type: %s (use 'folder' or 'space')", scopeType)
	}

	if err != nil {
		return nil, err
	}

	result := make([]*provider.WorkUnit, 0, len(tasks))
	for _, task := range tasks {
		wu := p.taskToWorkUnit(&task)
		result = append(result, wu)
	}

	return result, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Helper functions
// ──────────────────────────────────────────────────────────────────────────────

// mapStatus converts Wrike status to provider status
func mapStatus(status string) provider.Status {
	switch strings.ToLower(status) {
	case "active", "new", "in progress", "inprogress", "draft":
		return provider.StatusOpen
	case "completed", "done":
		return provider.StatusDone
	case "cancelled", "canceled", "deferred", "closed":
		return provider.StatusClosed
	case "review":
		return provider.StatusReview
	default:
		return provider.StatusOpen
	}
}

// mapPriority converts Wrike priority to provider priority
func mapPriority(priority string) provider.Priority {
	switch strings.ToLower(priority) {
	case "critical", "urgent":
		return provider.PriorityCritical
	case "high":
		return provider.PriorityHigh
	case "low":
		return provider.PriorityLow
	default:
		return provider.PriorityNormal
	}
}

// mapComments converts Wrike comments to provider comments
func mapComments(comments []Comment) []provider.Comment {
	if comments == nil {
		return nil
	}

	result := make([]provider.Comment, 0, len(comments))
	for _, c := range comments {
		author := provider.Person{
			ID:   c.AuthorID,
			Name: c.AuthorName,
		}
		result = append(result, provider.Comment{
			ID:        c.ID,
			Author:    author,
			Body:      c.Text,
			CreatedAt: c.CreatedDate,
			UpdatedAt: c.UpdatedDate,
		})
	}
	return result
}

// buildMetadata creates metadata map from task and subtasks
func buildMetadata(task *Task, subtasks []SubtaskInfo) map[string]any {
	metadata := make(map[string]any)

	metadata["permalink"] = task.Permalink
	metadata["api_id"] = task.ID
	metadata["wrike_status"] = task.Status
	metadata["wrike_priority"] = task.Priority

	if len(subtasks) > 0 {
		// Convert subtasks to a simpler format for JSON serialization
		subtaskList := make([]map[string]string, 0, len(subtasks))
		for _, st := range subtasks {
			subtaskList = append(subtaskList, map[string]string{
				"id":     st.ID,
				"title":  st.Title,
				"status": st.Status,
			})
		}
		metadata["subtasks"] = subtaskList
		metadata["subtask_count"] = len(subtasks)
	}

	return metadata
}

// taskToWorkUnit converts a Task to a WorkUnit without fetching nested data
// Used by ListTasks for efficiency when listing multiple tasks
func (p *Provider) taskToWorkUnit(task *Task) *provider.WorkUnit {
	// Extract numeric ID from permalink for ExternalKey
	numericID := ExtractNumericID(task.Permalink)
	if numericID == "" {
		numericID = task.ID
	}

	return &provider.WorkUnit{
		ID:          numericID,
		ExternalID:  task.ID,
		Provider:    ProviderName,
		Title:       task.Title,
		Description: task.Description,
		Status:      mapStatus(task.Status),
		Priority:    mapPriority(task.Priority),
		Labels:      []string{},
		Assignees:   []provider.Person{},
		Comments:    nil,
		Attachments: nil,
		Subtasks:    task.SubTaskIDs,
		Metadata:    buildMetadata(task, nil),
		CreatedAt:   task.CreatedDate,
		UpdatedAt:   task.UpdatedDate,
		Source: provider.SourceInfo{
			Type:      ProviderName,
			Reference: task.Permalink,
			SyncedAt:  time.Now(),
		},
		ExternalKey: numericID,
		TaskType:    "task",
		Slug:        naming.Slugify(task.Title, 50),
	}
}
