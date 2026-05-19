package provider

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/valksor/kvelmo/settings"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		wantProv string
		wantID   string
		wantErr  bool
	}{
		{
			name:     "file prefix",
			source:   "file:./tasks/task.md",
			wantProv: "file",
			wantID:   "./tasks/task.md",
		},
		{
			name:     "github shorthand",
			source:   "github:owner/repo#123",
			wantProv: "github",
			wantID:   "owner/repo#123",
		},
		{
			name:     "gitlab shorthand",
			source:   "gitlab:owner/repo#456",
			wantProv: "gitlab",
			wantID:   "owner/repo#456",
		},
		{
			name:     "wrike shorthand",
			source:   "wrike:IEAAXYZ",
			wantProv: "wrike",
			wantID:   "IEAAXYZ",
		},
		{
			name:     "empty shorthand",
			source:   "empty:Fix the login button",
			wantProv: "empty",
			wantID:   "Fix the login button",
		},
		{
			name:     "linear shorthand",
			source:   "linear:ENG-123",
			wantProv: "linear",
			wantID:   "ENG-123",
		},
		{
			name:     "linear short alias",
			source:   "ln:ENG-456",
			wantProv: "linear",
			wantID:   "ENG-456",
		},
		{
			name:     "github URL issue",
			source:   "https://github.com/owner/repo/issues/123",
			wantProv: "github",
			wantID:   "owner/repo#123",
		},
		{
			name:     "github URL pull",
			source:   "https://github.com/owner/repo/pull/456",
			wantProv: "github",
			wantID:   "owner/repo#456",
		},
		{
			name:    "unknown format",
			source:  "random-string",
			wantErr: true,
		},
		{
			name:    "unsupported URL",
			source:  "https://example.com/something",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov, id, err := Parse(tt.source)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if prov != tt.wantProv {
				t.Errorf("provider = %s, want %s", prov, tt.wantProv)
			}

			if id != tt.wantID {
				t.Errorf("id = %s, want %s", id, tt.wantID)
			}
		})
	}
}

func TestRegistry(t *testing.T) {
	r := NewRegistry(settings.DefaultSettings())

	// Should have default providers
	file, err := r.Get("file")
	if err != nil {
		t.Errorf("Get(file) error = %v", err)
	}
	if file == nil {
		t.Error("file provider should not be nil")
	}

	github, err := r.Get("github")
	if err != nil {
		t.Errorf("Get(github) error = %v", err)
	}
	if github == nil {
		t.Error("github provider should not be nil")
	}

	// Unknown provider should error
	_, err = r.Get("unknown")
	if err == nil {
		t.Error("Get(unknown) should return error")
	}
}

func TestRegistryParse(t *testing.T) {
	r := NewRegistry(settings.DefaultSettings())

	prov, id, err := r.Parse("github:owner/repo#123")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if prov != "github" {
		t.Errorf("provider = %s, want github", prov)
	}

	if id != "owner/repo#123" {
		t.Errorf("id = %s, want owner/repo#123", id)
	}
}

