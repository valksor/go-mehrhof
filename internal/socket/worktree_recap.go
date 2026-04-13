package socket

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/valksor/kvelmo/internal/conductor"
	"github.com/valksor/kvelmo/internal/git"
)

func (w *WorktreeSocket) handleStatus(ctx context.Context, req *Request) (*Response, error) {
	result := StatusResult{
		Path:  w.path,
		State: StateNone,
	}

	if w.conductor != nil {
		state := w.conductor.State()
		result.State = TaskState(state)

		if wu := w.conductor.WorkUnit(); wu != nil {
			var sourceRef string
			if wu.Source != nil {
				sourceRef = wu.Source.Reference
			}
			result.Task = &TaskInfo{
				ID:           wu.ID,
				Title:        wu.Title,
				Source:       sourceRef,
				Branch:       wu.Branch,
				WorktreePath: wu.WorktreePath,
				ContextItems: wu.ContextItems,
			}
		}

		if ids := w.conductor.PendingPromptIDs(); len(ids) > 0 {
			result.PendingPromptID = ids[0]
		}

		if wu := w.conductor.WorkUnit(); wu != nil {
			switch state {
			case conductor.StatePlanning, conductor.StateImplementing, conductor.StateSimplifying,
				conductor.StateOptimizing, conductor.StateReviewing:
				if len(wu.Jobs) > 0 {
					result.ActiveJobID = wu.Jobs[len(wu.Jobs)-1]
				}
			case conductor.StateNone, conductor.StateLoaded, conductor.StatePlanned,
				conductor.StateImplemented, conductor.StateSubmitted, conductor.StateFailed,
				conductor.StateWaiting, conductor.StatePaused:
				// Not in a working state — no active job
			}
		}

		result.QueueDepth = w.conductor.QueueLength()

		if fc := w.conductor.LastFailureClass(); fc != "" {
			result.LastFailureClass = string(fc)
		}

		if wu := w.conductor.WorkUnit(); wu != nil && len(wu.PhaseMetrics) > 0 {
			result.PhaseMetrics = wu.PhaseMetrics
		}

		if rs := w.conductor.RecoveryState(); rs != "" {
			result.NeedsRecovery = rs
		}

		if sp := w.conductor.SkipPhases(); len(sp) > 0 {
			result.SkipPhases = sp
		}
	}

	return NewResultResponse(req.ID, result)
}

func (w *WorktreeSocket) handleRecap(ctx context.Context, req *Request) (*Response, error) {
	result := RecapResult{
		Path:       w.path,
		State:      StateNone,
		NextAction: "Run 'kvelmo start' to load a task",
	}

	if w.conductor == nil {
		return NewResultResponse(req.ID, result)
	}

	state := w.conductor.State()
	result.State = TaskState(state)

	wu := w.conductor.WorkUnit()
	if wu != nil {
		var sourceRef string
		if wu.Source != nil {
			sourceRef = wu.Source.Reference
		}
		result.Task = &TaskInfo{
			ID:           wu.ID,
			Title:        wu.Title,
			Source:       sourceRef,
			Branch:       wu.Branch,
			WorktreePath: wu.WorktreePath,
			ContextItems: wu.ContextItems,
		}

		result.CheckpointCount = len(wu.Checkpoints)
		result.Tags = wu.Tags

		if len(wu.PhaseMetrics) > 0 {
			result.PhaseMetrics = wu.PhaseMetrics
		}

		// Last checkpoint with enrichment
		if len(wu.Checkpoints) > 0 {
			lastSHA := wu.Checkpoints[len(wu.Checkpoints)-1]
			info := CheckpointInfo{SHA: lastSHA}
			if w.repo != nil {
				if entry, err := w.repo.CommitInfo(ctx, lastSHA); err == nil {
					info.Message = entry.Message
					info.Author = entry.Author
					info.Timestamp = entry.Date
					// Compute human-readable time since last activity
					if t, parseErr := time.Parse(time.RFC3339, entry.Date); parseErr == nil {
						result.LastActivity = formatTimeSince(t)
					}
				}
			}
			result.LastCheckpoint = &info
		}
	}

	// Files changed in working tree
	if w.repo != nil {
		if files, err := w.repo.DiffFilesWithStatus(ctx); err == nil {
			result.FilesChanged = files
		}
	}

	if fc := w.conductor.LastFailureClass(); fc != "" {
		switch fc {
		case conductor.FailureClassHardStop:
			result.LastError = "Hard failure — requires manual intervention (run 'kvelmo logs' for details)"
		case conductor.FailureClassRecoverable:
			result.LastError = "Transient error — will auto-retry"
		case conductor.FailureClassDegraded:
			result.LastError = "Non-critical failure — workflow continued with warning"
		case conductor.FailureClassSkippable:
			result.LastError = "Phase skipped — nothing to do"
		default:
			result.LastError = string(fc)
		}
	}

	result.NextAction = suggestNextAction(state, wu)

	return NewResultResponse(req.ID, result)
}

