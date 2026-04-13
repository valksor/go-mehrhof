package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

// maxSiblingTasks is the maximum number of sibling tasks included in context
// to keep AI prompts concise.
const maxSiblingTasks = 5

// WrikeProvider fetches tasks from Wrike.
// It implements both Provider and HierarchyProvider so callers can enrich a
// fetched task with parent and sibling context.
type WrikeProvider struct {
	token string
}

// NewWrikeProvider creates a new Wrike provider.
// Token should come from Settings (settings.Providers.Wrike.Token).
func NewWrikeProvider(token string) *WrikeProvider {
	return &WrikeProvider{
		token: token,
	}
}

func (p *WrikeProvider) Name() string {
	return "wrike"
}

// wrikeTaskData is the raw shape returned by the Wrike v4 tasks API.
// Field names match the Wrike API v4 JSON response exactly.
type wrikeTaskData struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Permalink   string `json:"permalink"`
	Status      string `json:"status"`
	// ParentIDs contains the IDs of the parent folders the task lives in.
	// A task can appear in multiple folders; the first entry is the primary parent.
	ParentIDs []string `json:"parentIds"`
	// SuperParentIDs contains the top-level folder (space/folder hierarchy) IDs.
	// Used as a fallback when ParentIDs is empty for sibling lookup.
	SuperParentIDs []string `json:"superParentIds"`
	// SuperTaskIDs contains the parent task IDs when this task is a subtask.
	// This is distinct from ParentIDs (which are folders, not tasks).
	// Use SuperTaskIDs to navigate the task hierarchy (subtask → parent task).
	SuperTaskIDs []string `json:"superTaskIds"`
	// SubTaskIDs contains the IDs of child tasks (subtasks) of this task.
	SubTaskIDs []string `json:"subTaskIds"`
}

// FetchTask fetches a single task by its Wrike task ID.
func (p *WrikeProvider) FetchTask(ctx context.Context, id string) (*Task, error) {
	if p.token == "" {
		return nil, errors.New("WRIKE_TOKEN not set")
	}

	slog.Debug("wrike: fetching task", "id", id)
	data, err := p.fetchTaskByID(ctx, id)
	if err != nil {
		slog.Error("wrike: fetch task failed", "id", id, "error", err)

		return nil, err
	}

	slog.Debug("wrike: task fetched", "id", id, "title", data.Title)

	return p.taskDataToTask(data), nil
}

// FetchParent returns the parent task for the given task.
//
// Wrike distinguishes between folder parents (parentIds) and task parents
// (superTaskIds). For task hierarchy (subtask → parent task), we use
// superTaskIds. Returns nil (no error) when the task has no parent task.
func (p *WrikeProvider) FetchParent(ctx context.Context, task *Task) (*Task, error) {
	if p.token == "" {
		return nil, errors.New("WRIKE_TOKEN not set")
	}

	// superTaskIds identifies the parent task (task hierarchy, not folder hierarchy).
	// This is the correct field for subtask → parent task navigation.
	parentTaskID := task.Metadata("wrike_super_task_id")
	if parentTaskID == "" {
		// No parent task recorded — task is not a subtask.
		return nil, nil //nolint:nilnil // Documented API: nil, nil signals "task has no parent" (not an error state)
	}

	data, err := p.fetchTaskByID(ctx, parentTaskID)
	if err != nil {
		return nil, fmt.Errorf("fetch parent task %s: %w", parentTaskID, err)
	}

	return p.taskDataToTask(data), nil
}

// FetchSiblings returns up to maxSiblingTasks sibling tasks that share the
// same parent folder as the given task.
// Returns nil (no error) when there is no parent folder or no siblings.
//
// Uses GET /folders/{folderId}/tasks to list tasks in the parent folder.
// The parent folder is taken from parentIds (the first folder the task lives in).
func (p *WrikeProvider) FetchSiblings(ctx context.Context, task *Task) ([]*Task, error) {
	if p.token == "" {
		return nil, errors.New("WRIKE_TOKEN not set")
	}

	// Use the folder parent ID for siblings (not the super task ID).
	// parentIds are folder IDs — sibling tasks share the same folder.
	parentFolderID := task.Metadata("wrike_parent_folder_id")
	if parentFolderID == "" {
		return nil, nil
	}

	tasks, err := p.fetchTasksInFolder(ctx, parentFolderID)
	if err != nil {
		return nil, fmt.Errorf("fetch siblings from folder %s: %w", parentFolderID, err)
	}

	// Filter out the task itself and cap results.
	siblings := make([]*Task, 0, maxSiblingTasks)
	for _, t := range tasks {
		if t.ID == task.ID {
			continue
		}
		siblings = append(siblings, t)
		if len(siblings) >= maxSiblingTasks {
			break
		}
	}

	return siblings, nil
}