func TestFileProvider(t *testing.T) {
	fp := NewFileProvider()

	if fp.Name() != "file" {
		t.Errorf("Name() = %s, want file", fp.Name())
	}

	// Create temp file
	dir := t.TempDir()
	taskFile := filepath.Join(dir, "task.md")
	content := `# Test Task

This is a test task description.

## Requirements
- Requirement 1
- Requirement 2
`
	if err := os.WriteFile(taskFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	ctx := context.Background()
	task, err := fp.FetchTask(ctx, taskFile)
	if err != nil {
		t.Fatalf("FetchTask() error = %v", err)
	}

	if task.Title != "Test Task" {
		t.Errorf("task.Title = %s, want 'Test Task'", task.Title)
	}

	if task.Source != "file" {
		t.Errorf("task.Source = %s, want file", task.Source)
	}

	if task.Description == "" {
		t.Error("task.Description should not be empty")
	}
}

func TestFileProviderNotFound(t *testing.T) {
	fp := NewFileProvider()

	ctx := context.Background()
	_, err := fp.FetchTask(ctx, "/nonexistent/path/task.md")
	if err == nil {
		t.Error("FetchTask() should error for nonexistent file")
	}
}

func TestFileProviderUpdateStatus(t *testing.T) {
	fp := NewFileProvider()

	ctx := context.Background()
	// UpdateStatus is a no-op for file provider
	err := fp.UpdateStatus(ctx, "any-id", "done")
	if err != nil {
		t.Errorf("UpdateStatus() error = %v", err)
	}
}

func TestGitHubProvider(t *testing.T) {
	gp := NewGitHubProvider("")

	if gp.Name() != "github" {
		t.Errorf("Name() = %s, want github", gp.Name())
	}
}

func TestGitHubProviderParseID(t *testing.T) {
	tests := []struct {
		id        string
		wantOwner string
		wantRepo  string
		wantNum   string
	}{
		{"owner/repo#123", "owner", "repo", "123"},
		{"my-org/my-repo#456", "my-org", "my-repo", "456"},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			owner, repo, num := parseGitHubID(tt.id)
			if owner != tt.wantOwner {
				t.Errorf("owner = %s, want %s", owner, tt.wantOwner)
			}
			if repo != tt.wantRepo {
				t.Errorf("repo = %s, want %s", repo, tt.wantRepo)
			}
			if num != tt.wantNum {
				t.Errorf("num = %s, want %s", num, tt.wantNum)
			}
		})
	}
}

func TestGitLabProvider(t *testing.T) {
	gp, err := NewGitLabProvider("")
	if err != nil {
		t.Fatalf("NewGitLabProvider() error = %v", err)
	}

	if gp.Name() != "gitlab" {
		t.Errorf("Name() = %s, want gitlab", gp.Name())
	}
}

func TestWrikeProvider(t *testing.T) {
	wp := NewWrikeProvider("")

	if wp.Name() != "wrike" {
		t.Errorf("Name() = %s, want wrike", wp.Name())
	}
}

func TestParse_URLVariants(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		wantProv string
		wantID   string
		wantErr  bool
	}{
		{
			name:     "gitlab URL issue",
			source:   "https://gitlab.com/owner/repo/-/issues/123",
			wantProv: "gitlab",
			wantID:   "owner/repo#123",
		},
		{
			name:     "gitlab URL merge request",
			source:   "https://gitlab.com/owner/repo/-/merge_requests/45",
			wantProv: "gitlab",
			wantID:   "owner/repo!45",
		},
		{
			name:    "gitlab URL no issue path",
			source:  "https://gitlab.com/owner/repo",
			wantErr: true,
		},
		{
			name:     "wrike URL task prefix",
			source:   "https://www.wrike.com/open.htm/task-12345",
			wantProv: "wrike",
			wantID:   "task-12345",
		},
		{
			name:     "wrike URL IEAA prefix",
			source:   "https://www.wrike.com/workspaces/IEAABC123",
			wantProv: "wrike",
			wantID:   "IEAABC123",
		},
		{
			name:    "wrike URL no task ID",
			source:  "https://www.wrike.com/home/dashboard",
			wantErr: true,
		},
		{
			name:    "github URL short path",
			source:  "https://github.com/owner/repo",
			wantErr: true,
		},
		{
			name:     "linear URL",
			source:   "https://linear.app/myteam/issue/ENG-123-fix-login-bug",
			wantProv: "linear",
			wantID:   "ENG-123",
		},
		{
			name:     "linear URL no slug",
			source:   "https://linear.app/myteam/issue/ENG-456",
			wantProv: "linear",
			wantID:   "ENG-456",
		},
		{
			name:    "linear URL no issue path",
			source:  "https://linear.app/myteam/settings",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov, id, err := Parse(tt.source)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}

				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if prov != tt.wantProv {
				t.Errorf("provider = %s, want %s", prov, tt.wantProv)
			}
			if id != tt.wantID {
				t.Errorf("id = %s, want %s", id, tt.wantID)
			}
		})
	}
}

func TestRegistryHasAllProviders(t *testing.T) {
	r := NewRegistry(settings.DefaultSettings())

	providers := []string{"file", "github", "gitlab", "wrike", "empty"}
	for _, name := range providers {
		p, err := r.Get(name)
		if err != nil {
			t.Errorf("Get(%s) error = %v", name, err)
		}
		if p == nil {
			t.Errorf("%s provider should not be nil", name)
		}
	}
}

