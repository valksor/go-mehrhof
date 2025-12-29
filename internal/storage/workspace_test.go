package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOpenWorkspace(t *testing.T) {
	tmpDir := t.TempDir()

	ws, err := OpenWorkspace(tmpDir)
	if err != nil {
		t.Fatalf("OpenWorkspace failed: %v", err)
	}

	if ws.Root() != tmpDir {
		t.Errorf("Root() = %q, want %q", ws.Root(), tmpDir)
	}

	expectedTaskRoot := filepath.Join(tmpDir, ".mehrhof")
	if ws.TaskRoot() != expectedTaskRoot {
		t.Errorf("TaskRoot() = %q, want %q", ws.TaskRoot(), expectedTaskRoot)
	}

	expectedWorkRoot := filepath.Join(tmpDir, ".mehrhof", "work")
	if ws.WorkRoot() != expectedWorkRoot {
		t.Errorf("WorkRoot() = %q, want %q", ws.WorkRoot(), expectedWorkRoot)
	}
}

func TestWorkspaceConfigPath(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)

	expected := filepath.Join(tmpDir, ".mehrhof", "config.yaml")
	if ws.ConfigPath() != expected {
		t.Errorf("ConfigPath() = %q, want %q", ws.ConfigPath(), expected)
	}
}

func TestHasConfig(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)

	// Initially no config
	if ws.HasConfig() {
		t.Error("HasConfig() = true, want false (no config exists)")
	}

	// Create config directory and file
	if err := os.MkdirAll(ws.TaskRoot(), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", ws.TaskRoot(), err)
	}
	if err := os.WriteFile(ws.ConfigPath(), []byte("test: true"), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", ws.ConfigPath(), err)
	}

	if !ws.HasConfig() {
		t.Error("HasConfig() = false, want true (config exists)")
	}
}

func TestNewDefaultWorkspaceConfig(t *testing.T) {
	cfg := NewDefaultWorkspaceConfig()

	if cfg.Git.AutoCommit != true {
		t.Errorf("Git.AutoCommit = %v, want true", cfg.Git.AutoCommit)
	}
	if cfg.Git.CommitPrefix != "[{key}]" {
		t.Errorf("Git.CommitPrefix = %q, want %q", cfg.Git.CommitPrefix, "[{key}]")
	}
	if cfg.Git.BranchPattern != "{type}/{key}--{slug}" {
		t.Errorf("Git.BranchPattern = %q, want %q", cfg.Git.BranchPattern, "{type}/{key}--{slug}")
	}
	if cfg.Agent.Default != "claude" {
		t.Errorf("Agent.Default = %q, want %q", cfg.Agent.Default, "claude")
	}
	if cfg.Agent.Timeout != 300 {
		t.Errorf("Agent.Timeout = %d, want 300", cfg.Agent.Timeout)
	}
	if cfg.Agent.MaxRetries != 3 {
		t.Errorf("Agent.MaxRetries = %d, want 3", cfg.Agent.MaxRetries)
	}
	if cfg.Workflow.AutoInit != true {
		t.Errorf("Workflow.AutoInit = %v, want true", cfg.Workflow.AutoInit)
	}
	if cfg.Workflow.SessionRetentionDays != 30 {
		t.Errorf("Workflow.SessionRetentionDays = %d, want 30", cfg.Workflow.SessionRetentionDays)
	}
	if cfg.Env == nil {
		t.Error("Env is nil, want initialized map")
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)

	cfg := NewDefaultWorkspaceConfig()
	cfg.Git.CommitPrefix = "[custom]"
	cfg.Agent.Timeout = 600

	if err := ws.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	loaded, err := ws.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if loaded.Git.CommitPrefix != "[custom]" {
		t.Errorf("loaded Git.CommitPrefix = %q, want %q", loaded.Git.CommitPrefix, "[custom]")
	}
	if loaded.Agent.Timeout != 600 {
		t.Errorf("loaded Agent.Timeout = %d, want 600", loaded.Agent.Timeout)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)

	// No config file exists, should return defaults
	cfg, err := ws.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Agent.Default != "claude" {
		t.Errorf("default Agent.Default = %q, want %q", cfg.Agent.Default, "claude")
	}
}

func TestEnsureInitialized(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)

	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized failed: %v", err)
	}

	// Check work directory exists
	if _, err := os.Stat(ws.WorkRoot()); os.IsNotExist(err) {
		t.Error("work directory was not created")
	}

	// Check .mehrhof directory exists
	if _, err := os.Stat(ws.TaskRoot()); os.IsNotExist(err) {
		t.Error(".mehrhof directory was not created")
	}

	// Note: EnsureInitialized does NOT update .gitignore
	// .gitignore is only updated by the init command via UpdateGitignore()
}

func TestUpdateGitignore(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)

	// Test with no existing .gitignore
	if err := ws.UpdateGitignore(); err != nil {
		t.Fatalf("UpdateGitignore failed: %v", err)
	}

	gitignorePath := filepath.Join(tmpDir, ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}

	content := string(data)
	if !contains(content, ".mehrhof/") {
		t.Error(".gitignore does not contain .mehrhof/")
	}
	if !contains(content, ".active_task") {
		t.Error(".gitignore does not contain .active_task")
	}
}

func TestUpdateGitignoreExisting(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)

	// Create existing .gitignore
	gitignorePath := filepath.Join(tmpDir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte("node_modules/\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", gitignorePath, err)
	}

	if err := ws.UpdateGitignore(); err != nil {
		t.Fatalf("UpdateGitignore failed: %v", err)
	}

	data, _ := os.ReadFile(gitignorePath)
	content := string(data)

	if !contains(content, "node_modules/") {
		t.Error("existing .gitignore content was lost")
	}
	if !contains(content, ".mehrhof/") {
		t.Error(".gitignore does not contain .mehrhof/")
	}
}

func TestUpdateGitignoreIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)

	// Call UpdateGitignore twice - second call should not duplicate entries
	if err := ws.UpdateGitignore(); err != nil {
		t.Fatalf("UpdateGitignore (1st) failed: %v", err)
	}

	if err := ws.UpdateGitignore(); err != nil {
		t.Fatalf("UpdateGitignore (2nd) failed: %v", err)
	}

	gitignorePath := filepath.Join(tmpDir, ".gitignore")
	data, _ := os.ReadFile(gitignorePath)
	lines := strings.Split(string(data), "\n")

	// Count exact line matches for ".mehrhof/work/" (not substring matches)
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == ".mehrhof/work/" {
			count++
		}
	}
	if count != 1 {
		t.Errorf(".mehrhof/work/ appears %d times as exact line in .gitignore, want 1", count)
	}

	// Also verify .active_task appears exactly once
	activeTaskCount := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == ".active_task" {
			activeTaskCount++
		}
	}
	if activeTaskCount != 1 {
		t.Errorf(".active_task appears %d times in .gitignore, want 1", activeTaskCount)
	}
}

