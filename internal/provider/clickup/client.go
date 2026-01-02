package clickup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

const (
	defaultBaseURL = "https://api.clickup.com/api/v2"
	defaultTimeout = 30 * time.Second
)

// Client wraps the ClickUp API.
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

// NewClient creates a new ClickUp API client.
func NewClient(token string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: defaultTimeout},
		baseURL:    defaultBaseURL,
		token:      token,
	}
}

// ResolveToken finds ClickUp token from multiple sources
// Priority:
//  1. MEHR_CLICKUP_TOKEN
//  2. CLICKUP_TOKEN
//  3. Config value
func ResolveToken(configToken string) (string, error) {
	if t := os.Getenv("MEHR_CLICKUP_TOKEN"); t != "" {
		return t, nil
	}
	if t := os.Getenv("CLICKUP_TOKEN"); t != "" {
		return t, nil
	}
	if configToken != "" {
		return configToken, nil
	}

	return "", ErrNoToken
}

// --- API Types ---

// Task represents a ClickUp task.
type Task struct {
	ID              string        `json:"id"`
	CustomID        string        `json:"custom_id"`
	Name            string        `json:"name"`
	TextContent     string        `json:"text_content"`
	Description     string        `json:"description"`
	Status          *Status       `json:"status"`
	Priority        *Priority     `json:"priority"`
	DueDate         *int64        `json:"due_date,string"`
	StartDate       *int64        `json:"start_date,string"`
	DateCreated     string        `json:"date_created"`
	DateUpdated     string        `json:"date_updated"`
	DateClosed      string        `json:"date_closed"`
	Creator         *User         `json:"creator"`
	Assignees       []User        `json:"assignees"`
	Watchers        []User        `json:"watchers"`
	Checklists      []Checklist   `json:"checklists"`
	Tags            []Tag         `json:"tags"`
	Parent          string        `json:"parent"`
	Folder          *Folder       `json:"folder"`
	Space           *Space        `json:"space"`
	List            *List         `json:"list"`
	Project         *Project      `json:"project"`
	URL             string        `json:"url"`
	PermissionLevel string        `json:"permission_level"`
	CustomFields    []CustomField `json:"custom_fields"`
	Attachments     []Attachment  `json:"attachments"`
	LinkedTasks     []LinkedTask  `json:"linked_tasks"`
	TeamID          string        `json:"team_id"`
	Points          *float64      `json:"points"`
	TimeEstimate    *int64        `json:"time_estimate"`
}

// Status represents a task status.
type Status struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	Color      string `json:"color"`
	Type       string `json:"type"`
	Orderindex int    `json:"orderindex"`
}

// Priority represents a task priority.
type Priority struct {
	ID         string `json:"id"`
	Priority   string `json:"priority"`
	Color      string `json:"color"`
	Orderindex string `json:"orderindex"`
}

// User represents a ClickUp user.
type User struct {
	ID             int    `json:"id"`
	Username       string `json:"username"`
	Email          string `json:"email"`
	Color          string `json:"color"`
	ProfilePicture string `json:"profilePicture"`
	Initials       string `json:"initials"`
}

// Folder represents a ClickUp folder.
type Folder struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Hidden bool   `json:"hidden"`
	Access bool   `json:"access"`
}

// Space represents a ClickUp space.
type Space struct {
	ID string `json:"id"`
}

// List represents a ClickUp list.
type List struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Access bool   `json:"access"`
}

// Project represents a ClickUp project (legacy, same as List).
type Project struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Hidden bool   `json:"hidden"`
	Access bool   `json:"access"`
}

// Tag represents a ClickUp tag.
type Tag struct {
	Name    string `json:"name"`
	TagFg   string `json:"tag_fg"`
	TagBg   string `json:"tag_bg"`
	Creator int    `json:"creator"`
}

// Checklist represents a ClickUp checklist.
type Checklist struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Orderindex int             `json:"orderindex"`
	Items      []ChecklistItem `json:"items"`
}

// ChecklistItem represents an item in a checklist.
type ChecklistItem struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Orderindex int    `json:"orderindex"`
	Resolved   bool   `json:"resolved"`
}

// CustomField represents a custom field value.
type CustomField struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	TypeConfig     any    `json:"type_config"`
	Value          any    `json:"value"`
	DateCreated    string `json:"date_created"`
	HideFromGuests bool   `json:"hide_from_guests"`
	Required       bool   `json:"required"`
}

// Attachment represents a task attachment.
type Attachment struct {
	ID              string `json:"id"`
	Version         string `json:"version"`
	Date            string `json:"date"`
	Title           string `json:"title"`
	Extension       string `json:"extension"`
	ThumbnailSmall  string `json:"thumbnail_small"`
	ThumbnailMedium string `json:"thumbnail_medium"`
	ThumbnailLarge  string `json:"thumbnail_large"`
	URL             string `json:"url"`
}

// LinkedTask represents a linked task.
type LinkedTask struct {
	TaskID      string `json:"task_id"`
	LinkID      string `json:"link_id"`
	DateCreated string `json:"date_created"`
	Userid      string `json:"userid"`
}

// Comment represents a task comment.
type Comment struct {
	ID          string        `json:"id"`
	Comment     []CommentPart `json:"comment"`
	CommentText string        `json:"comment_text"`
	User        User          `json:"user"`
	Date        string        `json:"date"`
	Reactions   []Reaction    `json:"reactions"`
}

