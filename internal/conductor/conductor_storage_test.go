package conductor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/storage"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// newConductorWithStore creates a conductor backed by a real store rooted at dir.
func newConductorWithStore(t *testing.T, dir string) *Conductor {
	t.Helper()
	c, err := New(WithWorkDir(dir))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	store := storage.NewStore(dir, true) // saveInProject=true → keeps files inside dir
	c.SetStore(store)

	return c
}

// ─── persistState + LoadState round-trip ─────────────────────────────────────

func TestPersistAndLoadState_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	c := newConductorWithStore(t, dir)

	// Set up a work unit and force a known state
	wu := &WorkUnit{
		ID:          "task-roundtrip",
		Title:       "Round-trip Test",
		Description: "Testing persistence",
		Branch:      "feature/roundtrip",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Metadata:    map[string]string{"key": "value"},
	}
	c.ForceWorkUnit(wu)
	c.machine.ForceState(StatePlanned)

	// Persist
	c.persistState()

	// Create a fresh conductor with the same store
	c2, err := New(WithWorkDir(dir))
	if err != nil {
		t.Fatalf("New() for second conductor error = %v", err)
	}
	c2.SetStore(storage.NewStore(dir, true))

	// Load state
	if err := c2.LoadState(context.Background()); err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}

	// Verify state restored
	if c2.State() != StatePlanned {
		t.Errorf("LoadState() state = %s, want planned", c2.State())
	}
	if c2.workUnit == nil {
		t.Fatal("LoadState() workUnit should not be nil")
	}
	if c2.workUnit.ID != "task-roundtrip" {
		t.Errorf("LoadState() workUnit.ID = %q, want task-roundtrip", c2.workUnit.ID)
	}
	if c2.workUnit.Title != "Round-trip Test" {
		t.Errorf("LoadState() workUnit.Title = %q, want 'Round-trip Test'", c2.workUnit.Title)
	}
	if c2.workUnit.Branch != "feature/roundtrip" {
		t.Errorf("LoadState() workUnit.Branch = %q, want feature/roundtrip", c2.workUnit.Branch)
	}
}

func TestPersistState_NilStore(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{ID: "ps-1", Title: "T"})
	// store is nil — persistState should be a no-op, no panic
	c.persistState()
}

func TestPersistState_NilWorkUnit(t *testing.T) {
	dir := t.TempDir()
	c := newConductorWithStore(t, dir)
	// workUnit is nil — persistState should be a no-op, no panic
	c.persistState()
}

// ─── archiveTask ─────────────────────────────────────────────────────────────

func TestArchiveTask_NilStore(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{ID: "at-1", Title: "T"})
	// No panic expected with nil store
	c.archiveTask("finished")
}

func TestArchiveTask_NilWorkUnit(t *testing.T) {
	dir := t.TempDir()
	c := newConductorWithStore(t, dir)
	// No panic expected with nil workUnit
	c.archiveTask("finished")
}

func TestArchiveTask_ValidStoreAndWorkUnit(t *testing.T) {
	dir := t.TempDir()
	c := newConductorWithStore(t, dir)

	wu := &WorkUnit{
		ID:        "task-archive-1",
		Title:     "Archived Task",
		Branch:    "feature/archive",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Source:    &Source{Provider: "github", Reference: "github:owner/repo#1"},
	}
	c.ForceWorkUnit(wu)

	// archiveTask requires c.mu held (per its contract)
	c.mu.Lock()
	c.archiveTask("finished")
	c.mu.Unlock()

	// Verify archived task appears in store
	tasks, err := c.store.ListArchivedTasks()
	if err != nil {
		t.Fatalf("ListArchivedTasks() error = %v", err)
	}
	if len(tasks) == 0 {
		t.Fatal("archiveTask() should have created an archived task entry")
	}
	found := false
	for _, task := range tasks {
		if task.ID == "task-archive-1" {
			found = true
			if task.Title != "Archived Task" {
				t.Errorf("archived task Title = %q, want 'Archived Task'", task.Title)
			}
			if task.FinalState != "finished" {
				t.Errorf("archived task FinalState = %q, want 'finished'", task.FinalState)
			}
			if task.Branch != "feature/archive" {
				t.Errorf("archived task Branch = %q, want 'feature/archive'", task.Branch)
			}
		}
	}
	if !found {
		t.Error("archiveTask() task not found in archive")
	}
}