func TestActiveTaskPath(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)

	expected := filepath.Join(tmpDir, ".active_task")
	if ws.ActiveTaskPath() != expected {
		t.Errorf("ActiveTaskPath() = %q, want %q", ws.ActiveTaskPath(), expected)
	}
}

func TestHasActiveTask(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)

	if ws.HasActiveTask() {
		t.Error("HasActiveTask() = true, want false (no active task)")
	}

	// Create active task file
	if err := os.WriteFile(ws.ActiveTaskPath(), []byte("id: test"), 0o644); err != nil {
		t.Fatalf("WriteFile active task: %v", err)
	}

	if !ws.HasActiveTask() {
		t.Error("HasActiveTask() = false, want true")
	}
}

func TestSaveAndLoadActiveTask(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)

	active := &ActiveTask{
		ID:      "test123",
		Ref:     "file:task.md",
		WorkDir: ".mehrhof/work/test123",
		State:   "planning",
		Branch:  "task/test123",
		UseGit:  true,
		Started: time.Now(),
	}

	if err := ws.SaveActiveTask(active); err != nil {
		t.Fatalf("SaveActiveTask failed: %v", err)
	}

	loaded, err := ws.LoadActiveTask()
	if err != nil {
		t.Fatalf("LoadActiveTask failed: %v", err)
	}

	if loaded.ID != active.ID {
		t.Errorf("loaded ID = %q, want %q", loaded.ID, active.ID)
	}
	if loaded.Ref != active.Ref {
		t.Errorf("loaded Ref = %q, want %q", loaded.Ref, active.Ref)
	}
	if loaded.State != active.State {
		t.Errorf("loaded State = %q, want %q", loaded.State, active.State)
	}
	if loaded.UseGit != active.UseGit {
		t.Errorf("loaded UseGit = %v, want %v", loaded.UseGit, active.UseGit)
	}
}

func TestClearActiveTask(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)

	// Create active task
	if err := os.WriteFile(ws.ActiveTaskPath(), []byte("id: test"), 0o644); err != nil {
		t.Fatalf("WriteFile active task: %v", err)
	}

	if err := ws.ClearActiveTask(); err != nil {
		t.Fatalf("ClearActiveTask failed: %v", err)
	}

	if ws.HasActiveTask() {
		t.Error("active task still exists after clear")
	}

	// Clear non-existent should not error
	if err := ws.ClearActiveTask(); err != nil {
		t.Errorf("ClearActiveTask on non-existent failed: %v", err)
	}
}

func TestUpdateActiveTaskState(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)

	active := &ActiveTask{
		ID:    "test123",
		State: "idle",
	}
	if err := ws.SaveActiveTask(active); err != nil {
		t.Fatalf("SaveActiveTask: %v", err)
	}

	if err := ws.UpdateActiveTaskState("planning"); err != nil {
		t.Fatalf("UpdateActiveTaskState failed: %v", err)
	}

	loaded, _ := ws.LoadActiveTask()
	if loaded.State != "planning" {
		t.Errorf("State = %q, want %q", loaded.State, "planning")
	}
}

func TestWorkPath(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)

	expected := filepath.Join(tmpDir, ".mehrhof", "work", "abc123")
	if ws.WorkPath("abc123") != expected {
		t.Errorf("WorkPath() = %q, want %q", ws.WorkPath("abc123"), expected)
	}
}

func TestWorkExists(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)

	if ws.WorkExists("nonexistent") {
		t.Error("WorkExists() = true for non-existent work")
	}

	// Create work directory
	if err := os.MkdirAll(ws.WorkPath("test123"), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", ws.WorkPath("test123"), err)
	}

	if !ws.WorkExists("test123") {
		t.Error("WorkExists() = false for existing work")
	}
}

func TestGenerateTaskID(t *testing.T) {
	id1 := GenerateTaskID()
	id2 := GenerateTaskID()

	if id1 == "" {
		t.Error("GenerateTaskID returned empty string")
	}
	if len(id1) != 8 {
		t.Errorf("GenerateTaskID length = %d, want 8", len(id1))
	}
	if id1 == id2 {
		t.Error("GenerateTaskID returned duplicate IDs")
	}
}

func TestCreateWork(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	source := SourceInfo{
		Type:    "file",
		Ref:     "task.md",
		Content: "Test task content",
		ReadAt:  time.Now(),
	}

	work, err := ws.CreateWork("test123", source)
	if err != nil {
		t.Fatalf("CreateWork failed: %v", err)
	}

	if work.Metadata.ID != "test123" {
		t.Errorf("work ID = %q, want %q", work.Metadata.ID, "test123")
	}

	// Check directories were created
	if !ws.WorkExists("test123") {
		t.Error("work directory was not created")
	}

	specsDir := ws.SpecificationsDir("test123")
	if _, err := os.Stat(specsDir); os.IsNotExist(err) {
		t.Error("specifications directory was not created")
	}

	sessionsDir := ws.SessionsDir("test123")
	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		t.Error("sessions directory was not created")
	}

	// Check notes.md was created
	notesPath := ws.NotesPath("test123")
	if _, err := os.Stat(notesPath); os.IsNotExist(err) {
		t.Error("notes.md was not created")
	}
}

func TestLoadAndSaveWork(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	source := SourceInfo{
		Type:    "file",
		Ref:     "task.md",
		Content: "Test content",
	}

	work, _ := ws.CreateWork("test123", source)
	work.Metadata.Title = "Test Task"

	if err := ws.SaveWork(work); err != nil {
		t.Fatalf("SaveWork failed: %v", err)
	}

	loaded, err := ws.LoadWork("test123")
	if err != nil {
		t.Fatalf("LoadWork failed: %v", err)
	}

	if loaded.Metadata.Title != "Test Task" {
		t.Errorf("loaded Title = %q, want %q", loaded.Metadata.Title, "Test Task")
	}
	if loaded.Source.Content != "Test content" {
		t.Errorf("loaded Source.Content = %q, want %q", loaded.Source.Content, "Test content")
	}
}

func TestDeleteWork(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	source := SourceInfo{Type: "file", Ref: "task.md"}
	if _, err := ws.CreateWork("test123", source); err != nil {
		t.Fatalf("CreateWork(test123): %v", err)
	}

	if err := ws.DeleteWork("test123"); err != nil {
		t.Fatalf("DeleteWork failed: %v", err)
	}

	if ws.WorkExists("test123") {
		t.Error("work still exists after delete")
	}
}

