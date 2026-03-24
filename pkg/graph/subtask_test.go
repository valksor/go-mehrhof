package graph

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestNodeIsSubTask(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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

func TestSubTaskExecutor_NotConfigured(t *testing.T) {
	t.Parallel()

	g := New()
	_ = g.AddNode(&Node{
		ID:    "st",
		Label: "sub-task",
		SubTask: &SubTaskConfig{
			Title:  "Test task",
			Phases: []string{"implement"},
		},
	})
	_ = g.Validate()

	// No SubTaskExecutor configured — should fail gracefully
	s := NewScheduler(g, nil)
	events := s.Run(context.Background(), JobOpts{})

	var failed bool
	for evt := range events {
		if evt.Type == EventNodeFailed && evt.NodeID == "st" {
			failed = true
		}
	}

	if !failed {
		t.Fatal("expected sub-task to fail when no executor configured")
	}
}

func TestSubTaskExecutor_Success(t *testing.T) {
	t.Parallel()

	g := New()
	_ = g.AddNode(&Node{
		ID:    "st",
		Label: "sub-task",
		SubTask: &SubTaskConfig{
			Title:  "Write tests",
			Phases: []string{"implement"},
		},
	})
	_ = g.Validate()

	executor := func(_ context.Context, cfg SubTaskConfig) (string, error) {
		if cfg.Title != "Write tests" {
			return "", fmt.Errorf("unexpected title: %s", cfg.Title)
		}

		return "tests written", nil
	}

	s := NewScheduler(g, nil, WithSubTaskExecutor(executor))
	events := s.Run(context.Background(), JobOpts{})

	var completed bool
	for evt := range events {
		if evt.Type == EventNodeCompleted && evt.NodeID == "st" {
			completed = true
		}
	}

	if !completed {
		t.Fatal("expected sub-task to complete successfully")
	}

	result := s.State().Result("st")
	if result != "tests written" {
		t.Fatalf("unexpected result: %q", result)
	}
}

func TestSubTaskExecutor_Failure(t *testing.T) {
	t.Parallel()

	g := New()
	_ = g.AddNode(&Node{
		ID:    "st",
		Label: "sub-task",
		SubTask: &SubTaskConfig{
			Title:  "Bad task",
			Phases: []string{"implement"},
		},
	})
	_ = g.Validate()

	executor := func(_ context.Context, _ SubTaskConfig) (string, error) {
		return "", errors.New("sub-task failed: build error")
	}

	s := NewScheduler(g, nil, WithSubTaskExecutor(executor))
	events := s.Run(context.Background(), JobOpts{})

	var failed bool
	for evt := range events {
		if evt.Type == EventNodeFailed && evt.NodeID == "st" {
			failed = true
		}
	}

	if !failed {
		t.Fatal("expected sub-task to fail")
	}

	if s.State().Error("st") == "" {
		t.Fatal("expected error to be stored")
	}
}