// ─── saveJobSession ───────────────────────────────────────────────────────────

func TestSaveJobSession_StoreAndWorkUnit(t *testing.T) {
	dir := t.TempDir()
	c := newConductorWithStore(t, dir)

	wu := &WorkUnit{
		ID:    "task-session-1",
		Title: "Session Test",
	}
	c.ForceWorkUnit(wu)

	c.saveJobSession("job-abc-123", "plan", "claude")

	// Verify session was saved via storage API
	sessStore := storage.NewSessionStore(c.store)
	entry, err := sessStore.GetSession("task-session-1", "plan")
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if entry == nil {
		t.Fatal("saveJobSession() should have persisted a session entry")
	}
	if entry.SessionID != "job-abc-123" {
		t.Errorf("session SessionID = %q, want job-abc-123", entry.SessionID)
	}
	if entry.AgentType != "claude" {
		t.Errorf("session AgentType = %q, want claude", entry.AgentType)
	}
	if entry.TaskID != "task-session-1" {
		t.Errorf("session TaskID = %q, want task-session-1", entry.TaskID)
	}
	if entry.Phase != "plan" {
		t.Errorf("session Phase = %q, want plan", entry.Phase)
	}
}

func TestSaveJobSession_NilStore(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{ID: "sjs-1", Title: "T"})
	// Should not panic
	c.saveJobSession("job-1", "plan", "claude")
}

func TestSaveJobSession_NilWorkUnit(t *testing.T) {
	dir := t.TempDir()
	c := newConductorWithStore(t, dir)
	// workUnit is nil — should not panic
	c.saveJobSession("job-1", "plan", "claude")
}

// ─── getSpecificationPath ─────────────────────────────────────────────────────

func TestGetSpecificationPath_StoreAndWorkUnit(t *testing.T) {
	dir := t.TempDir()
	c := newConductorWithStore(t, dir)

	wu := &WorkUnit{
		ID:    "task-spec-path-1",
		Title: "Spec Path Test",
	}
	c.ForceWorkUnit(wu)

	path := c.getSpecificationPath()
	if path == "" {
		t.Error("getSpecificationPath() should return non-empty path with store and workUnit")
	}
	// Should contain the task ID and be a .md file
	if !filepath.IsAbs(path) {
		t.Errorf("getSpecificationPath() = %q, want absolute path", path)
	}
	if filepath.Ext(path) != ".md" {
		t.Errorf("getSpecificationPath() = %q, want .md extension", path)
	}
}

func TestGetSpecificationPath_NilStore(t *testing.T) {
	c, _ := New()
	c.ForceWorkUnit(&WorkUnit{ID: "task-spec-1", Title: "T"})
	// store is nil → returns ""
	path := c.getSpecificationPath()
	if path != "" {
		t.Errorf("getSpecificationPath() with nil store = %q, want empty string", path)
	}
}

func TestGetSpecificationPath_NilWorkUnit(t *testing.T) {
	dir := t.TempDir()
	c := newConductorWithStore(t, dir)
	// workUnit is nil → returns ""
	path := c.getSpecificationPath()
	if path != "" {
		t.Errorf("getSpecificationPath() with nil workUnit = %q, want empty string", path)
	}
}

