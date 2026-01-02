// Package testutil provides shared testing utilities for go-mehrhof tests.
package testutil

import (
	"time"

	"github.com/valksor/go-mehrhof/internal/provider"
	"github.com/valksor/go-mehrhof/internal/storage"
)

// SampleTaskContent returns a sample task markdown content.
func SampleTaskContent(title string) string {
	return `---` + "\n" + `title: ` + title + "\n" + `---` + "\n\n" +
		`This is a sample task description for testing purposes.`
}

// SampleWorkUnit returns a sample WorkUnit for testing.
func SampleWorkUnit() *provider.WorkUnit {
	return &provider.WorkUnit{
		ID:          "sample-task-123",
		Title:       "Sample Task",
		Description: "This is a sample task for testing",
		ExternalKey: "SAMPLE-123",
		Provider:    "file",
		Status:      provider.StatusOpen,
		Priority:    provider.PriorityNormal,
		Source: provider.SourceInfo{
			Type:      "file",
			Reference: "task.md",
			SyncedAt:  time.Now(),
		},
		ExternalID: "sample-task-123",
		TaskType:   "feature",
		Slug:       "sample-task",
		Metadata: map[string]any{
			"title": "Sample Task",
		},
		AgentConfig: nil,
	}
}

// SampleWorkUnitWithOptions returns a configurable WorkUnit for testing.
func SampleWorkUnitWithOptions(opts func(*provider.WorkUnit)) *provider.WorkUnit {
	wu := SampleWorkUnit()
	if opts != nil {
		opts(wu)
	}

	return wu
}

// SampleWorkspaceConfig returns a sample workspace configuration.
func SampleWorkspaceConfig() *storage.WorkspaceConfig {
	return &storage.WorkspaceConfig{
		Git: storage.GitSettings{
			BranchPattern: "feature/{key}--{slug}",
			CommitPrefix:  "[{key}]",
		},
		Agent: storage.AgentSettings{
			Default: "claude",
		},
		Providers: storage.ProvidersSettings{
			Default: "file",
		},
		Plugins: storage.PluginsConfig{
			Enabled: []string{},
			Config:  map[string]map[string]any{},
		},
		Agents: map[string]storage.AgentAliasConfig{},
		Env:    map[string]string{},
	}
}

// SampleAgentConfig returns a sample agent configuration.
func SampleAgentConfig() *provider.AgentConfig {
	return &provider.AgentConfig{
		Name: "test-agent",
		Env: map[string]string{
			"TEST_VAR": "test-value",
		},
		Args: []string{"--test-arg"},
		Steps: map[string]provider.StepAgentConfig{
			"planning": {
				Name: "planning-agent",
				Env:  map[string]string{"PLANNING_VAR": "planning-value"},
				Args: []string{"--planning-turns", "5"},
			},
		},
	}
}

// SampleSourceInfo returns a sample source info for testing.
func SampleSourceInfo() storage.SourceInfo {
	return storage.SourceInfo{
		Type:   "file",
		Ref:    "task.md",
		ReadAt: time.Now(),
	}
}

// SampleActiveTask returns a sample active task for testing.
func SampleActiveTask(taskID, title string) *storage.ActiveTask {
	return &storage.ActiveTask{
		ID:      taskID,
		Ref:     "file:task.md",
		WorkDir: ".mehrhof/work/" + taskID,
		State:   "idle",
		Branch:  "",
		UseGit:  false,
		Started: time.Now(),
	}
}

// SampleTaskWork returns sample task work for testing.
func SampleTaskWork(taskID, title string) *storage.TaskWork {
	now := time.Now()

	return &storage.TaskWork{
		Version: "1",
		Metadata: storage.WorkMetadata{
			ID:          taskID,
			Title:       title,
			ExternalKey: "SAMPLE-123",
			TaskType:    "feature",
			Slug:        "sample-task",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		Source: storage.SourceInfo{
			Type:   "file",
			Ref:    "task.md",
			ReadAt: now,
		},
		Git: storage.GitInfo{
			Branch:        "feature/sample-123--sample-task",
			BaseBranch:    "main",
			CommitPrefix:  "[SAMPLE-123]",
			BranchPattern: "feature/{key}--{slug}",
		},
		Agent: storage.AgentInfo{
			Name:   "claude",
			Source: "workspace",
		},
	}
}

// SampleSpecification returns sample specification content.
func SampleSpecification(num int) string {
	return `# Specification ` + string(rune('0'+num)) + `

## Summary
This is a sample specification for testing.

## Details
- Implement feature X
- Write tests for X
- Document the changes
`
}

// SamplePendingQuestion returns a sample pending question.
func SamplePendingQuestion() *storage.PendingQuestion {
	return &storage.PendingQuestion{
		Question:       "Should we implement option A or B?",
		Phase:          "planning",
		AskedAt:        time.Now(),
		ContextSummary: "Context summary here",
		FullContext:    "Full context here",
		Options: []storage.QuestionOption{
			{Label: "Option A", Description: "Implement A"},
			{Label: "Option B", Description: "Implement B"},
		},
	}
}

// SampleSession returns a sample session for testing.
func SampleSession() *storage.Session {
	now := time.Now()

	return &storage.Session{
		Version: "1",
		Kind:    "Session",
		Metadata: storage.SessionMetadata{
			StartedAt: now,
			EndedAt:   now.Add(5 * time.Minute),
			Type:      "planning",
			Agent:     "claude",
			State:     "idle",
		},
		Exchanges: []storage.Exchange{
			{
				Role:      "system",
				Timestamp: now,
				Content:   "You are a helpful assistant.",
			},
			{
				Role:      "user",
				Timestamp: now,
				Content:   "Plan this task.",
			},
		},
	}
}

// WithTitle sets a custom title on a WorkUnit.
func WithTitle(title string) func(*provider.WorkUnit) {
	return func(wu *provider.WorkUnit) {
		wu.Title = title
	}
}

// WithExternalKey sets a custom external key on a WorkUnit.
func WithExternalKey(key string) func(*provider.WorkUnit) {
	return func(wu *provider.WorkUnit) {
		wu.ExternalKey = key
	}
}

// WithAgentConfig sets agent configuration on a WorkUnit.
func WithAgentConfig(cfg *provider.AgentConfig) func(*provider.WorkUnit) {
	return func(wu *provider.WorkUnit) {
		wu.AgentConfig = cfg
	}
}

// WithTaskType sets the task type on a WorkUnit.
func WithTaskType(taskType string) func(*provider.WorkUnit) {
	return func(wu *provider.WorkUnit) {
		wu.TaskType = taskType
	}
}
