package conductor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/valksor/go-mehrhof/internal/agent"
	"github.com/valksor/go-mehrhof/internal/events"
	"github.com/valksor/go-mehrhof/internal/storage"
)

func TestFormatSpecContent(t *testing.T) {
	tests := []struct {
		name     string
		num      int
		response *agent.Response
		wantIn   []string
	}{
		{
			name: "with summary only",
			num:  1,
			response: &agent.Response{
				Summary: "This is the summary",
			},
			wantIn: []string{"# Specification 1", "## Summary", "This is the summary"},
		},
		{
			name: "with messages only",
			num:  2,
			response: &agent.Response{
				Messages: []string{"Message 1", "Message 2"},
			},
			wantIn: []string{"# Specification 2", "## Details", "Message 1", "Message 2"},
		},
		{
			name: "with both summary and messages",
			num:  3,
			response: &agent.Response{
				Summary:  "Summary text",
				Messages: []string{"Detail 1", "Detail 2"},
			},
			wantIn: []string{"# Specification 3", "## Summary", "Summary text", "## Details", "Detail 1", "Detail 2"},
		},
		{
			name:     "empty response",
			num:      4,
			response: &agent.Response{},
			wantIn:   []string{"# Specification 4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSpecificationContent(tt.num, tt.response)
			for _, want := range tt.wantIn {
				if !strings.Contains(got, want) {
					t.Errorf("formatSpecificationContent() missing %q in:\n%s", want, got)
				}
			}
		})
	}
}

func TestBuildPlanningPrompt(t *testing.T) {
	tests := []struct {
		name          string
		title         string
		sourceContent string
		notes         string
		existingSpecs string
		wantIn        []string
		wantNotIn     []string
	}{
		{
			name:          "basic prompt",
			title:         "Add login feature",
			sourceContent: "Task description here",
			notes:         "",
			existingSpecs: "",
			wantIn:        []string{"Add login feature", "Task description here", "## Instructions"},
			wantNotIn:     []string{"## Additional Notes", "## Previous Specifications"},
		},
		{
			name:          "with notes",
			title:         "Add login",
			sourceContent: "Task desc",
			notes:         "User wants OAuth",
			existingSpecs: "",
			wantIn:        []string{"Add login", "## Additional Notes", "User wants OAuth"},
			wantNotIn:     []string{"## Previous Specifications"},
		},
		{
			name:          "with existing specs",
			title:         "Add login",
			sourceContent: "Task desc",
			notes:         "",
			existingSpecs: "# Specification 1\nExisting spec content",
			wantIn:        []string{"## Previous Specifications", "DO NOT start from scratch", "Specification 1"},
			wantNotIn:     []string{"## Additional Notes"},
		},
		{
			name:          "with everything",
			title:         "Full task",
			sourceContent: "Full description",
			notes:         "Some notes",
			existingSpecs: "Previous spec",
			wantIn:        []string{"Full task", "Full description", "## Additional Notes", "Some notes", "## Previous Specifications", "Previous spec"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPlanningPrompt(tt.title, tt.sourceContent, tt.notes, tt.existingSpecs)
			for _, want := range tt.wantIn {
				if !strings.Contains(got, want) {
					t.Errorf("buildPlanningPrompt() missing %q", want)
				}
			}
			for _, notWant := range tt.wantNotIn {
				if strings.Contains(got, notWant) {
					t.Errorf("buildPlanningPrompt() should not contain %q", notWant)
				}
			}
		})
	}
}

func TestBuildImplementationPrompt(t *testing.T) {
	tests := []struct {
		name      string
		title     string
		source    string
		specs     string
		notes     string
		wantIn    []string
		wantNotIn []string
	}{
		{
			name:      "basic prompt",
			title:     "Add feature",
			source:    "Original requirements",
			specs:     "# Specification 1\nSpec content",
			notes:     "",
			wantIn:    []string{"Add feature", "Original requirements", "Spec content", "## Instructions"},
			wantNotIn: []string{"## Additional Notes"},
		},
		{
			name:   "with notes",
			title:  "Add feature",
			source: "Original",
			specs:  "Specs",
			notes:  "Implementation notes",
			wantIn: []string{"## Additional Notes", "Implementation notes"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildImplementationPrompt(tt.title, tt.source, tt.specs, tt.notes)
			for _, want := range tt.wantIn {
				if !strings.Contains(got, want) {
					t.Errorf("buildImplementationPrompt() missing %q", want)
				}
			}
			for _, notWant := range tt.wantNotIn {
				if strings.Contains(got, notWant) {
					t.Errorf("buildImplementationPrompt() should not contain %q", notWant)
				}
			}
		})
	}
}