func TestGetSpecificationPath_NumbersIncrement(t *testing.T) {
	dir := t.TempDir()
	c := newConductorWithStore(t, dir)

	wu := &WorkUnit{
		ID:    "task-spec-incr",
		Title: "Increment Test",
	}
	c.ForceWorkUnit(wu)

	// First call: should return specification-1.md
	path1 := c.getSpecificationPath()
	if !containsStr(path1, "specification-1.md") {
		t.Errorf("first getSpecificationPath() = %q, want specification-1.md", path1)
	}

	// Create the spec-1.md file to simulate it being written
	specDir := c.store.SpecificationsDir(wu.ID)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "specification-1.md"), []byte("content"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Second call: should return specification-2.md
	path2 := c.getSpecificationPath()
	if !containsStr(path2, "specification-2.md") {
		t.Errorf("second getSpecificationPath() = %q, want specification-2.md", path2)
	}
}

// containsStr is a small helper for substring checks.
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && stringContains(s, substr))
}

func stringContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}

	return false
}

// ─── AddReview / ListReviews happy path ──────────────────────────────────────

func TestAddReview_HappyPath(t *testing.T) {
	dir := t.TempDir()
	c := newConductorWithStore(t, dir)

	wu := &WorkUnit{
		ID:    "task-review-1",
		Title: "Review Test",
	}
	c.ForceWorkUnit(wu)

	// AddReview does not return an error or value; it logs on failure
	c.AddReview(true, "looks great")

	// Verify via ListReviews
	reviews, err := c.ListReviews()
	if err != nil {
		t.Fatalf("ListReviews() error = %v", err)
	}
	if len(reviews) == 0 {
		t.Fatal("AddReview() should have persisted a review; ListReviews() returned empty")
	}
}

func TestAddReview_RejectedStatus(t *testing.T) {
	dir := t.TempDir()
	c := newConductorWithStore(t, dir)

	wu := &WorkUnit{
		ID:    "task-review-reject",
		Title: "Reject Review Test",
	}
	c.ForceWorkUnit(wu)

	c.AddReview(false, "needs work on error handling")

	reviews, err := c.ListReviews()
	if err != nil {
		t.Fatalf("ListReviews() error = %v", err)
	}
	if len(reviews) == 0 {
		t.Fatal("AddReview() should have persisted a review")
	}
	if reviews[0].Status != "rejected" {
		t.Errorf("review Status = %q, want rejected", reviews[0].Status)
	}
}

// ─── ListReviews ─────────────────────────────────────────────────────────────

func TestListReviews_EmptyStore(t *testing.T) {
	dir := t.TempDir()
	c := newConductorWithStore(t, dir)
	c.ForceWorkUnit(&WorkUnit{ID: "task-lr-empty", Title: "T"})

	reviews, err := c.ListReviews()
	if err != nil {
		t.Fatalf("ListReviews() with empty store error = %v", err)
	}
	if len(reviews) != 0 {
		t.Errorf("ListReviews() on empty store = %v, want empty", reviews)
	}
}

func TestListReviews_AfterAddReview(t *testing.T) {
	dir := t.TempDir()
	c := newConductorWithStore(t, dir)

	wu := &WorkUnit{ID: "task-lr-1", Title: "List Reviews Test"}
	c.ForceWorkUnit(wu)

	c.AddReview(true, "first review")
	c.AddReview(false, "second review")

	reviews, err := c.ListReviews()
	if err != nil {
		t.Fatalf("ListReviews() error = %v", err)
	}
	if len(reviews) != 2 {
		t.Errorf("ListReviews() len = %d, want 2", len(reviews))
	}
}

// ─── detectSpecificationFiles ─────────────────────────────────────────────────

