package socket

import (
	"context"
	"encoding/json"
	"time"

	"github.com/valksor/kvelmo/internal/worker"
	"github.com/valksor/kvelmo/metrics"
)

// --- Worker Handlers ---

// poolStatsToWorkersStats converts pool stats to the WorkersStats response type.
func poolStatsToWorkersStats(stats worker.PoolStats) WorkersStats {
	return WorkersStats{
		TotalWorkers:     stats.TotalWorkers,
		AvailableWorkers: stats.AvailableWorkers,
		WorkingWorkers:   stats.WorkingWorkers,
		QueuedJobs:       stats.QueuedJobs,
		InProgressJobs:   stats.InProgressJobs,
		CompletedJobs:    stats.CompletedJobs,
		FailedJobs:       stats.FailedJobs,
	}
}

func (g *GlobalSocket) handleListWorkers(ctx context.Context, req *Request) (*Response, error) {
	if g.pool == nil {
		return NewResultResponse(req.ID, WorkersListResult{
			Workers: []WorkerInfo{},
			Stats:   WorkersStats{},
		})
	}

	workers := g.pool.ListWorkers()
	stats := g.pool.Stats()

	result := WorkersListResult{
		Workers: make([]WorkerInfo, len(workers)),
		Stats:   poolStatsToWorkersStats(stats),
	}

	for i, w := range workers {
		result.Workers[i] = WorkerInfo{
			ID:         w.ID,
			AgentName:  w.AgentName,
			Status:     string(w.Status),
			CurrentJob: w.CurrentJob,
			IsDefault:  w.IsDefault,
		}
	}

	return NewResultResponse(req.ID, result)
}

func (g *GlobalSocket) handleAddWorker(ctx context.Context, req *Request) (*Response, error) {
	if g.pool == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no worker pool configured"), nil
	}

	var params AddWorkerParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	// Default agent to claude if not specified
	if params.Agent == "" {
		params.Agent = "claude"
	}

	w := g.pool.AddWorkerWithAgent(params.Agent)
	if w == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "max workers reached"), nil
	}

	return NewResultResponse(req.ID, WorkerInfo{
		ID:        w.ID,
		AgentName: w.AgentName,
		Status:    string(w.Status),
	})
}

func (g *GlobalSocket) handleRemoveWorker(ctx context.Context, req *Request) (*Response, error) {
	if g.pool == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no worker pool configured"), nil
	}

	var params RemoveWorkerParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	if err := g.pool.RemoveWorker(params.ID); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, map[string]bool{"ok": true})
}

func (g *GlobalSocket) handleWorkerStats(ctx context.Context, req *Request) (*Response, error) {
	if g.pool == nil {
		return NewResultResponse(req.ID, WorkersStats{})
	}

	stats := g.pool.Stats()

	return NewResultResponse(req.ID, poolStatsToWorkersStats(stats))
}

func (g *GlobalSocket) handleMetrics(ctx context.Context, req *Request) (*Response, error) {
	snapshot := metrics.Global().Snapshot()

	return NewResultResponse(req.ID, snapshot)
}

// --- Job Handlers ---

func (g *GlobalSocket) handleListJobs(ctx context.Context, req *Request) (*Response, error) {
	if g.pool == nil {
		return NewResultResponse(req.ID, map[string]any{"jobs": []any{}})
	}

	jobs := g.pool.ListJobs()
	result := make([]map[string]any, len(jobs))
	for i, j := range jobs {
		result[i] = map[string]any{
			"id":         j.ID,
			"type":       j.Type,
			keyStatus:    j.Status,
			"worktree":   j.WorktreeID,
			"created_at": j.CreatedAt.Format(time.RFC3339),
		}
	}

	return NewResultResponse(req.ID, map[string]any{"jobs": result})
}

func (g *GlobalSocket) handleGetJob(ctx context.Context, req *Request) (*Response, error) {
	if g.pool == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no worker pool configured"), nil
	}

	var params struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	job := g.pool.GetJob(params.ID)
	if job == nil {
		return NewErrorResponse(req.ID, ErrCodeNotFound, "job not found: "+params.ID), nil
	}

	result := map[string]any{
		"id":         job.ID,
		"type":       job.Type,
		keyStatus:    job.Status,
		"worktree":   job.WorktreeID,
		"prompt":     job.Prompt,
		"created_at": job.CreatedAt.Format(time.RFC3339),
	}

	if job.WorkerID != "" {
		result["worker_id"] = job.WorkerID
	}
	if job.StartedAt != nil {
		result["started_at"] = job.StartedAt.Format(time.RFC3339)
	}
	if job.CompletedAt != nil {
		result["completed_at"] = job.CompletedAt.Format(time.RFC3339)
	}
	if job.Result != "" {
		result["result"] = job.Result
	}
	if job.Error != "" {
		result["error"] = job.Error
	}

	return NewResultResponse(req.ID, result)
}