// suggestNextAction returns a human-readable suggestion for the next workflow step.
func suggestNextAction(state conductor.State, wu *conductor.WorkUnit) string {
	switch state {
	case conductor.StateNone:
		return "Run 'kvelmo start' to load a task"
	case conductor.StateLoaded:
		return "Run 'kvelmo plan' to generate a specification"
	case conductor.StatePlanning:
		return "Planning in progress — wait or run 'kvelmo watch'"
	case conductor.StatePlanned:
		return "Run 'kvelmo implement' to start coding"
	case conductor.StateImplementing:
		return "Implementation in progress — wait or run 'kvelmo watch'"
	case conductor.StateImplemented:
		return "Run 'kvelmo review' to check the work, or 'kvelmo simplify' for cleanup"
	case conductor.StateSimplifying:
		return "Simplification in progress — wait or run 'kvelmo watch'"
	case conductor.StateOptimizing:
		return "Optimization in progress — wait or run 'kvelmo watch'"
	case conductor.StateReviewing:
		return "Review in progress — wait or check 'kvelmo checklist list'"
	case conductor.StateSubmitted:
		if wu != nil && wu.PRID != "" {
			return "PR submitted (" + wu.PRID + ") — merge it, then run 'kvelmo finish'"
		}

		return "PR submitted — merge it, then run 'kvelmo finish'"
	case conductor.StateFailed:
		return "Run 'kvelmo retry' to re-run the failed phase, or 'kvelmo reset' to start over"
	case conductor.StateWaiting:
		return "Agent is waiting for input — check 'kvelmo quality respond' or answer via chat"
	case conductor.StatePaused:
		return "Task is paused — resume when ready"
	default:
		return ""
	}
}

// formatTimeSince returns a human-readable duration string.
func formatTimeSince(t time.Time) string {
	d := time.Since(t)
	if d < 0 {
		return "just now"
	}
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 minute ago"
		}

		return fmt.Sprintf("%d minutes ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}

		return fmt.Sprintf("%d hours ago", h)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}

		return fmt.Sprintf("%d days ago", days)
	}
}

// --- Show Spec/Plan Handlers ---

// SpecEntry holds a single specification file's path and content.
type SpecEntry struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// ShowSpecResult is returned by the show.spec and show.plan RPC methods.
type ShowSpecResult struct {
	Specifications []SpecEntry `json:"specifications"`
}

func (w *WorktreeSocket) handleShowSpec(_ context.Context, req *Request) (*Response, error) {
	if resp := w.noConductor(req.ID); resp != nil {
		return resp, nil
	}

	wu := w.conductor.WorkUnit()
	if wu == nil {
		return NewResultResponse(req.ID, ShowSpecResult{Specifications: []SpecEntry{}})
	}

	specs := make([]SpecEntry, 0, len(wu.Specifications))
	for _, path := range wu.Specifications {
		content, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("show.spec: failed to read specification", "path", path, "error", err)

			continue
		}
		specs = append(specs, SpecEntry{Path: path, Content: string(content)})
	}

	return NewResultResponse(req.ID, ShowSpecResult{Specifications: specs})
}

// RecapResult is returned by the recap RPC method.
// It provides a concise summary of the current task state for resuming work.
type RecapResult struct {
	State           TaskState                          `json:"state"`
	Path            string                             `json:"path"`
	Task            *TaskInfo                          `json:"task,omitempty"`
	LastCheckpoint  *CheckpointInfo                    `json:"last_checkpoint,omitempty"`
	CheckpointCount int                                `json:"checkpoint_count"`
	FilesChanged    []git.FileStatus                   `json:"files_changed,omitempty"`
	PhaseMetrics    map[string]*conductor.PhaseMetrics `json:"phase_metrics,omitempty"`
	Tags            []string                           `json:"tags,omitempty"`
	LastActivity    string                             `json:"last_activity,omitempty"` // Human-readable time since last checkpoint
	NextAction      string                             `json:"next_action"`             // Suggested next step
	LastError       string                             `json:"last_error,omitempty"`
}