func (p *WrikeProvider) UpdateStatus(ctx context.Context, id string, status string) error {
	if p.token == "" {
		return errors.New("WRIKE_TOKEN not set")
	}

	slog.Debug("wrike: updating status", "id", id, "status", status)
	apiURL := "https://www.wrike.com/api/v4/tasks/" + url.PathEscape(id)

	payload := map[string]string{"status": status}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, apiURL, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(string(payloadBytes))), nil
	}

	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := DoWithRetry(httpClient, req, DefaultRetryConfig)
	if err != nil {
		slog.Error("wrike: update status failed", "id", id, "error", err)

		return fmt.Errorf("wrike api: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		slog.Error("wrike: update status failed", "id", id, "status_code", resp.StatusCode)

		return fmt.Errorf("wrike api error: %d", resp.StatusCode)
	}

	slog.Debug("wrike: status updated", "id", id, "status", status)

	return nil
}

// AddComment adds a comment to a Wrike task.
func (p *WrikeProvider) AddComment(ctx context.Context, id string, comment string) error {
	if p.token == "" {
		return errors.New("WRIKE_TOKEN required for comments")
	}

	apiURL := fmt.Sprintf("https://www.wrike.com/api/v4/tasks/%s/comments", url.PathEscape(id))

	payload := map[string]string{"text": comment}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(string(payloadBytes))), nil
	}

	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Content-Type", "application/json")

	// Use NoRetryConfig for POST - retries can create duplicate comments
	resp, err := DoWithRetry(httpClient, req, NoRetryConfig)
	if err != nil {
		return fmt.Errorf("wrike api: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("wrike api error: %d", resp.StatusCode)
	}

	return nil
}

// --- internal helpers ---

// fetchTaskByID calls GET /tasks/{id} and returns the raw task data.
//
// The Wrike v4 API returns all standard fields by default at this endpoint,
// including description, parentIds, superParentIds, superTaskIds, and subTaskIds.
// No additional fields parameter is needed for these fields.
func (p *WrikeProvider) fetchTaskByID(ctx context.Context, id string) (*wrikeTaskData, error) {
	apiURL := "https://www.wrike.com/api/v4/tasks/" + url.PathEscape(id)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.token)

	resp, err := DoWithRetry(httpClient, req, DefaultRetryConfig)
	if err != nil {
		return nil, fmt.Errorf("wrike api: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wrike api error: %d", resp.StatusCode)
	}

	var response struct {
		Data []wrikeTaskData `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(response.Data) == 0 {
		return nil, fmt.Errorf("task not found: %s", id)
	}

	return &response.Data[0], nil
}

// fetchTasksInFolder calls GET /folders/{folderId}/tasks and returns a list
// of tasks. Uses the Wrike v4 folder tasks endpoint.
func (p *WrikeProvider) fetchTasksInFolder(ctx context.Context, folderID string) ([]*Task, error) {
	apiURL := fmt.Sprintf("https://www.wrike.com/api/v4/folders/%s/tasks", url.PathEscape(folderID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.token)

	resp, err := DoWithRetry(httpClient, req, DefaultRetryConfig)
	if err != nil {
		return nil, fmt.Errorf("wrike api: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wrike api error: %d", resp.StatusCode)
	}

	var response struct {
		Data []wrikeTaskData `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	tasks := make([]*Task, 0, len(response.Data))
	for i := range response.Data {
		tasks = append(tasks, p.taskDataToTask(&response.Data[i]))
	}

	return tasks, nil
}

// taskDataToTask converts a raw Wrike API task response to a Task.
//
// Metadata keys stored:
//   - wrike_parent_folder_id: first entry of parentIds (folder hierarchy),
//     used by FetchSiblings to list tasks in the same folder.
//   - wrike_super_task_id: first entry of superTaskIds (task hierarchy),
//     used by FetchParent to fetch the parent task of a subtask.
func (p *WrikeProvider) taskDataToTask(data *wrikeTaskData) *Task {
	task := &Task{
		ID:          data.ID,
		Title:       data.Title,
		Description: data.Description,
		URL:         data.Permalink,
		Labels:      []string{data.Status},
		Source:      "wrike",
	}

	// Store the first parent folder ID for sibling lookup.
	// parentIds are the folder(s) a task lives in — not task parents.
	if len(data.ParentIDs) > 0 {
		task.SetMetadata("wrike_parent_folder_id", data.ParentIDs[0])
	} else if len(data.SuperParentIDs) > 0 {
		// Fallback to superParentIds (top-level space/folder IDs) when parentIds is empty.
		task.SetMetadata("wrike_parent_folder_id", data.SuperParentIDs[0])
	}

	// Store the first superTaskId for parent task lookup.
	// superTaskIds are the parent task IDs when this task is a subtask —
	// distinct from folder parents.
	if len(data.SuperTaskIDs) > 0 {
		task.SetMetadata("wrike_super_task_id", data.SuperTaskIDs[0])
	}

	return task
}