func TestDetectSpecificationFiles_FindsFiles(t *testing.T) {
	dir := t.TempDir()
	c := newConductorWithStore(t, dir)

	wu := &WorkUnit{
		ID:    "task-detect-spec",
		Title: "Detect Spec Files",
	}
	c.ForceWorkUnit(wu)

	// Create spec dir and a specification file
	specDir := c.store.SpecificationsDir(wu.ID)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	specFile := filepath.Join(specDir, "specification-1.md")
	if err := os.WriteFile(specFile, []byte("# Spec 1\ncontent"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// detectSpecificationFiles requires c.mu held
	c.mu.Lock()
	c.detectSpecificationFiles()
	c.mu.Unlock()

	if len(c.workUnit.Specifications) == 0 {
		t.Fatal("detectSpecificationFiles() should have detected specification-1.md")
	}
	if c.workUnit.Specifications[0] != specFile {
		t.Errorf("detectSpecificationFiles() Specifications[0] = %q, want %q",
			c.workUnit.Specifications[0], specFile)
	}
}

func TestDetectSpecificationFiles_NoDir(t *testing.T) {
	dir := t.TempDir()
	c := newConductorWithStore(t, dir)

	wu := &WorkUnit{ID: "task-detect-nodir", Title: "T"}
	c.ForceWorkUnit(wu)

	// detectSpecificationFiles with no spec dir should not panic
	c.mu.Lock()
	c.detectSpecificationFiles()
	c.mu.Unlock()

	if len(c.workUnit.Specifications) != 0 {
		t.Errorf("detectSpecificationFiles() with no dir should find nothing, got %v", c.workUnit.Specifications)
	}
}

func TestDetectSpecificationFiles_SkipsNonSpecFiles(t *testing.T) {
	dir := t.TempDir()
	c := newConductorWithStore(t, dir)

	wu := &WorkUnit{ID: "task-detect-skip", Title: "T"}
	c.ForceWorkUnit(wu)

	specDir := c.store.SpecificationsDir(wu.ID)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// Non-spec files should not be detected
	if err := os.WriteFile(filepath.Join(specDir, "plan.md"), []byte("plan"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "notes.txt"), []byte("notes"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// A valid spec file
	if err := os.WriteFile(filepath.Join(specDir, "specification-1.md"), []byte("spec"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	c.mu.Lock()
	c.detectSpecificationFiles()
	c.mu.Unlock()

	if len(c.workUnit.Specifications) != 1 {
		t.Errorf("detectSpecificationFiles() should find 1 spec file, got %d: %v",
			len(c.workUnit.Specifications), c.workUnit.Specifications)
	}
}

func TestDetectSpecificationFiles_DeduplicatesKnownSpecs(t *testing.T) {
	dir := t.TempDir()
	c := newConductorWithStore(t, dir)

	wu := &WorkUnit{ID: "task-detect-dedup", Title: "T"}
	c.ForceWorkUnit(wu)

	specDir := c.store.SpecificationsDir(wu.ID)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	specFile := filepath.Join(specDir, "specification-1.md")
	if err := os.WriteFile(specFile, []byte("spec"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Pre-populate with the already-known spec
	c.workUnit.Specifications = []string{specFile}

	c.mu.Lock()
	c.detectSpecificationFiles()
	c.mu.Unlock()

	// Should not duplicate
	if len(c.workUnit.Specifications) != 1 {
		t.Errorf("detectSpecificationFiles() should not duplicate known specs, got %d",
			len(c.workUnit.Specifications))
	}
}

// ─── TaskHistory with real store ──────────────────────────────────────────────

func TestTaskHistory_WithArchivedTasks(t *testing.T) {
	dir := t.TempDir()
	c := newConductorWithStore(t, dir)

	wu := &WorkUnit{
		ID:        "task-history-1",
		Title:     "History Task",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	c.ForceWorkUnit(wu)

	// Archive the task
	c.mu.Lock()
	c.archiveTask("submitted")
	c.mu.Unlock()

	tasks, err := c.TaskHistory()
	if err != nil {
		t.Fatalf("TaskHistory() error = %v", err)
	}
	if len(tasks) == 0 {
		t.Fatal("TaskHistory() should return archived tasks")
	}
	found := false
	for _, task := range tasks {
		if task.ID == "task-history-1" {
			found = true

			break
		}
	}
	if !found {
		t.Error("TaskHistory() should contain the archived task")
	}
}
