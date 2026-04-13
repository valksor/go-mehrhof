package socket

import (
	"context"
	"encoding/json"
	"time"

	"github.com/valksor/kvelmo/internal/filter"
	"github.com/valksor/kvelmo/internal/page"
	"github.com/valksor/kvelmo/internal/search"
)

// --- Project Handlers ---

func (g *GlobalSocket) handleListProjects(ctx context.Context, req *Request) (*Response, error) {
	// Snapshot registered worktrees under a short read lock.
	g.mu.RLock()
	infos := make([]*WorktreeInfo, 0, len(g.worktrees))
	for _, w := range g.worktrees {
		cp := *w
		infos = append(infos, &cp)
	}
	g.mu.RUnlock()

	// Query live state from each worktree socket (2-second timeout each).
	const liveTimeout = 2 * time.Second
	type liveResult struct {
		idx   int
		state string
	}
	results := make(chan liveResult, len(infos))

	for i, info := range infos {
		go func(i int, info *WorktreeInfo) {
			if info.SocketPath == "" {
				results <- liveResult{i, "offline"}

				return
			}
			client, err := NewClient(info.SocketPath, WithTimeout(liveTimeout))
			if err != nil {
				results <- liveResult{i, "offline"}

				return
			}
			defer func() { _ = client.Close() }()
			queryCtx, cancel := context.WithTimeout(ctx, liveTimeout)
			defer cancel()
			resp, err := client.Call(queryCtx, "status", nil)
			if err != nil {
				results <- liveResult{i, "offline"}

				return
			}
			var sr StatusResult
			if err := json.Unmarshal(resp.Result, &sr); err != nil {
				results <- liveResult{i, "offline"}

				return
			}
			results <- liveResult{i, string(sr.State)}
		}(i, info)
	}

	for range infos {
		r := <-results
		infos[r.idx].State = r.state
	}

	// Persist updated states back to the registry (skip offline so last-known state is preserved).
	g.mu.Lock()
	for _, info := range infos {
		if w, ok := g.worktrees[info.ID]; ok && info.State != "offline" {
			w.State = info.State
			w.LastSeen = time.Now()
		}
	}
	g.mu.Unlock()

	projects := make([]WorktreeInfo, 0, len(infos))
	for _, info := range infos {
		projects = append(projects, *info)
	}

	return NewResultResponse(req.ID, ProjectListResult{Projects: projects})
}

func (g *GlobalSocket) handleRegisterProject(ctx context.Context, req *Request) (*Response, error) {
	var params RegisterParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if params.Path == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "path is required"), nil
	}

	// Default socket path if not provided (frontend sends only path)
	socketPath := params.SocketPath
	if socketPath == "" {
		socketPath = WorktreeSocketPath(params.Path)
	}

	g.mu.Lock()
	id := WorktreeIDFromPath(params.Path)
	g.worktrees[id] = &WorktreeInfo{
		ID:         id,
		Path:       params.Path,
		SocketPath: socketPath,
		State:      "none",
		LastSeen:   time.Now(),
	}
	g.mu.Unlock()

	// Persist to file
	g.saveProjectsToFile()

	return NewResultResponse(req.ID, map[string]string{"id": id})
}

func (g *GlobalSocket) handleUnregisterProject(ctx context.Context, req *Request) (*Response, error) {
	var params UnregisterParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	g.mu.Lock()
	delete(g.worktrees, params.ID)
	g.mu.Unlock()

	// Persist to file
	g.saveProjectsToFile()

	return NewResultResponse(req.ID, map[string]bool{"ok": true})
}

// --- Worktree Handlers ---

func (g *GlobalSocket) handleWorktreeCreate(ctx context.Context, req *Request) (*Response, error) {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}
	if params.Path == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "path is required"), nil
	}

	_, err := g.GetOrCreateWorktreeSocket(params.Path) //nolint:contextcheck // provider clients are long-lived; request context must not cancel them
	if err != nil {
		return NewErrorResponse(req.ID, -32000, err.Error()), nil
	}

	socketPath := WorktreeSocketPath(params.Path)

	return NewResultResponse(req.ID, map[string]string{
		"socket_path": socketPath,
	})
}

// --- Tasks List Handler ---