func TestListWorks(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	// Initially empty
	works, err := ws.ListWorks()
	if err != nil {
		t.Fatalf("ListWorks failed: %v", err)
	}
	if len(works) != 0 {
		t.Errorf("ListWorks returned %d works, want 0", len(works))
	}

	// Create some works
	source := SourceInfo{Type: "file", Ref: "task.md"}
	if _, err := ws.CreateWork("task1", source); err != nil {
		t.Fatalf("CreateWork(task1): %v", err)
	}
	if _, err := ws.CreateWork("task2", source); err != nil {
		t.Fatalf("CreateWork(task2): %v", err)
	}

	works, err = ws.ListWorks()
	if err != nil {
		t.Fatalf("ListWorks failed: %v", err)
	}
	if len(works) != 2 {
		t.Errorf("ListWorks returned %d works, want 2", len(works))
	}
}

func TestNotesPath(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)

	expected := filepath.Join(tmpDir, ".mehrhof", "work", "test123", "notes.md")
	if ws.NotesPath("test123") != expected {
		t.Errorf("NotesPath() = %q, want %q", ws.NotesPath("test123"), expected)
	}
}

func TestAppendAndReadNotes(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	source := SourceInfo{Type: "file", Ref: "task.md"}
	if _, err := ws.CreateWork("test123", source); err != nil {
		t.Fatalf("CreateWork(test123): %v", err)
	}

	if err := ws.AppendNote("test123", "First note", "planning"); err != nil {
		t.Fatalf("AppendNote failed: %v", err)
	}

	if err := ws.AppendNote("test123", "Second note", "implementing"); err != nil {
		t.Fatalf("AppendNote failed: %v", err)
	}

	content, err := ws.ReadNotes("test123")
	if err != nil {
		t.Fatalf("ReadNotes failed: %v", err)
	}

	if !contains(content, "First note") {
		t.Error("notes do not contain 'First note'")
	}
	if !contains(content, "Second note") {
		t.Error("notes do not contain 'Second note'")
	}
	if !contains(content, "[planning]") {
		t.Error("notes do not contain state tag '[planning]'")
	}
}

func TestSpecsDir(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)

	expected := filepath.Join(tmpDir, ".mehrhof", "work", "test123", "specifications")
	if ws.SpecificationsDir("test123") != expected {
		t.Errorf("SpecsDir() = %q, want %q", ws.SpecificationsDir("test123"), expected)
	}
}

func TestSpecPath(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)

	expected := filepath.Join(tmpDir, ".mehrhof", "work", "test123", "specifications", "specification-1.md")
	if ws.SpecificationPath("test123", 1) != expected {
		t.Errorf("SpecPath() = %q, want %q", ws.SpecificationPath("test123", 1), expected)
	}
}

func TestSaveAndLoadSpec(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	source := SourceInfo{Type: "file", Ref: "task.md"}
	if _, err := ws.CreateWork("test123", source); err != nil {
		t.Fatalf("CreateWork(test123): %v", err)
	}

	content := "# Spec 1\n\nThis is the first spec."
	if err := ws.SaveSpecification("test123", 1, content); err != nil {
		t.Fatalf("SaveSpec failed: %v", err)
	}

	loaded, err := ws.LoadSpecification("test123", 1)
	if err != nil {
		t.Fatalf("LoadSpec failed: %v", err)
	}

	if loaded != content {
		t.Errorf("loaded spec = %q, want %q", loaded, content)
	}
}

func TestListSpecs(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	source := SourceInfo{Type: "file", Ref: "task.md"}
	if _, err := ws.CreateWork("test123", source); err != nil {
		t.Fatalf("CreateWork(test123): %v", err)
	}
	// Initially empty
	specifications, err := ws.ListSpecifications("test123")
	if err != nil {
		t.Fatalf("ListSpecifications failed: %v", err)
	}
	if len(specifications) != 0 {
		t.Errorf("ListSpecifications returned %d specifications, want 0", len(specifications))
	}

	// Add some specifications
	if err := ws.SaveSpecification("test123", 1, "Specification 1"); err != nil {
		t.Fatalf("SaveSpecification(1): %v", err)
	}
	if err := ws.SaveSpecification("test123", 3, "Specification 3"); err != nil {
		t.Fatalf("SaveSpecification(3): %v", err)
	}
	if err := ws.SaveSpecification("test123", 2, "Specification 2"); err != nil {
		t.Fatalf("SaveSpecification(2): %v", err)
	}

	specifications, err = ws.ListSpecifications("test123")
	if err != nil {
		t.Fatalf("ListSpecifications failed: %v", err)
	}
	if len(specifications) != 3 {
		t.Errorf("ListSpecifications returned %d specifications, want 3", len(specifications))
	}

	// Should be sorted
	if specifications[0] != 1 || specifications[1] != 2 || specifications[2] != 3 {
		t.Errorf("specifications not sorted: %v", specifications)
	}
}

func TestNextSpecNumber(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	source := SourceInfo{Type: "file", Ref: "task.md"}
	if _, err := ws.CreateWork("test123", source); err != nil {
		t.Fatalf("CreateWork(test123): %v", err)
	}

	// First spec should be 1
	num, err := ws.NextSpecificationNumber("test123")
	if err != nil {
		t.Fatalf("NextSpecNumber failed: %v", err)
	}
	if num != 1 {
		t.Errorf("NextSpecNumber = %d, want 1", num)
	}

	// After adding spec 1, next should be 2
	if err := ws.SaveSpecification("test123", 1, "Spec 1"); err != nil {
		t.Fatalf("SaveSpec(1): %v", err)
	}
	num, _ = ws.NextSpecificationNumber("test123")
	if num != 2 {
		t.Errorf("NextSpecNumber = %d, want 2", num)
	}

	// After adding spec 5, next should be 6
	if err := ws.SaveSpecification("test123", 5, "Spec 5"); err != nil {
		t.Fatalf("SaveSpec(5): %v", err)
	}
	num, _ = ws.NextSpecificationNumber("test123")
	if num != 6 {
		t.Errorf("NextSpecNumber = %d, want 6", num)
	}
}

func TestGatherSpecsContent(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	source := SourceInfo{Type: "file", Ref: "task.md"}
	if _, err := ws.CreateWork("test123", source); err != nil {
		t.Fatalf("CreateWork(test123): %v", err)
	}

	if err := ws.SaveSpecification("test123", 1, "Content of spec 1"); err != nil {
		t.Fatalf("SaveSpec(1): %v", err)
	}
	if err := ws.SaveSpecification("test123", 2, "Content of spec 2"); err != nil {
		t.Fatalf("SaveSpec(2): %v", err)
	}

	content, err := ws.GatherSpecificationsContent("test123")
	if err != nil {
		t.Fatalf("GatherSpecsContent failed: %v", err)
	}

	if !contains(content, "Specification 1") {
		t.Error("gathered content does not contain Specification 1")
	}
	if !contains(content, "Content of spec 1") {
		t.Error("gathered content does not contain spec 1 content")
	}
	if !contains(content, "Specification 2") {
		t.Error("gathered content does not contain Specification 2")
	}
}

