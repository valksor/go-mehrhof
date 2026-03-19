// Package timeline provides a read-only event timeline service that converts
// raw activity log entries into human-readable activity summaries. It reads
// data written by the activitylog package and adds structure, descriptions,
// and summary capabilities on top.
package timeline

import (
	"fmt"
	"strings"
	"time"

	"github.com/valksor/kvelmo/pkg/activitylog"
)

// Activity represents a single human-readable event in the project timeline.
type Activity struct {
	Timestamp   time.Time      `json:"timestamp"`
	Type        string         `json:"type"`                   // e.g., "task.loaded", "phase.started", "checkpoint.created"
	Description string         `json:"description"`            // Human-readable
	TaskID      string         `json:"task_id,omitempty"`      // Task that produced this event
	Phase       string         `json:"phase,omitempty"`        // Phase context (plan, implement, etc.)
	TraceID     string         `json:"trace_id,omitempty"`     // Task trace ID for causality chain
	Metadata    map[string]any `json:"metadata,omitempty"`     // Additional context
	Error       string         `json:"error,omitempty"`        // Error message if this was a failure
	DurationMs  int64          `json:"duration_ms,omitempty"`  // How long the operation took
}

// Service provides timeline queries over the activity log. It is read-only:
// all data comes from activity log entries written by the middleware layer.
type Service struct {
	activityLog *activitylog.Log
}

// New creates a timeline Service backed by the given activity log.
func New(log *activitylog.Log) *Service {
	return &Service{activityLog: log}
}

// TaskTimeline returns ordered activities for a specific task, identified by
// its task trace ID. Results are ordered oldest-first (chronological). If
// limit <= 0, all matching entries are returned.
func (s *Service) TaskTimeline(taskTraceID string, limit int) ([]Activity, error) {
	entries, err := s.activityLog.ListByTaskTrace(taskTraceID)
	if err != nil {
		return nil, fmt.Errorf("task timeline: %w", err)
	}

	activities := make([]Activity, 0, len(entries))
	for _, e := range entries {
		activities = append(activities, entryToActivity(e))
	}

	if limit > 0 && len(activities) > limit {
		activities = activities[:limit]
	}

	return activities, nil
}

// ProjectSummary returns event counts by type since a given time. It scans
// all activity log entries from the given time forward and groups them by
// their method (which maps to the event type).
func (s *Service) ProjectSummary(since time.Time) (map[string]int, error) {
	sinceD := time.Since(since)
	if sinceD < 0 {
		sinceD = 0
	}

	entries, err := s.activityLog.Query(activitylog.QueryOptions{
		Since: sinceD,
	})
	if err != nil {
		return nil, fmt.Errorf("project summary: %w", err)
	}

	counts := make(map[string]int)
	for _, e := range entries {
		counts[e.Method]++
	}

	return counts, nil
}

// entryToActivity converts a raw activity log entry to a human-readable Activity.
func entryToActivity(e activitylog.Entry) Activity {
	return Activity{
		Timestamp:   e.Timestamp,
		Type:        e.Method,
		Description: describe(e),
		TaskID:      e.TaskID,
		Phase:       phaseFromMethod(e.Method),
		TraceID:     e.TaskTraceID,
		Error:       e.Error,
		DurationMs:  e.DurationMs,
	}
}

// phaseFromMethod extracts the phase name from a method like "task.plan" -> "plan".
func phaseFromMethod(method string) string {
	if idx := strings.IndexByte(method, '.'); idx >= 0 {
		return method[idx+1:]
	}

	return method
}

// descriptions maps method names to human-readable format strings.
// Use %s as a placeholder for dynamic values extracted from the entry.
var descriptions = map[string]string{
	"task.start":       "Task loaded: %s",
	"task.plan":        "Agent started planning",
	"task.implement":   "Agent started implementing",
	"task.simplify":    "Agent started simplifying",
	"task.optimize":    "Agent started optimizing",
	"task.review":      "Entered review mode",
	"task.submit":      "PR submitted",
	"task.finish":      "Task completed",
	"task.undo":        "Undid to checkpoint %s",
	"task.redo":        "Redid to checkpoint %s",
	"task.abort":       "Task aborted",
	"task.reset":       "Task reset",
	"agent.connect":    "Agent connected",
	"agent.disconnect": "Agent disconnected",
	"agent.error":      "Agent error: %s",
	"quality.check":    "Quality gate: %s",
}

// describe generates a human-readable description from an activity log entry.
func describe(e activitylog.Entry) string {
	tmpl, ok := descriptions[e.Method]
	if !ok {
		// Fall back to a generic description.
		if e.Error != "" {
			return fmt.Sprintf("%s (error: %s)", e.Method, e.Error)
		}

		return e.Method
	}

	// If template has a %s placeholder, fill it with contextual info.
	if strings.Contains(tmpl, "%s") {
		arg := contextArg(e)
		if arg == "" {
			return e.Method
		}

		return fmt.Sprintf(tmpl, arg)
	}

	return tmpl
}

// contextArg extracts a contextual argument for description formatting.
func contextArg(e activitylog.Entry) string {
	switch {
	case e.Error != "":
		return e.Error
	case e.TaskID != "":
		return e.TaskID
	default:
		return ""
	}
}
