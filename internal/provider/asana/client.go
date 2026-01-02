package asana

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

const (
	defaultBaseURL = "https://app.asana.com/api/1.0"
	defaultTimeout = 30 * time.Second
)

// Client wraps the Asana API.
type Client struct {
	httpClient   *http.Client
	baseURL      string
	token        string
	workspaceGID string
}

// NewClient creates a new Asana API client.
func NewClient(token, workspaceGID string) *Client {
	return &Client{
		httpClient:   &http.Client{Timeout: defaultTimeout},
		baseURL:      defaultBaseURL,
		token:        token,
		workspaceGID: workspaceGID,
	}
}

// ResolveToken finds Asana token from multiple sources
// Priority:
//  1. MEHR_ASANA_TOKEN
//  2. ASANA_TOKEN
//  3. Config value
func ResolveToken(configToken string) (string, error) {
	if t := os.Getenv("MEHR_ASANA_TOKEN"); t != "" {
		return t, nil
	}
	if t := os.Getenv("ASANA_TOKEN"); t != "" {
		return t, nil
	}
	if configToken != "" {
		return configToken, nil
	}
	return "", ErrNoToken
}

// SetWorkspace updates the workspace GID.
func (c *Client) SetWorkspace(workspaceGID string) {
	c.workspaceGID = workspaceGID
}

// WorkspaceGID returns the current workspace GID.
func (c *Client) WorkspaceGID() string {
	return c.workspaceGID
}

// --- API Types ---

// Task represents an Asana task.
type Task struct {
	GID             string        `json:"gid"`
	Name            string        `json:"name"`
	Notes           string        `json:"notes"`
	HTMLNotes       string        `json:"html_notes"`
	ResourceType    string        `json:"resource_type"`
	Completed       bool          `json:"completed"`
	CompletedAt     *time.Time    `json:"completed_at"`
	DueOn           string        `json:"due_on"`
	DueAt           *time.Time    `json:"due_at"`
	StartOn         string        `json:"start_on"`
	CreatedAt       time.Time     `json:"created_at"`
	ModifiedAt      time.Time     `json:"modified_at"`
	Assignee        *User         `json:"assignee"`
	AssigneeStatus  string        `json:"assignee_status"`
	Followers       []User        `json:"followers"`
	Parent          *TaskRef      `json:"parent"`
	Projects        []Project     `json:"projects"`
	Memberships     []Membership  `json:"memberships"`
	Tags            []Tag         `json:"tags"`
	Workspace       *Workspace    `json:"workspace"`
	CustomFields    []CustomField `json:"custom_fields"`
	NumHearts       int           `json:"num_hearts"`
	NumLikes        int           `json:"num_likes"`
	Liked           bool          `json:"liked"`
	PermalinkURL    string        `json:"permalink_url"`
	ResourceSubtype string        `json:"resource_subtype"`
	ApprovalStatus  string        `json:"approval_status"`
}

// TaskRef is a minimal task reference.
type TaskRef struct {
	GID          string `json:"gid"`
	Name         string `json:"name"`
	ResourceType string `json:"resource_type"`
}

