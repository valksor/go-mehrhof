package conductor

import (
	"github.com/valksor/kvelmo/internal/storage"
)

// workUnitToTaskState converts a WorkUnit + state to the on-disk TaskState struct.
func workUnitToTaskState(state State, wu *WorkUnit, history []HistoryEntry) *storage.TaskState {
	ts := &storage.TaskState{
		State:             string(state),
		ID:                wu.ID,
		ExternalID:        wu.ExternalID,
		Title:             wu.Title,
		Description:       wu.Description,
		Branch:            wu.Branch,
		WorktreePath:      wu.WorktreePath,
		Specifications:    wu.Specifications,
		Checkpoints:       wu.Checkpoints,
		CheckpointMeta:    convertCheckpointMetaToStorage(wu.CheckpointMeta),
		RedoStack:         wu.RedoStack,
		Jobs:              wu.Jobs,
		Metadata:          wu.Metadata,
		ChecklistChecked:  wu.ChecklistChecked,
		Tags:              wu.Tags,
		Priority:          wu.Priority,
		DependsOn:         wu.DependsOn,
		QualityGatePassed: wu.QualityGatePassed,
		VarPoolPath:       wu.VarPoolPath,
		HasImplemented:    wu.HasImplemented,
		TaskTraceID:       wu.TaskTraceID,
		CreatedAt:         wu.CreatedAt,
		UpdatedAt:         wu.UpdatedAt,
	}
	// Convert approval records
	if len(wu.Approvals) > 0 {
		ts.Approvals = make(map[string]storage.TaskApprovalRecord, len(wu.Approvals))
		for k, v := range wu.Approvals {
			ts.Approvals[k] = storage.TaskApprovalRecord{
				ApprovedBy: v.ApprovedBy,
				ApprovedAt: v.ApprovedAt,
			}
		}
	}
	if wu.Source != nil {
		ts.Source = &storage.TaskSource{
			Provider:  wu.Source.Provider,
			Reference: wu.Source.Reference,
			URL:       wu.Source.URL,
			Content:   wu.Source.Content,
		}
	}
	if wu.Hierarchy != nil {
		h := &storage.TaskHierarchy{}
		if wu.Hierarchy.Parent != nil {
			h.Parent = &storage.TaskHierarchySummary{
				ID:          wu.Hierarchy.Parent.ID,
				Title:       wu.Hierarchy.Parent.Title,
				Description: wu.Hierarchy.Parent.Description,
				URL:         wu.Hierarchy.Parent.URL,
				Status:      wu.Hierarchy.Parent.Status,
			}
		}
		for _, s := range wu.Hierarchy.Siblings {
			h.Siblings = append(h.Siblings, storage.TaskHierarchySummary{
				ID:          s.ID,
				Title:       s.Title,
				Description: s.Description,
				URL:         s.URL,
				Status:      s.Status,
			})
		}
		ts.Hierarchy = h
	}
	for _, h := range history {
		ts.History = append(ts.History, storage.TaskHistoryEntry{
			From:      string(h.From),
			To:        string(h.To),
			Event:     string(h.Event),
			Timestamp: h.Timestamp,
		})
	}

	return ts
}

// taskStateToWorkUnit converts an on-disk TaskState back to a State + WorkUnit pair
// along with the persisted state machine history.
func taskStateToWorkUnit(ts *storage.TaskState) (State, *WorkUnit, []HistoryEntry) {
	wu := &WorkUnit{
		ID:                ts.ID,
		ExternalID:        ts.ExternalID,
		Title:             ts.Title,
		Description:       ts.Description,
		Branch:            ts.Branch,
		WorktreePath:      ts.WorktreePath,
		Specifications:    ts.Specifications,
		Checkpoints:       ts.Checkpoints,
		CheckpointMeta:    convertCheckpointMetaFromStorage(ts.CheckpointMeta),
		RedoStack:         ts.RedoStack,
		Jobs:              ts.Jobs,
		Metadata:          ts.Metadata,
		ChecklistChecked:  ts.ChecklistChecked,
		Tags:              ts.Tags,
		Priority:          ts.Priority,
		DependsOn:         ts.DependsOn,
		QualityGatePassed: ts.QualityGatePassed,
		VarPoolPath:       ts.VarPoolPath,
		HasImplemented:    ts.HasImplemented,
		TaskTraceID:       ts.TaskTraceID,
		CreatedAt:         ts.CreatedAt,
		UpdatedAt:         ts.UpdatedAt,
	}
	// Convert approval records
	if len(ts.Approvals) > 0 {
		wu.Approvals = make(map[string]ApprovalRecord, len(ts.Approvals))
		for k, v := range ts.Approvals {
			wu.Approvals[k] = ApprovalRecord{
				ApprovedBy: v.ApprovedBy,
				ApprovedAt: v.ApprovedAt,
			}
		}
	}
	if wu.Metadata == nil {
		wu.Metadata = make(map[string]string)
	}
	if ts.Source != nil {
		wu.Source = &Source{
			Provider:  ts.Source.Provider,
			Reference: ts.Source.Reference,
			URL:       ts.Source.URL,
			Content:   ts.Source.Content,
		}
	}
	if ts.Hierarchy != nil {
		h := &HierarchyContext{}
		if ts.Hierarchy.Parent != nil {
			h.Parent = &TaskSummary{
				ID:          ts.Hierarchy.Parent.ID,
				Title:       ts.Hierarchy.Parent.Title,
				Description: ts.Hierarchy.Parent.Description,
				URL:         ts.Hierarchy.Parent.URL,
				Status:      ts.Hierarchy.Parent.Status,
			}
		}
		for _, s := range ts.Hierarchy.Siblings {
			h.Siblings = append(h.Siblings, TaskSummary{
				ID:          s.ID,
				Title:       s.Title,
				Description: s.Description,
				URL:         s.URL,
				Status:      s.Status,
			})
		}
		wu.Hierarchy = h
	}

	history := make([]HistoryEntry, 0, len(ts.History))
	for _, h := range ts.History {
		history = append(history, HistoryEntry{
			From:      State(h.From),
			To:        State(h.To),
			Event:     Event(h.Event),
			Timestamp: h.Timestamp,
		})
	}

	return State(ts.State), wu, history
}

// convertCheckpointMetaToStorage converts conductor CheckpointMeta to storage format.
func convertCheckpointMetaToStorage(meta map[string]CheckpointMeta) map[string]storage.TaskCheckpointMeta {
	if len(meta) == 0 {
		return nil
	}
	result := make(map[string]storage.TaskCheckpointMeta, len(meta))
	for sha, m := range meta {
		result[sha] = storage.TaskCheckpointMeta{
			Message:   m.Message,
			State:     m.State,
			CreatedAt: m.CreatedAt,
		}
	}

	return result
}

// convertCheckpointMetaFromStorage converts storage CheckpointMeta to conductor format.
func convertCheckpointMetaFromStorage(meta map[string]storage.TaskCheckpointMeta) map[string]CheckpointMeta {
	if len(meta) == 0 {
		return nil
	}
	result := make(map[string]CheckpointMeta, len(meta))
	for sha, m := range meta {
		result[sha] = CheckpointMeta{
			Message:   m.Message,
			State:     m.State,
			CreatedAt: m.CreatedAt,
		}
	}

	return result
}