func TestBuildReviewPrompt(t *testing.T) {
	got := buildReviewPrompt("Task Title", "Source content", "Spec content")

	wantIn := []string{
		"Task Title",
		"Source content",
		"Spec content",
		"## Instructions",
		"Correctness",
		"Code quality",
		"Security",
		"Performance",
		"Best practices",
	}

	for _, want := range wantIn {
		if !strings.Contains(got, want) {
			t.Errorf("buildReviewPrompt() missing %q", want)
		}
	}
}

func TestApplyFiles_Create(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	c, err := New(WithWorkDir(tmpDir))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.eventBus = events.NewBus()

	files := []agent.FileChange{
		{
			Path:      "test/new-file.txt",
			Operation: agent.FileOpCreate,
			Content:   "Hello, World!",
		},
	}

	err = applyFiles(ctx, c, files)
	if err != nil {
		t.Fatalf("applyFiles: %v", err)
	}

	// Verify file was created
	content, err := os.ReadFile(filepath.Join(tmpDir, "test/new-file.txt"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(content) != "Hello, World!" {
		t.Errorf("file content = %q, want %q", string(content), "Hello, World!")
	}
}

func TestApplyFiles_Update(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Create existing file
	existingPath := filepath.Join(tmpDir, "existing.txt")
	if err := os.WriteFile(existingPath, []byte("old content"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	c, err := New(WithWorkDir(tmpDir))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.eventBus = events.NewBus()

	files := []agent.FileChange{
		{
			Path:      "existing.txt",
			Operation: agent.FileOpUpdate,
			Content:   "new content",
		},
	}

	err = applyFiles(ctx, c, files)
	if err != nil {
		t.Fatalf("applyFiles: %v", err)
	}

	// Verify file was updated
	content, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(content) != "new content" {
		t.Errorf("file content = %q, want %q", string(content), "new content")
	}
}

func TestApplyFiles_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Create file to delete
	filePath := filepath.Join(tmpDir, "to-delete.txt")
	if err := os.WriteFile(filePath, []byte("delete me"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	c, err := New(WithWorkDir(tmpDir))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.eventBus = events.NewBus()

	files := []agent.FileChange{
		{
			Path:      "to-delete.txt",
			Operation: agent.FileOpDelete,
		},
	}

	err = applyFiles(ctx, c, files)
	if err != nil {
		t.Fatalf("applyFiles: %v", err)
	}

	// Verify file was deleted
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("file should have been deleted")
	}
}

func TestApplyFiles_DeleteSentinel(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Create file to delete
	filePath := filepath.Join(tmpDir, "sentinel-delete.txt")
	if err := os.WriteFile(filePath, []byte("delete me"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	c, err := New(WithWorkDir(tmpDir))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.eventBus = events.NewBus()

	files := []agent.FileChange{
		{
			Path:    "sentinel-delete.txt",
			Content: DeleteFileSentinel, // Use sentinel instead of operation
		},
	}

	err = applyFiles(ctx, c, files)
	if err != nil {
		t.Fatalf("applyFiles: %v", err)
	}

	// Verify file was deleted
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("file should have been deleted via sentinel")
	}
}

func TestApplyFiles_DeleteNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	c, err := New(WithWorkDir(tmpDir))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.eventBus = events.NewBus()

	files := []agent.FileChange{
		{
			Path:      "does-not-exist.txt",
			Operation: agent.FileOpDelete,
		},
	}

	// Should not error when deleting non-existent file
	err = applyFiles(ctx, c, files)
	if err != nil {
		t.Errorf("applyFiles should not error on non-existent file: %v", err)
	}
}

func TestApplyFiles_Multiple(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Create file for update and delete
	updatePath := filepath.Join(tmpDir, "update.txt")
	deletePath := filepath.Join(tmpDir, "delete.txt")
	if err := os.WriteFile(updatePath, []byte("old"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(deletePath, []byte("gone"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	c, err := New(WithWorkDir(tmpDir))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.eventBus = events.NewBus()

	files := []agent.FileChange{
		{Path: "new.txt", Operation: agent.FileOpCreate, Content: "created"},
		{Path: "update.txt", Operation: agent.FileOpUpdate, Content: "updated"},
		{Path: "delete.txt", Operation: agent.FileOpDelete},
	}

	err = applyFiles(ctx, c, files)
	if err != nil {
		t.Fatalf("applyFiles: %v", err)
	}

	// Verify create
	if content, _ := os.ReadFile(filepath.Join(tmpDir, "new.txt")); string(content) != "created" {
		t.Error("create failed")
	}

	// Verify update
	if content, _ := os.ReadFile(updatePath); string(content) != "updated" {
		t.Error("update failed")
	}

	// Verify delete
	if _, err := os.Stat(deletePath); !os.IsNotExist(err) {
		t.Error("delete failed")
	}
}

func TestApplyFiles_WithGitRoot(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	ctx := context.Background()

	// Initialize git repo
	initGitRepo(t, tmpDir)

	c, err := New(WithWorkDir(tmpDir))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.eventBus = events.NewBus()

	// Initialize to set git
	_ = c.Initialize(ctx)

	files := []agent.FileChange{
		{
			Path:      "git-tracked.txt",
			Operation: agent.FileOpCreate,
			Content:   "tracked by git",
		},
	}

	err = applyFiles(ctx, c, files)
	if err != nil {
		t.Fatalf("applyFiles: %v", err)
	}

	// Verify file was created in git root
	content, err := os.ReadFile(filepath.Join(tmpDir, "git-tracked.txt"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(content) != "tracked by git" {
		t.Errorf("file content = %q, want %q", string(content), "tracked by git")
	}
}

func TestDeleteFileSentinelConstant(t *testing.T) {
	if DeleteFileSentinel != "__DELETE_FILE__" {
		t.Errorf("DeleteFileSentinel = %q, want %q", DeleteFileSentinel, "__DELETE_FILE__")
	}
}

func TestErrPendingQuestion(t *testing.T) {
	if ErrPendingQuestion == nil {
		t.Fatal("ErrPendingQuestion should not be nil")
	}
	if ErrPendingQuestion.Error() != "agent has a pending question" {
		t.Errorf("ErrPendingQuestion.Error() = %q", ErrPendingQuestion.Error())
	}
}

func TestCreateCheckpointIfNeeded_NoGit(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// No git - should return nil
	c.activeTask = &storage.ActiveTask{
		ID:     "test",
		UseGit: true,
	}
	event := c.createCheckpointIfNeeded("test", "message")
	if event != nil {
		t.Error("createCheckpointIfNeeded should return nil when git is nil")
	}
}

func TestCreateCheckpointIfNeeded_GitNotUsed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)
	ctx := context.Background()

	c, err := New(WithWorkDir(tmpDir))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Initialize to set git
	_ = c.Initialize(ctx)

	// UseGit is false
	c.activeTask = &storage.ActiveTask{
		ID:     "test",
		UseGit: false,
	}

	event := c.createCheckpointIfNeeded("test", "message")
	if event != nil {
		t.Error("createCheckpointIfNeeded should return nil when UseGit is false")
	}
}

func TestCreateCheckpointIfNeeded_NoChanges(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)
	ctx := context.Background()

	c, err := New(WithWorkDir(tmpDir))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Initialize to set git
	_ = c.Initialize(ctx)

	c.activeTask = &storage.ActiveTask{
		ID:      "test",
		UseGit:  true,
		Started: time.Now(),
	}

	// No changes - should return nil
	event := c.createCheckpointIfNeeded("test", "message")
	if event != nil {
		t.Error("createCheckpointIfNeeded should return nil when no changes")
	}
}