// User represents an Asana user.
type User struct {
	GID          string `json:"gid"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	ResourceType string `json:"resource_type"`
}

// Project represents an Asana project.
type Project struct {
	GID          string `json:"gid"`
	Name         string `json:"name"`
	ResourceType string `json:"resource_type"`
}

// Workspace represents an Asana workspace.
type Workspace struct {
	GID          string `json:"gid"`
	Name         string `json:"name"`
	ResourceType string `json:"resource_type"`
}

// Membership represents a task's membership in a project/section.
type Membership struct {
	Project *Project `json:"project"`
	Section *Section `json:"section"`
}

// Section represents a project section.
type Section struct {
	GID          string `json:"gid"`
	Name         string `json:"name"`
	ResourceType string `json:"resource_type"`
}

// Tag represents an Asana tag.
type Tag struct {
	GID          string `json:"gid"`
	Name         string `json:"name"`
	Color        string `json:"color"`
	ResourceType string `json:"resource_type"`
}

// CustomField represents a custom field value.
type CustomField struct {
	GID             string       `json:"gid"`
	Name            string       `json:"name"`
	Type            string       `json:"type"`
	ResourceType    string       `json:"resource_type"`
	TextValue       string       `json:"text_value"`
	NumberValue     *float64     `json:"number_value"`
	DisplayValue    string       `json:"display_value"`
	EnumValue       *EnumOption  `json:"enum_value"`
	MultiEnumValues []EnumOption `json:"multi_enum_values"`
	ResourceSubtype string       `json:"resource_subtype"`
}

// EnumOption represents an enum custom field option.
type EnumOption struct {
	GID          string `json:"gid"`
	Name         string `json:"name"`
	Color        string `json:"color"`
	Enabled      bool   `json:"enabled"`
	ResourceType string `json:"resource_type"`
}

// Story represents a story (comment/activity) on a task.
type Story struct {
	GID             string    `json:"gid"`
	ResourceType    string    `json:"resource_type"`
	Type            string    `json:"type"`
	Text            string    `json:"text"`
	HTMLText        string    `json:"html_text"`
	CreatedAt       time.Time `json:"created_at"`
	CreatedBy       *User     `json:"created_by"`
	Target          *TaskRef  `json:"target"`
	ResourceSubtype string    `json:"resource_subtype"`
}

// APIResponse wraps API responses.
type APIResponse[T any] struct {
	Data     T          `json:"data"`
	NextPage *NextPage  `json:"next_page"`
	Errors   []APIError `json:"errors"`
}

// NextPage contains pagination info.
type NextPage struct {
	Offset string `json:"offset"`
	Path   string `json:"path"`
	URI    string `json:"uri"`
}

// APIError represents an API error.
type APIError struct {
	Message string `json:"message"`
	Help    string `json:"help"`
	Phrase  string `json:"phrase"`
}

// --- HTTP Methods ---

func (c *Client) doRequest(ctx context.Context, method, path string, body any) ([]byte, error) {
	u := c.baseURL + path

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(map[string]any{"data": body})
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

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

// GetTask fetches a task by GID.
func (c *Client) GetTask(ctx context.Context, taskGID string) (*Task, error) {
	// Request common fields
	optFields := "gid,name,notes,html_notes,completed,completed_at,due_on,due_at,start_on," +
		"created_at,modified_at,assignee,assignee.name,assignee.email,followers,followers.name," +
		"parent,projects,projects.name,memberships,memberships.project.name,memberships.section.name," +
		"tags,tags.name,tags.color,workspace,workspace.name,custom_fields," +
		"num_likes,permalink_url,resource_subtype,approval_status"

	path := fmt.Sprintf("/tasks/%s?opt_fields=%s", taskGID, url.QueryEscape(optFields))

	body, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var resp APIResponse[Task]
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal task: %w", err)
	}

	return &resp.Data, nil
}

// ListProjectTasks lists tasks in a project.
func (c *Client) ListProjectTasks(ctx context.Context, projectGID string, completedSince *time.Time, limit int) ([]Task, error) {
	optFields := "gid,name,completed,due_on,assignee,assignee.name,tags,tags.name"

	params := url.Values{}
	params.Set("opt_fields", optFields)
	if completedSince != nil {
		params.Set("completed_since", completedSince.Format(time.RFC3339))
	}
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}

	path := fmt.Sprintf("/projects/%s/tasks?%s", projectGID, params.Encode())

	body, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var resp APIResponse[[]Task]
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal tasks: %w", err)
	}

	return resp.Data, nil
}

// GetTaskStories fetches stories (comments/activity) for a task.
func (c *Client) GetTaskStories(ctx context.Context, taskGID string) ([]Story, error) {
	optFields := "gid,type,text,html_text,created_at,created_by,created_by.name,resource_subtype"

	path := fmt.Sprintf("/tasks/%s/stories?opt_fields=%s", taskGID, url.QueryEscape(optFields))

	body, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var resp APIResponse[[]Story]
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal stories: %w", err)
	}

	return resp.Data, nil
}

// AddTaskComment adds a comment to a task.
func (c *Client) AddTaskComment(ctx context.Context, taskGID string, text string) (*Story, error) {
	path := fmt.Sprintf("/tasks/%s/stories", taskGID)

	reqBody := map[string]any{
		"text": text,
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, path, reqBody)
	if err != nil {
		return nil, err
	}

	var resp APIResponse[Story]
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal story: %w", err)
	}

	return &resp.Data, nil
}

// UpdateTask updates task fields.
func (c *Client) UpdateTask(ctx context.Context, taskGID string, updates map[string]any) (*Task, error) {
	path := fmt.Sprintf("/tasks/%s", taskGID)

	respBody, err := c.doRequest(ctx, http.MethodPut, path, updates)
	if err != nil {
		return nil, err
	}

	var resp APIResponse[Task]
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal task: %w", err)
	}

	return &resp.Data, nil
}

// CompleteTask marks a task as complete.
func (c *Client) CompleteTask(ctx context.Context, taskGID string) (*Task, error) {
	return c.UpdateTask(ctx, taskGID, map[string]any{"completed": true})
}

// AddTaskToSection moves a task to a section.
func (c *Client) AddTaskToSection(ctx context.Context, sectionGID, taskGID string) error {
	path := fmt.Sprintf("/sections/%s/addTask", sectionGID)

	reqBody := map[string]any{
		"task": taskGID,
	}

	_, err := c.doRequest(ctx, http.MethodPost, path, reqBody)
	return err
}

// --- Project API ---

// GetProject fetches project details.
func (c *Client) GetProject(ctx context.Context, projectGID string) (*Project, error) {
	path := fmt.Sprintf("/projects/%s", projectGID)

	body, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var resp APIResponse[Project]
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal project: %w", err)
	}

	return &resp.Data, nil
}

// GetProjectSections fetches sections in a project.
func (c *Client) GetProjectSections(ctx context.Context, projectGID string) ([]Section, error) {
	path := fmt.Sprintf("/projects/%s/sections", projectGID)

	body, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var resp APIResponse[[]Section]
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal sections: %w", err)
	}

	return resp.Data, nil
}

// --- Tag API ---

// GetWorkspaceTags fetches all tags in the workspace.
func (c *Client) GetWorkspaceTags(ctx context.Context) ([]Tag, error) {
	if c.workspaceGID == "" {
		return nil, fmt.Errorf("workspace GID required")
	}

	path := fmt.Sprintf("/workspaces/%s/tags?opt_fields=gid,name,color", c.workspaceGID)

	body, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var resp APIResponse[[]Tag]
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal tags: %w", err)
	}

	return resp.Data, nil
}

// CreateTag creates a new tag in the workspace.
func (c *Client) CreateTag(ctx context.Context, name string) (*Tag, error) {
	if c.workspaceGID == "" {
		return nil, fmt.Errorf("workspace GID required")
	}

	path := "/tags"

	reqBody := map[string]any{
		"name":      name,
		"workspace": c.workspaceGID,
	}

	respBody, err := c.doRequest(ctx, http.MethodPost, path, reqBody)
	if err != nil {
		return nil, err
	}

	var resp APIResponse[Tag]
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal tag: %w", err)
	}

	return &resp.Data, nil
}

// AddTagToTask adds a tag to a task.
func (c *Client) AddTagToTask(ctx context.Context, taskGID, tagGID string) error {
	path := fmt.Sprintf("/tasks/%s/addTag", taskGID)

	reqBody := map[string]any{
		"tag": tagGID,
	}

	_, err := c.doRequest(ctx, http.MethodPost, path, reqBody)
	return err
}

// RemoveTagFromTask removes a tag from a task.
func (c *Client) RemoveTagFromTask(ctx context.Context, taskGID, tagGID string) error {
	path := fmt.Sprintf("/tasks/%s/removeTag", taskGID)

	reqBody := map[string]any{
		"tag": tagGID,
	}

	_, err := c.doRequest(ctx, http.MethodPost, path, reqBody)
	return err
}

// --- Subtask API ---

// GetSubtasks fetches subtasks for a task.
func (c *Client) GetSubtasks(ctx context.Context, taskGID string) ([]Task, error) {
	optFields := "gid,name,notes,completed,completed_at,due_on,created_at,modified_at," +
		"assignee,assignee.name,assignee.email,tags,tags.name,permalink_url,resource_subtype"

	path := fmt.Sprintf("/tasks/%s/subtasks?opt_fields=%s", taskGID, url.QueryEscape(optFields))

	body, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var resp APIResponse[[]Task]
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal subtasks: %w", err)
	}

	return resp.Data, nil
}