func TestGetLatestSpecContent(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	source := SourceInfo{Type: "file", Ref: "task.md"}
	if _, err := ws.CreateWork("test123", source); err != nil {
		t.Fatalf("CreateWork(test123): %v", err)
	}

	// No specifications
	content, num, err := ws.GetLatestSpecificationContent("test123")
	if err != nil {
		t.Fatalf("GetLatestSpecificationContent failed: %v", err)
	}
	if content != "" || num != 0 {
		t.Errorf("expected empty content and 0, got %q and %d", content, num)
	}

	// Add specifications
	if err := ws.SaveSpecification("test123", 1, "First specification"); err != nil {
		t.Fatalf("SaveSpecification(1): %v", err)
	}
	if err := ws.SaveSpecification("test123", 3, "Third specification"); err != nil {
		t.Fatalf("SaveSpecification(3): %v", err)
	}

	content, num, err = ws.GetLatestSpecificationContent("test123")
	if err != nil {
		t.Fatalf("GetLatestSpecificationContent failed: %v", err)
	}
	if num != 3 {
		t.Errorf("latest specification number = %d, want 3", num)
	}
	if content != "Third specification" {
		t.Errorf("latest specification content = %q, want %q", content, "Third specification")
	}
}

func TestSessionsDir(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)

	expected := filepath.Join(tmpDir, ".mehrhof", "work", "test123", "sessions")
	if ws.SessionsDir("test123") != expected {
		t.Errorf("SessionsDir() = %q, want %q", ws.SessionsDir("test123"), expected)
	}
}

func TestSessionPath(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)

	expected := filepath.Join(tmpDir, ".mehrhof", "work", "test123", "sessions", "session.yaml")
	if ws.SessionPath("test123", "session.yaml") != expected {
		t.Errorf("SessionPath() = %q, want %q", ws.SessionPath("test123", "session.yaml"), expected)
	}
}

func TestCreateSession(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	source := SourceInfo{Type: "file", Ref: "task.md"}
	if _, err := ws.CreateWork("test123", source); err != nil {
		t.Fatalf("CreateWork(test123): %v", err)
	}

	session, filename, err := ws.CreateSession("test123", "planning", "claude", "planning")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if session.Metadata.Type != "planning" {
		t.Errorf("session type = %q, want %q", session.Metadata.Type, "planning")
	}
	if session.Metadata.Agent != "claude" {
		t.Errorf("session agent = %q, want %q", session.Metadata.Agent, "claude")
	}
	if filename == "" {
		t.Error("filename is empty")
	}
	if !contains(filename, "planning") {
		t.Error("filename does not contain session type")
	}
}

func TestLoadAndSaveSession(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	source := SourceInfo{Type: "file", Ref: "task.md"}
	if _, err := ws.CreateWork("test123", source); err != nil {
		t.Fatalf("CreateWork(test123): %v", err)
	}

	session, filename, _ := ws.CreateSession("test123", "planning", "claude", "planning")

	// Add an exchange
	session.Exchanges = append(session.Exchanges, Exchange{
		Role:      "user",
		Timestamp: time.Now(),
		Content:   "Test message",
	})

	if err := ws.SaveSession("test123", filename, session); err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	loaded, err := ws.LoadSession("test123", filename)
	if err != nil {
		t.Fatalf("LoadSession failed: %v", err)
	}

	if len(loaded.Exchanges) != 1 {
		t.Errorf("loaded exchanges count = %d, want 1", len(loaded.Exchanges))
	}
	if loaded.Exchanges[0].Content != "Test message" {
		t.Errorf("loaded exchange content = %q, want %q", loaded.Exchanges[0].Content, "Test message")
	}
}

func TestGetSourceContent(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	// Test single file source
	source := SourceInfo{
		Type:    "file",
		Ref:     "task.md",
		Content: "Single file content",
	}
	if _, err := ws.CreateWork("test1", source); err != nil {
		t.Fatalf("CreateWork(test1): %v", err)
	}

	content, err := ws.GetSourceContent("test1")
	if err != nil {
		t.Fatalf("GetSourceContent failed: %v", err)
	}
	if content != "Single file content" {
		t.Errorf("content = %q, want %q", content, "Single file content")
	}

	// Test directory source with multiple files
	source2 := SourceInfo{
		Type: "directory",
		Ref:  "tasks/",
		Files: []SourceFile{
			{Path: "file1.md", Content: "Content 1"},
			{Path: "file2.md", Content: "Content 2"},
		},
	}
	if _, err := ws.CreateWork("test2", source2); err != nil {
		t.Fatalf("CreateWork(test2): %v", err)
	}

	content, err = ws.GetSourceContent("test2")
	if err != nil {
		t.Fatalf("GetSourceContent failed: %v", err)
	}
	if !contains(content, "file1.md") {
		t.Error("content does not contain file1.md")
	}
	if !contains(content, "Content 1") {
		t.Error("content does not contain Content 1")
	}
	if !contains(content, "file2.md") {
		t.Error("content does not contain file2.md")
	}
}

func TestPendingQuestionPath(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)

	expected := filepath.Join(tmpDir, ".mehrhof", "work", "test123", "pending_question.yaml")
	if ws.PendingQuestionPath("test123") != expected {
		t.Errorf("PendingQuestionPath() = %q, want %q", ws.PendingQuestionPath("test123"), expected)
	}
}

func TestHasPendingQuestion(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	source := SourceInfo{Type: "file", Ref: "task.md"}
	if _, err := ws.CreateWork("test123", source); err != nil {
		t.Fatalf("CreateWork(test123): %v", err)
	}

	if ws.HasPendingQuestion("test123") {
		t.Error("HasPendingQuestion() = true, want false")
	}

	// Create pending question
	q := &PendingQuestion{
		Question: "Test question?",
		Phase:    "planning",
		AskedAt:  time.Now(),
	}
	if err := ws.SavePendingQuestion("test123", q); err != nil {
		t.Fatalf("SavePendingQuestion: %v", err)
	}

	if !ws.HasPendingQuestion("test123") {
		t.Error("HasPendingQuestion() = false, want true")
	}
}