func TestEmptyProvider(t *testing.T) {
	ep := NewEmptyProvider()

	if ep.Name() != "empty" {
		t.Errorf("Name() = %s, want empty", ep.Name())
	}

	ctx := context.Background()

	// Test with description
	task, err := ep.FetchTask(ctx, "Fix the login button styling")
	if err != nil {
		t.Fatalf("FetchTask() error = %v", err)
	}

	if task.Title == "" {
		t.Error("task.Title should not be empty")
	}

	if task.Description != "Fix the login button styling" {
		t.Errorf("task.Description = %s, want 'Fix the login button styling'", task.Description)
	}

	if task.Source != "empty" {
		t.Errorf("task.Source = %s, want empty", task.Source)
	}

	// Test with empty description should error
	_, err = ep.FetchTask(ctx, "")
	if err == nil {
		t.Error("FetchTask() should error for empty description")
	}
}

func TestEmptyProviderTruncateTitle(t *testing.T) {
	tests := []struct {
		desc     string
		expected string
	}{
		{"Short title", "Short title"},
		{"This is a very long description that exceeds the maximum title length and should be truncated to fit within the limit", "This is a very long description that exceeds the maximum title length and sho..."},
		{"First line\nSecond line", "First line"},
	}

	for _, tt := range tests {
		result := truncateTitle(tt.desc)
		if result != tt.expected {
			t.Errorf("truncateTitle(%q) = %q, want %q", tt.desc, result, tt.expected)
		}
	}
}

func TestTask(t *testing.T) {
	task := &Task{
		ID:          "123",
		Title:       "Test Task",
		Description: "A test task",
		URL:         "https://github.com/owner/repo/issues/123",
		Labels:      []string{"bug", "priority"},
		Source:      "github",
	}

	if task.ID != "123" {
		t.Errorf("task.ID = %s, want 123", task.ID)
	}

	if len(task.Labels) != 2 {
		t.Errorf("len(task.Labels) = %d, want 2", len(task.Labels))
	}
}

func TestTaskInferenceFields(t *testing.T) {
	task := &Task{ID: "test-1", Priority: "p1", Type: "bug", Slug: "fix-login"}
	if task.Priority != "p1" {
		t.Errorf("Priority = %s, want p1", task.Priority)
	}
	if task.Type != "bug" {
		t.Errorf("Type = %s, want bug", task.Type)
	}
	if task.Slug != "fix-login" {
		t.Errorf("Slug = %s, want fix-login", task.Slug)
	}
}

func TestSubtask(t *testing.T) {
	subtask := &Subtask{
		ID:        "task-123-task-0",
		Text:      "Implement login validation",
		Completed: false,
		Index:     0,
	}
	if subtask.ID != "task-123-task-0" {
		t.Errorf("Subtask.ID = %s, want task-123-task-0", subtask.ID)
	}
	if subtask.Text != "Implement login validation" {
		t.Errorf("Subtask.Text = %s, want 'Implement login validation'", subtask.Text)
	}
	if subtask.Completed != false {
		t.Errorf("Subtask.Completed = %v, want false", subtask.Completed)
	}
	if subtask.Index != 0 {
		t.Errorf("Subtask.Index = %d, want 0", subtask.Index)
	}
}

func TestTaskSubtasks(t *testing.T) {
	task := &Task{
		ID:    "task-123",
		Title: "Test Task",
		Subtasks: []*Subtask{
			{ID: "task-123-task-0", Text: "First subtask", Completed: true, Index: 0},
			{ID: "task-123-task-1", Text: "Second subtask", Completed: false, Index: 1},
		},
	}
	if len(task.Subtasks) != 2 {
		t.Errorf("len(task.Subtasks) = %d, want 2", len(task.Subtasks))
	}
	if task.Subtasks[0].Completed != true {
		t.Errorf("Subtasks[0].Completed = %v, want true", task.Subtasks[0].Completed)
	}
	if task.Subtasks[1].Index != 1 {
		t.Errorf("Subtasks[1].Index = %d, want 1", task.Subtasks[1].Index)
	}
}

