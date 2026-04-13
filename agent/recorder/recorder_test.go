package recorder

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNew_RequiresJobID(t *testing.T) {
	cfg := DefaultConfig()
	cfg.JobID = ""
	_, err := New(cfg)
	if err == nil {
		t.Error("New() should require job ID")
	}
}

func TestRecorder_BasicRecording(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Dir:   dir,
		JobID: "test-job-123",
		Agent: "claude",
	}

	r, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer func() { _ = r.Close() }()

	// Record inbound
	if err := r.RecordInbound("Hello, agent"); err != nil {
		t.Fatalf("RecordInbound() error: %v", err)
	}

	// Record outbound
	event := map[string]string{"type": "stream", "content": "Hi there"}
	if err := r.RecordOutbound("stream", event); err != nil {
		t.Fatalf("RecordOutbound() error: %v", err)
	}

	// Flush and check line count
	if err := r.Flush(); err != nil {
		t.Fatalf("Flush() error: %v", err)
	}

	// Header + 2 records = 3 lines
	if r.LineCount() != 3 {
		t.Errorf("LineCount() = %d, want 3", r.LineCount())
	}
}

func TestRecorder_FileCreation(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Dir:   dir,
		JobID: "file-test",
		Agent: "codex",
	}

	r, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	path := r.Path()
	_ = r.Close()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("Recording file not created: %s", path)
	}

	// File should contain header
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	var header Header
	if err := json.Unmarshal(content[:len(content)-1], &header); err != nil {
		t.Fatalf("Unmarshal header error: %v", err)
	}

	if header.JobID != "file-test" {
		t.Errorf("Header.JobID = %q, want %q", header.JobID, "file-test")
	}
	if header.Agent != "codex" {
		t.Errorf("Header.Agent = %q, want %q", header.Agent, "codex")
	}
}

func TestRecorder_Rotation(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Dir:      dir,
		JobID:    "rotate-test",
		Agent:    "claude",
		MaxLines: 5, // Rotate after 5 lines (header + 4 records)
	}

	r, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer func() { _ = r.Close() }()

	// Write enough records to trigger rotation
	for range 10 {
		if err := r.RecordInbound("test"); err != nil {
			t.Fatalf("RecordInbound() error: %v", err)
		}
	}
	_ = r.Flush()

	// Should have created multiple files
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir error: %v", err)
	}

	if len(entries) < 2 {
		t.Errorf("Expected multiple files after rotation, got %d", len(entries))
	}
}

func TestReader_ReadAll(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Dir:   dir,
		JobID: "read-test",
		Agent: "claude",
	}

	r, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Write some records
	_ = r.RecordInbound("prompt1")
	_ = r.RecordOutbound("stream", map[string]string{"content": "response"})
	path := r.Path()
	_ = r.Close()

	// Read them back
	records, err := ReadAll(path)
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}

	if len(records) != 2 {
		t.Errorf("ReadAll() returned %d records, want 2", len(records))
	}

	// Check first record
	if records[0].Direction != Inbound {
		t.Errorf("records[0].Direction = %q, want %q", records[0].Direction, Inbound)
	}
	if records[0].Type != "prompt" {
		t.Errorf("records[0].Type = %q, want %q", records[0].Type, "prompt")
	}
}

func TestReader_Header(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Dir:     dir,
		JobID:   "header-test",
		Agent:   "claude",
		Model:   "opus",
		WorkDir: "/test/project",
	}

	r, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	path := r.Path()
	_ = r.Close()

	reader, err := OpenReader(path)
	if err != nil {
		t.Fatalf("OpenReader() error: %v", err)
	}
	defer func() { _ = reader.Close() }()

	h := reader.Header()
	if h == nil {
		t.Fatal("Header() returned nil")
	}
	if h.JobID != "header-test" {
		t.Errorf("Header.JobID = %q, want %q", h.JobID, "header-test")
	}
	if h.Agent != "claude" {
		t.Errorf("Header.Agent = %q, want %q", h.Agent, "claude")
	}
	if h.Model != "opus" {
		t.Errorf("Header.Model = %q, want %q", h.Model, "opus")
	}
}