func TestSaveAndLoadPendingQuestion(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	source := SourceInfo{Type: "file", Ref: "task.md"}
	if _, err := ws.CreateWork("test123", source); err != nil {
		t.Fatalf("CreateWork(test123): %v", err)
	}

	q := &PendingQuestion{
		Question: "What should we do?",
		Options: []QuestionOption{
			{Label: "A", Description: "Option A"},
			{Label: "B", Description: "Option B"},
		},
		Phase:   "planning",
		AskedAt: time.Now(),
	}

	if err := ws.SavePendingQuestion("test123", q); err != nil {
		t.Fatalf("SavePendingQuestion failed: %v", err)
	}

	loaded, err := ws.LoadPendingQuestion("test123")
	if err != nil {
		t.Fatalf("LoadPendingQuestion failed: %v", err)
	}

	if loaded.Question != q.Question {
		t.Errorf("loaded question = %q, want %q", loaded.Question, q.Question)
	}
	if len(loaded.Options) != 2 {
		t.Errorf("loaded options count = %d, want 2", len(loaded.Options))
	}
	if loaded.Phase != "planning" {
		t.Errorf("loaded phase = %q, want %q", loaded.Phase, "planning")
	}
}

func TestClearPendingQuestion(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	source := SourceInfo{Type: "file", Ref: "task.md"}
	if _, err := ws.CreateWork("test123", source); err != nil {
		t.Fatalf("CreateWork(test123): %v", err)
	}

	q := &PendingQuestion{Question: "Test?", Phase: "planning", AskedAt: time.Now()}
	if err := ws.SavePendingQuestion("test123", q); err != nil {
		t.Fatalf("SavePendingQuestion: %v", err)
	}

	if err := ws.ClearPendingQuestion("test123"); err != nil {
		t.Fatalf("ClearPendingQuestion failed: %v", err)
	}

	if ws.HasPendingQuestion("test123") {
		t.Error("pending question still exists after clear")
	}

	// Clear non-existent should not error
	if err := ws.ClearPendingQuestion("test123"); err != nil {
		t.Errorf("ClearPendingQuestion on non-existent failed: %v", err)
	}
}

// Type constructor tests

func TestNewActiveTask(t *testing.T) {
	task := NewActiveTask("task123", "file:test.md", ".mehrhof/work/task123")

	if task.ID != "task123" {
		t.Errorf("ID = %q, want %q", task.ID, "task123")
	}
	if task.Ref != "file:test.md" {
		t.Errorf("Ref = %q, want %q", task.Ref, "file:test.md")
	}
	if task.WorkDir != ".mehrhof/work/task123" {
		t.Errorf("WorkDir = %q, want %q", task.WorkDir, ".mehrhof/work/task123")
	}
	if task.State != "idle" {
		t.Errorf("State = %q, want %q", task.State, "idle")
	}
	if task.UseGit != false {
		t.Errorf("UseGit = %v, want false", task.UseGit)
	}
	if task.Started.IsZero() {
		t.Error("Started should not be zero")
	}
}

func TestNewTaskWork(t *testing.T) {
	source := SourceInfo{
		Type:    "file",
		Ref:     "task.md",
		Content: "Test content",
	}
	work := NewTaskWork("work123", source)

	if work.Version != "1" {
		t.Errorf("Version = %q, want %q", work.Version, "1")
	}
	if work.Metadata.ID != "work123" {
		t.Errorf("Metadata.ID = %q, want %q", work.Metadata.ID, "work123")
	}
	if work.Source.Type != "file" {
		t.Errorf("Source.Type = %q, want %q", work.Source.Type, "file")
	}
	if work.Metadata.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestNewSession(t *testing.T) {
	session := NewSession("planning", "claude", "planning")

	if session.Version != "1" {
		t.Errorf("Version = %q, want %q", session.Version, "1")
	}
	if session.Kind != "Session" {
		t.Errorf("Kind = %q, want %q", session.Kind, "Session")
	}
	if session.Metadata.Type != "planning" {
		t.Errorf("Metadata.Type = %q, want %q", session.Metadata.Type, "planning")
	}
	if session.Metadata.Agent != "claude" {
		t.Errorf("Metadata.Agent = %q, want %q", session.Metadata.Agent, "claude")
	}
	if session.Metadata.State != "planning" {
		t.Errorf("Metadata.State = %q, want %q", session.Metadata.State, "planning")
	}
	if session.Exchanges == nil {
		t.Error("Exchanges should not be nil")
	}
}

// Spec parsing tests

func TestParseSpec(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	source := SourceInfo{Type: "file", Ref: "task.md"}
	if _, err := ws.CreateWork("test123", source); err != nil {
		t.Fatalf("CreateWork: %v", err)
	}

	// Test spec without frontmatter
	content := "# Spec Title\n\nThis is the spec content."
	if err := ws.SaveSpecification("test123", 1, content); err != nil {
		t.Fatalf("SaveSpec: %v", err)
	}

	spec, err := ws.ParseSpecification("test123", 1)
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	if spec.Number != 1 {
		t.Errorf("Number = %d, want 1", spec.Number)
	}
	if spec.Title != "Spec Title" {
		t.Errorf("Title = %q, want %q", spec.Title, "Spec Title")
	}
	if spec.Status != SpecificationStatusDraft {
		t.Errorf("Status = %q, want %q", spec.Status, SpecificationStatusDraft)
	}
}

func TestParseSpecWithFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	source := SourceInfo{Type: "file", Ref: "task.md"}
	if _, err := ws.CreateWork("test123", source); err != nil {
		t.Fatalf("CreateWork: %v", err)
	}

	// Test spec with frontmatter
	content := `---
title: "Frontmatter Title"
status: ready
---

# Content Title

This is the spec content.`
	if err := ws.SaveSpecification("test123", 2, content); err != nil {
		t.Fatalf("SaveSpec: %v", err)
	}

	spec, err := ws.ParseSpecification("test123", 2)
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	// Note: ParseSpec extracts title from markdown heading (# Content Title),
	// which overwrites any frontmatter title. This is intentional - frontmatter
	// stores metadata like status, while visible title comes from the document.
	if spec.Title != "Content Title" {
		t.Errorf("Title = %q, want %q (from heading, not frontmatter)", spec.Title, "Content Title")
	}
	if spec.Status != "ready" {
		t.Errorf("Status = %q, want %q", spec.Status, "ready")
	}
}

func TestSaveSpecWithMeta(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	source := SourceInfo{Type: "file", Ref: "task.md"}
	if _, err := ws.CreateWork("test123", source); err != nil {
		t.Fatalf("CreateWork: %v", err)
	}

	spec := &Specification{
		Number:  3,
		Title:   "Test Spec",
		Status:  SpecificationStatusReady,
		Content: "# Test Spec\n\nContent here.",
	}

	if err := ws.SaveSpecificationWithMeta("test123", spec); err != nil {
		t.Fatalf("SaveSpecWithMeta: %v", err)
	}

	// Load and verify
	loaded, err := ws.ParseSpecification("test123", 3)
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}

	if loaded.Title != "Test Spec" {
		t.Errorf("loaded Title = %q, want %q", loaded.Title, "Test Spec")
	}
	if loaded.Status != SpecificationStatusReady {
		t.Errorf("loaded Status = %q, want %q", loaded.Status, SpecificationStatusReady)
	}
}