func TestTaskDependencies(t *testing.T) {
	dep1 := &Task{ID: "dep-1", Title: "Dependency 1", Source: "github"}
	dep2 := &Task{ID: "dep-2", Title: "Dependency 2", Source: "github"}

	task := &Task{
		ID:           "task-123",
		Title:        "Main Task",
		Dependencies: []*Task{dep1, dep2},
	}

	if len(task.Dependencies) != 2 {
		t.Errorf("len(task.Dependencies) = %d, want 2", len(task.Dependencies))
	}
	if task.Dependencies[0].ID != "dep-1" {
		t.Errorf("Dependencies[0].ID = %s, want dep-1", task.Dependencies[0].ID)
	}
	if task.Dependencies[1].Title != "Dependency 2" {
		t.Errorf("Dependencies[1].Title = %s, want 'Dependency 2'", task.Dependencies[1].Title)
	}
}

// --- SubtaskProvider and DependencyProvider interface tests ---

// mockSubtaskProvider implements SubtaskProvider for testing.
type mockSubtaskProvider struct {
	name     string
	task     *Task
	subtasks []*Subtask
	fetchErr error
}

func (p *mockSubtaskProvider) Name() string { return p.name }
func (p *mockSubtaskProvider) FetchTask(_ context.Context, _ string) (*Task, error) {
	return p.task, nil
}
func (p *mockSubtaskProvider) UpdateStatus(_ context.Context, _, _ string) error { return nil }
func (p *mockSubtaskProvider) FetchSubtasks(_ context.Context, _ *Task) ([]*Subtask, error) {
	return p.subtasks, p.fetchErr
}

func TestSubtaskProviderInterface(t *testing.T) {
	subtasks := []*Subtask{
		{ID: "task-1-task-0", Text: "First item", Completed: true, Index: 0},
		{ID: "task-1-task-1", Text: "Second item", Completed: false, Index: 1},
	}
	task := &Task{ID: "task-1", Title: "Test Task", Source: "mock"}

	provider := &mockSubtaskProvider{
		name:     "mock-subtask",
		task:     task,
		subtasks: subtasks,
	}

	// Verify it implements SubtaskProvider
	var _ SubtaskProvider = provider

	ctx := context.Background()
	result, err := provider.FetchSubtasks(ctx, task)
	if err != nil {
		t.Fatalf("FetchSubtasks() error = %v", err)
	}
	if len(result) != 2 {
		t.Errorf("FetchSubtasks() returned %d subtasks, want 2", len(result))
	}
	if result[0].Completed != true {
		t.Errorf("result[0].Completed = %v, want true", result[0].Completed)
	}
}

// mockDependencyProvider implements DependencyProvider for testing.
type mockDependencyProvider struct {
	name         string
	task         *Task
	dependencies []*Task
	fetchErr     error
	createErr    error
	createdDeps  []struct{ taskID, dependsOnID string }
}

func (p *mockDependencyProvider) Name() string { return p.name }
func (p *mockDependencyProvider) FetchTask(_ context.Context, _ string) (*Task, error) {
	return p.task, nil
}
func (p *mockDependencyProvider) UpdateStatus(_ context.Context, _, _ string) error { return nil }
func (p *mockDependencyProvider) FetchDependencies(_ context.Context, _ *Task) ([]*Task, error) {
	return p.dependencies, p.fetchErr
}

func (p *mockDependencyProvider) CreateDependency(_ context.Context, taskID, dependsOnID string) error {
	p.createdDeps = append(p.createdDeps, struct{ taskID, dependsOnID string }{taskID, dependsOnID})

	return p.createErr
}

