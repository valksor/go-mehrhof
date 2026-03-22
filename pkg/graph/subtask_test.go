package graph

import (
	"testing"
)

func TestNodeIsSubTask(t *testing.T) {
	normal := &Node{ID: "a", Label: "normal"}
	if normal.IsSubTask() {
		t.Error("normal node should not be a sub-task")
	}

	subtask := &Node{
		ID:    "b",
		Label: "sub",
		SubTask: &SubTaskConfig{
			Title:  "Write tests",
			Phases: []string{"plan", "implement"},
			Branch: "subtask/tests",
		},
	}
	if !subtask.IsSubTask() {
		t.Error("node with SubTask config should be a sub-task")
	}
}

func TestSubTaskConfig(t *testing.T) {
	cfg := SubTaskConfig{
		Title:       "Add authentication",
		Description: "Implement OAuth2",
		Phases:      []string{"plan", "implement"},
		Branch:      "feat/auth",
		Metadata:    map[string]string{"priority": "high"},
	}

	if cfg.Title != "Add authentication" {
		t.Errorf("unexpected title: %s", cfg.Title)
	}
	if len(cfg.Phases) != 2 {
		t.Errorf("expected 2 phases, got %d", len(cfg.Phases))
	}
	if cfg.Metadata["priority"] != "high" {
		t.Error("metadata not preserved")
	}
}