func TestUpdateSpecStatus(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	source := SourceInfo{Type: "file", Ref: "task.md"}
	if _, err := ws.CreateWork("test123", source); err != nil {
		t.Fatalf("CreateWork: %v", err)
	}

	// Create spec
	spec := &Specification{
		Number:  1,
		Title:   "Updateable Spec",
		Status:  SpecificationStatusDraft,
		Content: "# Updateable Spec\n\nContent.",
	}
	if err := ws.SaveSpecificationWithMeta("test123", spec); err != nil {
		t.Fatalf("SaveSpecWithMeta: %v", err)
	}

	// Update to implementing
	if err := ws.UpdateSpecificationStatus("test123", 1, SpecificationStatusImplementing); err != nil {
		t.Fatalf("UpdateSpecStatus: %v", err)
	}

	loaded, _ := ws.ParseSpecification("test123", 1)
	if loaded.Status != SpecificationStatusImplementing {
		t.Errorf("Status = %q, want %q", loaded.Status, SpecificationStatusImplementing)
	}

	// Update to done (should set CompletedAt)
	if err := ws.UpdateSpecificationStatus("test123", 1, SpecificationStatusDone); err != nil {
		t.Fatalf("UpdateSpecStatus to done: %v", err)
	}

	loaded, _ = ws.ParseSpecification("test123", 1)
	if loaded.Status != SpecificationStatusDone {
		t.Errorf("Status = %q, want %q", loaded.Status, SpecificationStatusDone)
	}
}

func TestListSpecsWithStatus(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	source := SourceInfo{Type: "file", Ref: "task.md"}
	if _, err := ws.CreateWork("test123", source); err != nil {
		t.Fatalf("CreateWork: %v", err)
	}

	// Create specifications with different statuses
	specifications := []*Specification{
		{Number: 1, Status: SpecificationStatusDraft, Content: "# Specification 1"},
		{Number: 2, Status: SpecificationStatusReady, Content: "# Specification 2"},
		{Number: 3, Status: SpecificationStatusDone, Content: "# Specification 3"},
	}

	for _, s := range specifications {
		if err := ws.SaveSpecificationWithMeta("test123", s); err != nil {
			t.Fatalf("SaveSpecificationWithMeta(%d): %v", s.Number, err)
		}
	}

	result, err := ws.ListSpecificationsWithStatus("test123")
	if err != nil {
		t.Fatalf("ListSpecsWithStatus: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("result length = %d, want 3", len(result))
	}

	// Check statuses are loaded
	statusMap := make(map[int]string)
	for _, s := range result {
		statusMap[s.Number] = s.Status
	}

	if statusMap[1] != SpecificationStatusDraft {
		t.Errorf("spec 1 status = %q, want %q", statusMap[1], SpecificationStatusDraft)
	}
	if statusMap[2] != SpecificationStatusReady {
		t.Errorf("spec 2 status = %q, want %q", statusMap[2], SpecificationStatusReady)
	}
	if statusMap[3] != SpecificationStatusDone {
		t.Errorf("spec 3 status = %q, want %q", statusMap[3], SpecificationStatusDone)
	}
}

func TestGetSpecsSummary(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	source := SourceInfo{Type: "file", Ref: "task.md"}
	if _, err := ws.CreateWork("test123", source); err != nil {
		t.Fatalf("CreateWork: %v", err)
	}

	// Create specifications with various statuses
	specifications := []*Specification{
		{Number: 1, Status: SpecificationStatusDraft, Content: "# S1"},
		{Number: 2, Status: SpecificationStatusDraft, Content: "# S2"},
		{Number: 3, Status: SpecificationStatusReady, Content: "# S3"},
		{Number: 4, Status: SpecificationStatusDone, Content: "# S4"},
	}

	for _, s := range specifications {
		if err := ws.SaveSpecificationWithMeta("test123", s); err != nil {
			t.Fatalf("SaveSpecificationWithMeta(%d): %v", s.Number, err)
		}
	}

	summary, err := ws.GetSpecificationsSummary("test123")
	if err != nil {
		t.Fatalf("GetSpecsSummary: %v", err)
	}

	if summary[SpecificationStatusDraft] != 2 {
		t.Errorf("draft count = %d, want 2", summary[SpecificationStatusDraft])
	}
	if summary[SpecificationStatusReady] != 1 {
		t.Errorf("ready count = %d, want 1", summary[SpecificationStatusReady])
	}
	if summary[SpecificationStatusDone] != 1 {
		t.Errorf("done count = %d, want 1", summary[SpecificationStatusDone])
	}
}

// Plan tests

func TestPlannedRoot(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)

	expected := filepath.Join(tmpDir, ".mehrhof", "planned")
	if ws.PlannedRoot() != expected {
		t.Errorf("PlannedRoot() = %q, want %q", ws.PlannedRoot(), expected)
	}
}

func TestPlannedPath(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)

	expected := filepath.Join(tmpDir, ".mehrhof", "planned", "plan123")
	if ws.PlannedPath("plan123") != expected {
		t.Errorf("PlannedPath() = %q, want %q", ws.PlannedPath("plan123"), expected)
	}
}

func TestGeneratePlanID(t *testing.T) {
	id1 := GeneratePlanID()
	if id1 == "" {
		t.Error("GeneratePlanID returned empty string")
	}

	// Should be timestamp format YYYY-MM-DD-HHMMSS
	if len(id1) != 17 { // 2006-01-02-150405
		t.Errorf("GeneratePlanID length = %d, want 17", len(id1))
	}
}

func TestCreatePlan(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	plan, err := ws.CreatePlan("test-plan", "Initial seed idea")
	if err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}

	if plan.ID != "test-plan" {
		t.Errorf("ID = %q, want %q", plan.ID, "test-plan")
	}
	if plan.Seed != "Initial seed idea" {
		t.Errorf("Seed = %q, want %q", plan.Seed, "Initial seed idea")
	}
	if plan.Version != "1" {
		t.Errorf("Version = %q, want %q", plan.Version, "1")
	}

	// Check plan directory was created
	planPath := ws.PlannedPath("test-plan")
	if _, err := os.Stat(planPath); os.IsNotExist(err) {
		t.Error("plan directory was not created")
	}

	// Check plan-history.md was created
	historyPath := filepath.Join(planPath, "plan-history.md")
	if _, err := os.Stat(historyPath); os.IsNotExist(err) {
		t.Error("plan-history.md was not created")
	}

	// Verify history contains seed
	data, _ := os.ReadFile(historyPath)
	if !contains(string(data), "Initial seed idea") {
		t.Error("history file does not contain seed")
	}
}