func TestDependencyProviderInterface(t *testing.T) {
	dep1 := &Task{ID: "dep-1", Title: "Dependency 1", Source: "mock"}
	dep2 := &Task{ID: "dep-2", Title: "Dependency 2", Source: "mock"}
	task := &Task{ID: "task-1", Title: "Main Task", Source: "mock"}

	provider := &mockDependencyProvider{
		name:         "mock-dependency",
		task:         task,
		dependencies: []*Task{dep1, dep2},
	}

	// Verify it implements DependencyProvider
	var _ DependencyProvider = provider

	ctx := context.Background()

	// Test FetchDependencies
	deps, err := provider.FetchDependencies(ctx, task)
	if err != nil {
		t.Fatalf("FetchDependencies() error = %v", err)
	}
	if len(deps) != 2 {
		t.Errorf("FetchDependencies() returned %d deps, want 2", len(deps))
	}
	if deps[0].ID != "dep-1" {
		t.Errorf("deps[0].ID = %s, want dep-1", deps[0].ID)
	}

	// Test CreateDependency
	err = provider.CreateDependency(ctx, "task-1", "dep-3")
	if err != nil {
		t.Fatalf("CreateDependency() error = %v", err)
	}
	if len(provider.createdDeps) != 1 {
		t.Errorf("CreateDependency() should record 1 dep, got %d", len(provider.createdDeps))
	}
	if provider.createdDeps[0].taskID != "task-1" || provider.createdDeps[0].dependsOnID != "dep-3" {
		t.Errorf("CreateDependency() recorded wrong values: %+v", provider.createdDeps[0])
	}
}

// Helper to parse GitHub ID format.
//
//nolint:nonamedreturns // Named returns document the return values
func parseGitHubID(id string) (owner, repo, num string) {
	// Format: owner/repo#num
	for i, c := range id {
		if c == '/' {
			owner = id[:i]
			rest := id[i+1:]
			for j, c2 := range rest {
				if c2 == '#' {
					repo = rest[:j]
					num = rest[j+1:]

					return
				}
			}
		}
	}

	return
}

// --- Task metadata tests ---

func TestTaskSetMetadata(t *testing.T) {
	task := &Task{ID: "task-1"}
	task.SetMetadata("parent_id", "folder-42")
	if task.Metadata("parent_id") != "folder-42" {
		t.Errorf("Metadata(parent_id) = %q, want %q", task.Metadata("parent_id"), "folder-42")
	}
}

func TestTaskMetadata_Missing(t *testing.T) {
	task := &Task{ID: "task-1"}
	if got := task.Metadata("nonexistent"); got != "" {
		t.Errorf("Metadata(nonexistent) = %q, want empty string", got)
	}
}

func TestTaskSetMetadata_NilMapInit(t *testing.T) {
	// metadata map starts nil; SetMetadata should initialise it
	task := &Task{}
	task.SetMetadata("key", "value")
	task.SetMetadata("key2", "value2")
	if task.Metadata("key") != "value" {
		t.Errorf("Metadata(key) = %q, want value", task.Metadata("key"))
	}
	if task.Metadata("key2") != "value2" {
		t.Errorf("Metadata(key2) = %q, want value2", task.Metadata("key2"))
	}
}

func TestEmptyProviderUpdateStatus(t *testing.T) {
	ep := NewEmptyProvider()
	ctx := context.Background()
	if err := ep.UpdateStatus(ctx, "any-id", "done"); err != nil {
		t.Errorf("UpdateStatus() error = %v, want nil", err)
	}
}

func TestRegistryFetchTask_InvalidSource(t *testing.T) {
	r := NewRegistry(settings.DefaultSettings())
	_, err := r.FetchTask(context.Background(), "invalid-source-no-prefix")
	if err == nil {
		t.Error("FetchTask() with invalid source should return error")
	}
}

func TestRegistryFetch_UnknownProvider(t *testing.T) {
	r := NewRegistry(settings.DefaultSettings())
	_, err := r.Fetch(context.Background(), "unknown-provider-xyz", "task-1")
	if err == nil {
		t.Error("Fetch() with unknown provider should return error")
	}
}

// --- FetchWithHierarchy tests ---

// nonHierarchyProvider is a minimal Provider without HierarchyProvider support.
type nonHierarchyProvider struct {
	name string
	task *Task
}

func (p *nonHierarchyProvider) Name() string { return p.name }
func (p *nonHierarchyProvider) FetchTask(_ context.Context, _ string) (*Task, error) {
	return p.task, nil
}
func (p *nonHierarchyProvider) UpdateStatus(_ context.Context, _, _ string) error { return nil }