// CommentPart represents part of a comment (text, mentions, etc.)
type CommentPart struct {
	Text string `json:"text"`
}

// Reaction represents a comment reaction.
type Reaction struct {
	Reaction string `json:"reaction"`
	Date     string `json:"date"`
	User     User   `json:"user"`
}

// TasksResponse represents the response from listing tasks.
type TasksResponse struct {
	Tasks []Task `json:"tasks"`
}

// CommentsResponse represents the response from listing comments.
type CommentsResponse struct {
	Comments []Comment `json:"comments"`
}

// --- HTTP Methods ---

func (c *Client) doRequest(ctx context.Context, method, path string, body any) ([]byte, error) {
	u := c.baseURL + path

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, wrapAPIError(err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, wrapAPIError(fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody)))
	}

	return respBody, nil
}

// --- Task API ---

// GetTask fetches a task by ID.
func (c *Client) GetTask(ctx context.Context, taskID string) (*Task, error) {
	path := fmt.Sprintf("/task/%s?include_subtasks=true&custom_task_ids=true", taskID)

	body, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var task Task
	if err := json.Unmarshal(body, &task); err != nil {
		return nil, fmt.Errorf("unmarshal task: %w", err)
	}

	return &task, nil
}

// GetTaskByCustomID fetches a task by custom ID.
func (c *Client) GetTaskByCustomID(ctx context.Context, teamID, customID string) (*Task, error) {
	path := fmt.Sprintf("/task/%s?custom_task_ids=true&team_id=%s", customID, teamID)

	body, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var task Task
	if err := json.Unmarshal(body, &task); err != nil {
		return nil, fmt.Errorf("unmarshal task: %w", err)
	}

	return &task, nil
}

// ListTasks lists tasks in a list.
func (c *Client) ListTasks(ctx context.Context, listID string, archived bool, limit int) ([]Task, error) {
	params := url.Values{}
	params.Set("archived", strconv.FormatBool(archived))
	if limit > 0 {
		params.Set("page_size", strconv.Itoa(limit))
	}
	params.Set("include_closed", "true")

	path := fmt.Sprintf("/list/%s/task?%s", listID, params.Encode())

	body, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var resp TasksResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal tasks: %w", err)
	}

	return resp.Tasks, nil
}

// GetTaskComments fetches comments for a task.
func (c *Client) GetTaskComments(ctx context.Context, taskID string) ([]Comment, error) {
	path := fmt.Sprintf("/task/%s/comment", taskID)

	body, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var resp CommentsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal comments: %w", err)
	}

	return resp.Comments, nil
}

// AddTaskComment adds a comment to a task.
func (c *Client) AddTaskComment(ctx context.Context, taskID string, text string) (*Comment, error) {
	path := fmt.Sprintf("/task/%s/comment", taskID)

	reqBody := map[string]any{
		"comment_text": text,
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, path, reqBody)
	if err != nil {
		return nil, err
	}

	var comment Comment
	if err := json.Unmarshal(respBody, &comment); err != nil {
		return nil, fmt.Errorf("unmarshal comment: %w", err)
	}

	return &comment, nil
}

// UpdateTask updates task fields.
func (c *Client) UpdateTask(ctx context.Context, taskID string, updates map[string]any) (*Task, error) {
	path := "/task/" + taskID

	respBody, err := c.doRequest(ctx, http.MethodPut, path, updates)
	if err != nil {
		return nil, err
	}

	var task Task
	if err := json.Unmarshal(respBody, &task); err != nil {
		return nil, fmt.Errorf("unmarshal task: %w", err)
	}

	return &task, nil
}

// UpdateTaskStatus updates the task status.
func (c *Client) UpdateTaskStatus(ctx context.Context, taskID string, status string) (*Task, error) {
	return c.UpdateTask(ctx, taskID, map[string]any{"status": status})
}

// GetListStatuses fetches available statuses for a list.
func (c *Client) GetListStatuses(ctx context.Context, listID string) ([]Status, error) {
	path := "/list/" + listID

	body, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Statuses []Status `json:"statuses"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal list: %w", err)
	}

	return resp.Statuses, nil
}

// CreateTask creates a new task in a list.
func (c *Client) CreateTask(ctx context.Context, listID string, taskData map[string]any) (*Task, error) {
	path := fmt.Sprintf("/list/%s/task", listID)

	respBody, err := c.doRequest(ctx, http.MethodPost, path, taskData)
	if err != nil {
		return nil, err
	}

	var task Task
	if err := json.Unmarshal(respBody, &task); err != nil {
		return nil, fmt.Errorf("unmarshal task: %w", err)
	}

	return &task, nil
}

// GetSubtasks fetches subtasks for a given task.
func (c *Client) GetSubtasks(ctx context.Context, taskID string) ([]Task, error) {
	// Get the task with subtasks included
	path := fmt.Sprintf("/task/%s?include_subtasks=true", taskID)

	body, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var task struct {
		Subtasks []Task `json:"subtasks"`
	}
	if err := json.Unmarshal(body, &task); err != nil {
		return nil, fmt.Errorf("unmarshal task: %w", err)
	}

	return task.Subtasks, nil
}