func TestSaveAndLoadPlan(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	plan, err := ws.CreatePlan("save-test", "")
	if err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}

	plan.Title = "Updated Title"
	if err := ws.SavePlan(plan); err != nil {
		t.Fatalf("SavePlan: %v", err)
	}

	loaded, err := ws.LoadPlan("save-test")
	if err != nil {
		t.Fatalf("LoadPlan: %v", err)
	}

	if loaded.Title != "Updated Title" {
		t.Errorf("loaded Title = %q, want %q", loaded.Title, "Updated Title")
	}
}

func TestAppendPlanHistory(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	if _, err := ws.CreatePlan("history-test", ""); err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}

	// Add user message
	if err := ws.AppendPlanHistory("history-test", "user", "User message"); err != nil {
		t.Fatalf("AppendPlanHistory (user): %v", err)
	}

	// Add assistant message
	if err := ws.AppendPlanHistory("history-test", "assistant", "Assistant response"); err != nil {
		t.Fatalf("AppendPlanHistory (assistant): %v", err)
	}

	// Load and verify
	plan, _ := ws.LoadPlan("history-test")
	if len(plan.History) != 2 {
		t.Fatalf("History length = %d, want 2", len(plan.History))
	}

	if plan.History[0].Role != "user" {
		t.Errorf("History[0].Role = %q, want %q", plan.History[0].Role, "user")
	}
	if plan.History[0].Content != "User message" {
		t.Errorf("History[0].Content = %q, want %q", plan.History[0].Content, "User message")
	}
	if plan.History[1].Role != "assistant" {
		t.Errorf("History[1].Role = %q, want %q", plan.History[1].Role, "assistant")
	}

	// Check markdown file was updated
	historyPath := filepath.Join(ws.PlannedPath("history-test"), "plan-history.md")
	data, _ := os.ReadFile(historyPath)
	content := string(data)

	if !contains(content, "User message") {
		t.Error("history markdown does not contain user message")
	}
	if !contains(content, "Assistant response") {
		t.Error("history markdown does not contain assistant response")
	}
}

func TestListPlans(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	// Initially empty
	plans, err := ws.ListPlans()
	if err != nil {
		t.Fatalf("ListPlans: %v", err)
	}
	if len(plans) != 0 {
		t.Errorf("ListPlans returned %d plans, want 0", len(plans))
	}

	// Create some plans
	if _, err := ws.CreatePlan("plan-a", ""); err != nil {
		t.Fatalf("CreatePlan(a): %v", err)
	}
	if _, err := ws.CreatePlan("plan-b", ""); err != nil {
		t.Fatalf("CreatePlan(b): %v", err)
	}

	plans, err = ws.ListPlans()
	if err != nil {
		t.Fatalf("ListPlans: %v", err)
	}
	if len(plans) != 2 {
		t.Errorf("ListPlans returned %d plans, want 2", len(plans))
	}
}

func TestDeletePlan(t *testing.T) {
	tmpDir := t.TempDir()
	ws, _ := OpenWorkspace(tmpDir)
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	if _, err := ws.CreatePlan("delete-me", ""); err != nil {
		t.Fatalf("CreatePlan: %v", err)
	}

	if err := ws.DeletePlan("delete-me"); err != nil {
		t.Fatalf("DeletePlan: %v", err)
	}

	// Verify deleted
	if _, err := os.Stat(ws.PlannedPath("delete-me")); !os.IsNotExist(err) {
		t.Error("plan directory still exists after delete")
	}

	plans, _ := ws.ListPlans()
	if len(plans) != 0 {
		t.Error("plan still appears in list after delete")
	}
}

func TestSpecStatusConstants(t *testing.T) {
	if SpecificationStatusDraft != "draft" {
		t.Errorf("SpecificationStatusDraft = %q, want %q", SpecificationStatusDraft, "draft")
	}
	if SpecificationStatusReady != "ready" {
		t.Errorf("SpecificationStatusReady = %q, want %q", SpecificationStatusReady, "ready")
	}
	if SpecificationStatusImplementing != "implementing" {
		t.Errorf("SpecificationStatusImplementing = %q, want %q", SpecificationStatusImplementing, "implementing")
	}
	if SpecificationStatusDone != "done" {
		t.Errorf("SpecificationStatusDone = %q, want %q", SpecificationStatusDone, "done")
	}
}