func TestListRecordings(t *testing.T) {
	dir := t.TempDir()

	// Create a few recordings
	for _, jobID := range []string{"job1", "job2", "job3"} {
		cfg := Config{
			Dir:   dir,
			JobID: jobID,
			Agent: "claude",
		}
		r, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error: %v", err)
		}
		_ = r.RecordInbound("test")
		_ = r.Close()
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// List them
	infos, err := ListRecordings(dir)
	if err != nil {
		t.Fatalf("ListRecordings() error: %v", err)
	}

	if len(infos) != 3 {
		t.Errorf("ListRecordings() returned %d infos, want 3", len(infos))
	}

	// Should be sorted newest first
	if infos[0].JobID != "job3" {
		t.Errorf("First recording should be job3 (newest), got %q", infos[0].JobID)
	}
}

func TestListRecordings_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	infos, err := ListRecordings(dir)
	if err != nil {
		t.Fatalf("ListRecordings() error: %v", err)
	}
	if len(infos) != 0 {
		t.Errorf("ListRecordings() on empty dir returned %d infos", len(infos))
	}
}

func TestListRecordings_NonexistentDir(t *testing.T) {
	infos, err := ListRecordings("/nonexistent/path")
	if err != nil {
		t.Fatalf("ListRecordings() error: %v", err)
	}
	if infos != nil {
		t.Errorf("ListRecordings() on nonexistent dir should return nil")
	}
}

func TestListSessionRecordings(t *testing.T) {
	dir := t.TempDir()

	// Create recordings for different jobs
	for range 3 {
		cfg := Config{
			Dir:   dir,
			JobID: "target-job",
			Agent: "claude",
		}
		r, err := New(cfg)
		if err != nil {
			t.Fatalf("New(cfg) failed: %v", err)
		}
		_ = r.RecordInbound("test")
		_ = r.Close()
		time.Sleep(10 * time.Millisecond)
	}

	cfg := Config{
		Dir:   dir,
		JobID: "other-job",
		Agent: "claude",
	}
	r, err := New(cfg)
	if err != nil {
		t.Fatalf("New(cfg) failed: %v", err)
	}
	_ = r.RecordInbound("test")
	_ = r.Close()

	// Should only return target-job recordings
	infos, err := ListSessionRecordings(dir, "target-job")
	if err != nil {
		t.Fatalf("ListSessionRecordings() error: %v", err)
	}

	if len(infos) != 3 {
		t.Errorf("ListSessionRecordings() returned %d infos, want 3", len(infos))
	}
}

func TestFilterRecords(t *testing.T) {
	records := []Record{
		{JobID: "job1", Direction: Inbound, Type: "prompt"},
		{JobID: "job1", Direction: Outbound, Type: "stream"},
		{JobID: "job2", Direction: Outbound, Type: "stream"},
		{JobID: "job1", Direction: Outbound, Type: "complete"},
	}

	// Filter by job
	filtered := FilterRecords(records, Filter{JobID: "job1"})
	if len(filtered) != 3 {
		t.Errorf("Filter by JobID returned %d, want 3", len(filtered))
	}

	// Filter by direction
	filtered = FilterRecords(records, Filter{Direction: Outbound})
	if len(filtered) != 3 {
		t.Errorf("Filter by Direction returned %d, want 3", len(filtered))
	}

	// Filter by type
	filtered = FilterRecords(records, Filter{Types: []string{"stream"}})
	if len(filtered) != 2 {
		t.Errorf("Filter by Types returned %d, want 2", len(filtered))
	}

	// Combined filter
	filtered = FilterRecords(records, Filter{JobID: "job1", Direction: Outbound})
	if len(filtered) != 2 {
		t.Errorf("Combined filter returned %d, want 2", len(filtered))
	}
}

func TestCleanOldRecordings(t *testing.T) {
	dir := t.TempDir()

	// Create an old file
	oldPath := filepath.Join(dir, "old_recording.jsonl")
	if err := os.WriteFile(oldPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	// Set modification time to past
	oldTime := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes error: %v", err)
	}

	// Create a new file
	newPath := filepath.Join(dir, "new_recording.jsonl")
	if err := os.WriteFile(newPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	// Clean files older than 24 hours ago
	cutoff := time.Now().Add(-24 * time.Hour).Unix()
	removed, err := CleanOldRecordings(dir, cutoff)
	if err != nil {
		t.Fatalf("CleanOldRecordings() error: %v", err)
	}

	if removed != 1 {
		t.Errorf("CleanOldRecordings() removed %d, want 1", removed)
	}

	// Old file should be gone
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("Old file should have been removed")
	}

	// New file should remain
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		t.Error("New file should not have been removed")
	}
}