// TaskListSummary represents a task for the tasks.list response.
type TaskListSummary struct {
	ID               string `json:"id"`
	Path             string `json:"path"`
	State            string `json:"state"`
	TaskID           string `json:"task_id,omitempty"`
	TaskTitle        string `json:"task_title,omitempty"`
	Source           string `json:"source,omitempty"`
	QueueCount       int    `json:"queue_count,omitempty"`
	LastError        string `json:"last_error,omitempty"`
	LastFailureClass string `json:"last_failure_class,omitempty"`
	PendingPromptID  string `json:"pending_prompt_id,omitempty"`
}

// Searchable interface implementation for TaskListSummary.
func (t TaskListSummary) SearchTitle() string        { return t.TaskTitle }
func (t TaskListSummary) SearchDescription() string  { return t.Source }
func (t TaskListSummary) SearchTags() []string       { return nil }
func (t TaskListSummary) SearchStatus() string       { return t.State }
func (t TaskListSummary) SearchCreatedAt() time.Time { return time.Time{} }
func (t TaskListSummary) SearchPriority() int        { return 0 }

// TasksListResult is the response for tasks.list.
type TasksListResult struct {
	Tasks      []TaskListSummary `json:"tasks"`
	Total      int               `json:"total"`
	Page       int               `json:"page,omitempty"`
	PerPage    int               `json:"per_page,omitempty"`
	TotalPages int               `json:"total_pages,omitempty"`
	HasNext    bool              `json:"has_next,omitempty"`
}

func (g *GlobalSocket) handleTasksList(ctx context.Context, req *Request) (*Response, error) {
	var params struct {
		State   string `json:"state,omitempty"`
		Query   string `json:"query,omitempty"`
		Page    int    `json:"page,omitempty"`
		PerPage int    `json:"per_page,omitempty"`
	}
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	g.mu.RLock()
	worktrees := make([]*WorktreeInfo, 0, len(g.worktrees))
	for _, w := range g.worktrees {
		worktrees = append(worktrees, w)
	}
	g.mu.RUnlock()

	tasks := make([]TaskListSummary, 0, len(worktrees))
	for _, wt := range worktrees {
		summary := TaskListSummary{
			ID:    wt.ID,
			Path:  wt.Path,
			State: wt.State,
		}

		// Try to get more task details by calling the worktree socket
		if wt.SocketPath != "" && SocketExists(wt.SocketPath) {
			client, err := NewClient(wt.SocketPath, WithTimeout(1*time.Second))
			if err == nil {
				resp, callErr := client.Call(ctx, "status", nil)
				if callErr == nil && resp != nil && resp.Result != nil {
					var status StatusResult
					if jsonErr := json.Unmarshal(resp.Result, &status); jsonErr == nil {
						if status.Task != nil {
							summary.TaskID = status.Task.ID
							summary.TaskTitle = status.Task.Title
							summary.Source = status.Task.Source
						}
						summary.State = string(status.State)
						summary.LastError = status.LastError
						summary.LastFailureClass = status.LastFailureClass
						summary.PendingPromptID = status.PendingPromptID
					}
				}
				// Fetch queue count
				qResp, qErr := client.Call(ctx, "queue.list", nil)
				if qErr == nil && qResp != nil && qResp.Result != nil {
					var qResult struct {
						Count int `json:"count"`
					}
					if jsonErr := json.Unmarshal(qResp.Result, &qResult); jsonErr == nil {
						summary.QueueCount = qResult.Count
					}
				}
				_ = client.Close()
			}
		}

		tasks = append(tasks, summary)
	}

	// Apply filters if provided
	if params.State != "" {
		f := filter.New[TaskListSummary]().
			Eq(func(t TaskListSummary) any { return t.State }, params.State)
		tasks = f.Apply(tasks)
	}

	// Apply search if query provided
	if params.Query != "" {
		ranked := search.Search(tasks, params.Query, search.Options{BoostActive: true})
		tasks = make([]TaskListSummary, len(ranked))
		for i, r := range ranked {
			tasks[i] = r.Item
		}
	}

	total := len(tasks)

	// Apply pagination if requested
	if params.Page > 0 || params.PerPage > 0 {
		pg := paginationWithDefault(params.Page, params.PerPage)
		p := page.NewPage(tasks, pg)

		return NewResultResponse(req.ID, TasksListResult{
			Tasks:      p.Items,
			Total:      int(p.Total),
			Page:       p.PageNum,
			PerPage:    p.PerPage,
			TotalPages: p.TotalPages,
			HasNext:    p.HasNext,
		})
	}

	return NewResultResponse(req.ID, TasksListResult{Tasks: tasks, Total: total})
}