// trackingHierarchyProvider records whether FetchParent/FetchSiblings were called.
type trackingHierarchyProvider struct {
	name           string
	task           *Task
	parentCalled   bool
	siblingsCalled bool
	parentResult   *Task
	siblingsResult []*Task
}

func (p *trackingHierarchyProvider) Name() string { return p.name }
func (p *trackingHierarchyProvider) FetchTask(_ context.Context, _ string) (*Task, error) {
	return p.task, nil
}

func (p *trackingHierarchyProvider) UpdateStatus(_ context.Context, _, _ string) error { return nil }

func (p *trackingHierarchyProvider) FetchParent(_ context.Context, _ *Task) (*Task, error) {
	p.parentCalled = true

	return p.parentResult, nil
}

func (p *trackingHierarchyProvider) FetchSiblings(_ context.Context, _ *Task) ([]*Task, error) {
	p.siblingsCalled = true

	return p.siblingsResult, nil
}

func TestFetchWithHierarchyNonHierarchyProvider(t *testing.T) {
	ctx := context.Background()

	baseTask := &Task{
		ID:          "task-1",
		Title:       "Base Task",
		Description: "A simple task with no hierarchy support",
		Source:      "custom",
	}

	p := &nonHierarchyProvider{name: "custom", task: baseTask}
	r := NewRegistry(settings.DefaultSettings())
	r.Register(p)

	opts := HierarchyOptions{
		IncludeParent:   true,
		IncludeSiblings: true,
	}

	result, err := r.FetchWithHierarchy(ctx, "custom", "task-1", opts)
	if err != nil {
		t.Fatalf("FetchWithHierarchy() error = %v", err)
	}
	if result == nil {
		t.Fatal("FetchWithHierarchy() returned nil task")
	}
	if result.ParentTask != nil {
		t.Errorf("expected ParentTask to be nil for non-hierarchy provider, got %+v", result.ParentTask)
	}
	if result.SiblingTasks != nil {
		t.Errorf("expected SiblingTasks to be nil for non-hierarchy provider, got %+v", result.SiblingTasks)
	}
}

func TestFetchWithHierarchyOptionsDisabled(t *testing.T) {
	ctx := context.Background()

	baseTask := &Task{
		ID:          "task-2",
		Title:       "Task With Hierarchy Provider But Opts Disabled",
		Description: "Provider supports hierarchy but opts are false",
		Source:      "tracking",
	}
	parentTask := &Task{ID: "parent-1", Title: "Parent", Source: "tracking"}
	sibling := &Task{ID: "sibling-1", Title: "Sibling", Source: "tracking"}

	tp := &trackingHierarchyProvider{
		name:           "tracking",
		task:           baseTask,
		parentResult:   parentTask,
		siblingsResult: []*Task{sibling},
	}

	r := NewRegistry(settings.DefaultSettings())
	r.Register(tp)

	opts := HierarchyOptions{
		IncludeParent:   false,
		IncludeSiblings: false,
	}

	result, err := r.FetchWithHierarchy(ctx, "tracking", "task-2", opts)
	if err != nil {
		t.Fatalf("FetchWithHierarchy() error = %v", err)
	}

	if tp.parentCalled {
		t.Error("FetchParent should not have been called when IncludeParent=false")
	}
	if tp.siblingsCalled {
		t.Error("FetchSiblings should not have been called when IncludeSiblings=false")
	}
	if result.ParentTask != nil {
		t.Errorf("expected ParentTask to be nil when IncludeParent=false, got %+v", result.ParentTask)
	}
	if result.SiblingTasks != nil {
		t.Errorf("expected SiblingTasks to be nil when IncludeSiblings=false, got %+v", result.SiblingTasks)
	}
}