func TestGetEnvForAgent(t *testing.T) {
	tests := []struct {
		env       map[string]string
		want      map[string]string
		name      string
		agentName string
	}{
		{
			name: "filters claude vars and strips prefix",
			env: map[string]string{
				"CLAUDE_ANTHROPIC_API_KEY": "sk-ant-xxx",
				"CLAUDE_MAX_TOKENS":        "4096",
				"OPENAI_API_KEY":           "sk-openai-xxx",
				"UNRELATED_VAR":            "value",
			},
			agentName: "claude",
			want: map[string]string{
				"ANTHROPIC_API_KEY": "sk-ant-xxx",
				"MAX_TOKENS":        "4096",
			},
		},
		{
			name: "filters openai vars",
			env: map[string]string{
				"CLAUDE_ANTHROPIC_API_KEY": "sk-ant-xxx",
				"OPENAI_API_KEY":           "sk-openai-xxx",
				"OPENAI_MODEL":             "gpt-4",
			},
			agentName: "openai",
			want: map[string]string{
				"API_KEY": "sk-openai-xxx",
				"MODEL":   "gpt-4",
			},
		},
		{
			name: "case insensitive agent name",
			env: map[string]string{
				"CLAUDE_FOO": "bar",
			},
			agentName: "Claude",
			want: map[string]string{
				"FOO": "bar",
			},
		},
		{
			name:      "empty env returns empty map",
			env:       map[string]string{},
			agentName: "claude",
			want:      map[string]string{},
		},
		{
			name: "no matching prefix returns empty map",
			env: map[string]string{
				"OPENAI_API_KEY": "sk-xxx",
			},
			agentName: "claude",
			want:      map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &WorkspaceConfig{Env: tt.env}
			got := cfg.GetEnvForAgent(tt.agentName)

			if len(got) != len(tt.want) {
				t.Errorf("GetEnvForAgent() returned %d vars, want %d", len(got), len(tt.want))
			}

			for k, wantV := range tt.want {
				if gotV, ok := got[k]; !ok {
					t.Errorf("GetEnvForAgent() missing key %q", k)
				} else if gotV != wantV {
					t.Errorf("GetEnvForAgent()[%q] = %q, want %q", k, gotV, wantV)
				}
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestActiveTaskStruct(t *testing.T) {
	now := time.Now()
	task := ActiveTask{
		ID:           "task-789",
		Ref:          "github:owner/repo#123",
		WorkDir:      "/work/task-789",
		State:        "planning",
		Branch:       "feature/task-789",
		UseGit:       true,
		WorktreePath: "/worktrees/task-789",
		Started:      now,
	}

	if task.ID != "task-789" {
		t.Errorf("ID = %q, want %q", task.ID, "task-789")
	}
	if task.Branch != "feature/task-789" {
		t.Errorf("Branch = %q, want %q", task.Branch, "feature/task-789")
	}
	if task.WorktreePath != "/worktrees/task-789" {
		t.Errorf("WorktreePath = %q, want %q", task.WorktreePath, "/worktrees/task-789")
	}
}

func TestTaskWorkStruct(t *testing.T) {
	now := time.Now()
	work := TaskWork{
		Version: "1",
		Metadata: WorkMetadata{
			ID:          "task-abc",
			Title:       "Test Task",
			CreatedAt:   now,
			UpdatedAt:   now,
			ExternalKey: "FEAT-123",
			TaskType:    "feature",
			Slug:        "test-task",
		},
		Source: SourceInfo{
			Type:    "directory",
			Ref:     ".mehrhof/plans/my-plan",
			ReadAt:  now,
			Content: "# Plan",
		},
		Git: GitInfo{
			Branch:        "feature/FEAT-123--test-task",
			BaseBranch:    "main",
			CommitPrefix:  "[FEAT-123]",
			BranchPattern: "{type}/{key}--{slug}",
		},
		Agent: AgentInfo{
			Name:   "claude",
			Source: "workspace",
		},
	}

	if work.Metadata.ExternalKey != "FEAT-123" {
		t.Errorf("Metadata.ExternalKey = %q, want %q", work.Metadata.ExternalKey, "FEAT-123")
	}
	if work.Git.CommitPrefix != "[FEAT-123]" {
		t.Errorf("Git.CommitPrefix = %q, want %q", work.Git.CommitPrefix, "[FEAT-123]")
	}
	if work.Agent.Name != "claude" {
		t.Errorf("Agent.Name = %q, want %q", work.Agent.Name, "claude")
	}
}

func TestSourceInfoStruct(t *testing.T) {
	now := time.Now()
	source := SourceInfo{
		Type:   "directory",
		Ref:    ".mehrhof/plans/my-plan",
		ReadAt: now,
		Files: []SourceFile{
			{Path: "task.md", Content: "# Task"},
			{Path: "notes.md", Content: "# Notes"},
		},
	}

	if len(source.Files) != 2 {
		t.Errorf("Files length = %d, want 2", len(source.Files))
	}
	if source.Files[0].Path != "task.md" {
		t.Errorf("Files[0].Path = %q, want %q", source.Files[0].Path, "task.md")
	}
}

func TestSessionStruct(t *testing.T) {
	now := time.Now()
	session := Session{
		Version: "1",
		Kind:    "Session",
		Metadata: SessionMetadata{
			StartedAt: now,
			EndedAt:   now.Add(time.Hour),
			Type:      "implementation",
			Agent:     "claude",
			State:     "implementing",
		},
		Usage: &UsageInfo{
			InputTokens:  1000,
			OutputTokens: 500,
			CachedTokens: 200,
			CostUSD:      0.05,
		},
		Exchanges: []Exchange{
			{
				Role:      "user",
				Timestamp: now,
				Content:   "Implement the feature",
			},
			{
				Role:      "agent",
				Timestamp: now.Add(time.Minute),
				Content:   "Done!",
				FilesChanged: []FileChange{
					{Path: "main.go", Operation: "update"},
				},
			},
		},
	}

	if session.Usage.InputTokens != 1000 {
		t.Errorf("Usage.InputTokens = %d, want 1000", session.Usage.InputTokens)
	}
	if len(session.Exchanges) != 2 {
		t.Errorf("Exchanges length = %d, want 2", len(session.Exchanges))
	}
	if len(session.Exchanges[1].FilesChanged) != 1 {
		t.Errorf("FilesChanged length = %d, want 1", len(session.Exchanges[1].FilesChanged))
	}
}

func TestCheckpointStruct(t *testing.T) {
	now := time.Now()
	cp := Checkpoint{
		ID:        "cp-001",
		Commit:    "abc123def456",
		Message:   "Checkpoint after planning",
		State:     "planning",
		CreatedAt: now,
	}

	if cp.ID != "cp-001" {
		t.Errorf("ID = %q, want %q", cp.ID, "cp-001")
	}
	if cp.Commit != "abc123def456" {
		t.Errorf("Commit = %q, want %q", cp.Commit, "abc123def456")
	}
}

func TestSpecificationStruct(t *testing.T) {
	now := time.Now()
	spec := Specification{
		Number:      1,
		Title:       "Feature Specification",
		Description: "Implement the feature",
		Status:      SpecificationStatusReady,
		CreatedAt:   now,
		UpdatedAt:   now,
		Sections:    []string{"Overview", "Requirements", "Implementation"},
		Content:     "# Feature Specification\n\n## Overview\n...",
	}

	if spec.Number != 1 {
		t.Errorf("Number = %d, want 1", spec.Number)
	}
	if spec.Status != "ready" {
		t.Errorf("Status = %q, want %q", spec.Status, "ready")
	}
	if len(spec.Sections) != 3 {
		t.Errorf("Sections length = %d, want 3", len(spec.Sections))
	}
}

func TestNoteStruct(t *testing.T) {
	now := time.Now()
	note := Note{
		Timestamp: now,
		Content:   "This is a note",
		State:     "idle",
	}

	if note.Content != "This is a note" {
		t.Errorf("Content = %q, want %q", note.Content, "This is a note")
	}
	if note.State != "idle" {
		t.Errorf("State = %q, want %q", note.State, "idle")
	}
}

func TestNotesFileStruct(t *testing.T) {
	notes := NotesFile{
		Notes: []Note{
			{Content: "Note 1"},
			{Content: "Note 2"},
		},
	}

	if len(notes.Notes) != 2 {
		t.Errorf("Notes length = %d, want 2", len(notes.Notes))
	}
}

func TestAgentInfoStruct(t *testing.T) {
	info := AgentInfo{
		Name:      "claude",
		Source:    "workspace",
		InlineEnv: map[string]string{"API_KEY": "secret"},
		Steps: map[string]StepAgentInfo{
			"planning": {
				Name:      "glm",
				Source:    "cli-step",
				InlineEnv: map[string]string{"MODEL": "glm-4"},
			},
		},
	}

	if info.Name != "claude" {
		t.Errorf("Name = %q, want %q", info.Name, "claude")
	}
	if info.Steps["planning"].Name != "glm" {
		t.Errorf("Steps[planning].Name = %q, want %q", info.Steps["planning"].Name, "glm")
	}
}
