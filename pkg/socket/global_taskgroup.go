package socket

import (
	"context"
	"encoding/json"

	"github.com/valksor/kvelmo/pkg/taskgroup"
)

var taskGroupCoordinator *taskgroup.Coordinator

// SetTaskGroupCoordinator sets the global task group coordinator instance.
func SetTaskGroupCoordinator(c *taskgroup.Coordinator) {
	taskGroupCoordinator = c
}

// GetTaskGroupCoordinator returns the global task group coordinator.
func GetTaskGroupCoordinator() *taskgroup.Coordinator {
	return taskGroupCoordinator
}

func (g *GlobalSocket) handleTaskGroupCreate(_ context.Context, req *Request) (*Response, error) {
	if taskGroupCoordinator == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "task groups not configured"), nil
	}
	var params struct {
		Label string `json:"label"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil
	}
	if params.Label == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "label is required"), nil
	}
	group, err := taskGroupCoordinator.CreateGroup(params.Label)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}
	return NewResultResponse(req.ID, group)
}

func (g *GlobalSocket) handleTaskGroupList(_ context.Context, req *Request) (*Response, error) {
	if taskGroupCoordinator == nil {
		return NewResultResponse(req.ID, map[string]any{"groups": []any{}})
	}
	groups := taskGroupCoordinator.ListGroups()
	return NewResultResponse(req.ID, map[string]any{"groups": groups})
}

func (g *GlobalSocket) handleTaskGroupStatus(_ context.Context, req *Request) (*Response, error) {
	if taskGroupCoordinator == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "task groups not configured"), nil
	}
	var params struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil
	}
	group, err := taskGroupCoordinator.GetGroup(params.ID)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}
	return NewResultResponse(req.ID, group)
}

func (g *GlobalSocket) handleTaskGroupAdd(_ context.Context, req *Request) (*Response, error) {
	if taskGroupCoordinator == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "task groups not configured"), nil
	}
	var params struct {
		ID         string `json:"id"`
		ProjectDir string `json:"project_dir"`
		TaskID     string `json:"task_id"`
		State      string `json:"state"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil
	}
	if params.ID == "" || params.TaskID == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "id and task_id are required"), nil
	}
	ref := taskgroup.TaskRef{
		ProjectDir: params.ProjectDir,
		TaskID:     params.TaskID,
		State:      params.State,
	}
	if err := taskGroupCoordinator.AddTask(params.ID, ref); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}
	group, _ := taskGroupCoordinator.GetGroup(params.ID)
	return NewResultResponse(req.ID, group)
}

func (g *GlobalSocket) handleTaskGroupSubmit(_ context.Context, req *Request) (*Response, error) {
	if taskGroupCoordinator == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "task groups not configured"), nil
	}
	var params struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil
	}
	if err := taskGroupCoordinator.SubmitGroup(params.ID); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}
	group, _ := taskGroupCoordinator.GetGroup(params.ID)
	return NewResultResponse(req.ID, group)
}

func (g *GlobalSocket) handleTaskGroupRemove(_ context.Context, req *Request) (*Response, error) {
	if taskGroupCoordinator == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "task groups not configured"), nil
	}
	var params struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params"), nil
	}
	if err := taskGroupCoordinator.RemoveGroup(params.ID); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}
	return NewResultResponse(req.ID, map[string]string{"status": "removed"})
}