func TestFetchWithHierarchyOptionsEnabled(t *testing.T) {
	ctx := context.Background()

	baseTask := &Task{
		ID:          "task-3",
		Title:       "Task With Hierarchy",
		Description: "Provider supports hierarchy and opts are true",
		Source:      "tracking2",
	}
	parentTask := &Task{ID: "parent-2", Title: "Parent Task", Source: "tracking2"}
	sibling := &Task{ID: "sibling-2", Title: "Sibling Task", Source: "tracking2"}

	tp := &trackingHierarchyProvider{
		name:           "tracking2",
		task:           baseTask,
		parentResult:   parentTask,
		siblingsResult: []*Task{sibling},
	}

	r := NewRegistry(settings.DefaultSettings())
	r.Register(tp)

	opts := HierarchyOptions{
		IncludeParent:   true,
		IncludeSiblings: true,
	}

	result, err := r.FetchWithHierarchy(ctx, "tracking2", "task-3", opts)
	if err != nil {
		t.Fatalf("FetchWithHierarchy() error = %v", err)
	}

	if !tp.parentCalled {
		t.Error("FetchParent should have been called when IncludeParent=true")
	}
	if !tp.siblingsCalled {
		t.Error("FetchSiblings should have been called when IncludeSiblings=true")
	}
	if result.ParentTask == nil {
		t.Error("expected ParentTask to be set when IncludeParent=true")
	} else if result.ParentTask.ID != "parent-2" {
		t.Errorf("ParentTask.ID = %q, want %q", result.ParentTask.ID, "parent-2")
	}
	if len(result.SiblingTasks) != 1 {
		t.Errorf("expected 1 sibling, got %d", len(result.SiblingTasks))
	}
}

// errorHierarchyProvider implements HierarchyProvider but always errors on hierarchy calls.
type errorHierarchyProvider struct {
	name string
	task *Task
}

func (p *errorHierarchyProvider) Name() string { return p.name }
func (p *errorHierarchyProvider) FetchTask(_ context.Context, _ string) (*Task, error) {
	return p.task, nil
}
func (p *errorHierarchyProvider) UpdateStatus(_ context.Context, _, _ string) error { return nil }
func (p *errorHierarchyProvider) FetchParent(_ context.Context, _ *Task) (*Task, error) {
	return nil, errors.New("parent fetch failed")
}

func (p *errorHierarchyProvider) FetchSiblings(_ context.Context, _ *Task) ([]*Task, error) {
	return nil, errors.New("siblings fetch failed")
}

func TestFetchWithHierarchy_BestEffortErrorSwallowing(t *testing.T) {
	r := NewRegistry(settings.DefaultSettings())
	p := &errorHierarchyProvider{
		name: "hierr",
		task: &Task{ID: "t1", Source: "hierr"},
	}
	r.Register(p)

	opts := HierarchyOptions{IncludeParent: true, IncludeSiblings: true}
	result, err := r.FetchWithHierarchy(context.Background(), "hierr", "t1", opts)
	if err != nil {
		t.Fatalf("FetchWithHierarchy() error = %v, want nil (errors are best-effort)", err)
	}
	if result.ParentTask != nil {
		t.Error("ParentTask should be nil when FetchParent errors (best-effort)")
	}
	if result.SiblingTasks != nil {
		t.Error("SiblingTasks should be nil when FetchSiblings errors (best-effort)")
	}
}

func TestRegistryFetchTask_ProviderNotFound_AfterParse(t *testing.T) {
	// Create empty registry — Parse("github:...") succeeds but r.Get("github") fails.
	r := &Registry{providers: make(map[string]Provider)}
	_, err := r.FetchTask(context.Background(), "github:owner/repo#1")
	if err == nil {
		t.Error("FetchTask() should return error when provider not registered")
	}
}

func TestFileProvider_NoHeading(t *testing.T) {
	fp := NewFileProvider()
	dir := t.TempDir()
	// File with no "# " heading — title falls back to basename
	taskFile := filepath.Join(dir, "plain.md")
	if err := os.WriteFile(taskFile, []byte("Just plain content\nno heading here"), 0o644); err != nil {
		t.Fatal(err)
	}
	task, err := fp.FetchTask(context.Background(), taskFile)
	if err != nil {
		t.Fatalf("FetchTask() error = %v", err)
	}
	if task.Title != "plain.md" {
		t.Errorf("Title = %q, want plain.md (basename when no heading)", task.Title)
	}
	if task.Description == "" {
		t.Error("Description should not be empty")
	}
}
