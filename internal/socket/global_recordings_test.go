package socket

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/valksor/kvelmo/agent/recorder"
	"github.com/valksor/kvelmo/paths"
)

// createTestRecording creates a recording file in the given directory and returns its path.
func createTestRecording(t *testing.T, dir, jobID, agent string) string {
	t.Helper()

	rec, err := recorder.New(recorder.Config{
		Dir:     dir,
		JobID:   jobID,
		Agent:   agent,
		WorkDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("create recorder: %v", err)
	}

	// Write a test record
	if err := rec.RecordInbound("test prompt"); err != nil {
		t.Fatalf("record inbound: %v", err)
	}

	path := rec.Path()
	if err := rec.Close(); err != nil {
		t.Fatalf("close recorder: %v", err)
	}

	return path
}

// withTestRecordingsDir sets paths to use a temp dir and returns the recordings subdirectory.
func withTestRecordingsDir(t *testing.T) string {
	t.Helper()

	baseDir := t.TempDir()
	paths.SetPaths(paths.NewPathResolver(baseDir))
	t.Cleanup(func() { paths.SetPaths(nil) })

	return filepath.Join(baseDir, "recordings")
}

// ============================================================
// handleRecordingsList tests
// ============================================================

func TestGlobalHandleRecordingsList_EmptyDir(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	_ = withTestRecordingsDir(t)

	resp, err := g.handleRecordingsList(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleRecordingsList() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleRecordingsList() returned error: %s", resp.Error.Message)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := result["recordings"]; !ok {
		t.Error("result should have 'recordings' key")
	}

	var recordings []recorder.RecordingInfo
	if err := json.Unmarshal(result["recordings"], &recordings); err != nil {
		t.Fatalf("unmarshal recordings: %v", err)
	}
	if len(recordings) != 0 {
		t.Errorf("expected 0 recordings, got %d", len(recordings))
	}
}

func TestGlobalHandleRecordingsList_WithRecordings(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	recDir := withTestRecordingsDir(t)

	createTestRecording(t, recDir, "job-1", "claude")
	createTestRecording(t, recDir, "job-2", "codex")

	resp, err := g.handleRecordingsList(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleRecordingsList() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleRecordingsList() returned error: %s", resp.Error.Message)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	var recordings []recorder.RecordingInfo
	if err := json.Unmarshal(result["recordings"], &recordings); err != nil {
		t.Fatalf("unmarshal recordings: %v", err)
	}
	if len(recordings) != 2 {
		t.Errorf("expected 2 recordings, got %d", len(recordings))
	}
}

func TestGlobalHandleRecordingsList_FilterByJob(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	recDir := withTestRecordingsDir(t)

	createTestRecording(t, recDir, "job-alpha", "claude")
	createTestRecording(t, recDir, "job-beta", "claude")

	params, _ := json.Marshal(recordingsListParams{Job: "job-alpha"}) //nolint:errchkjson // test data
	resp, err := g.handleRecordingsList(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleRecordingsList() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleRecordingsList() returned error: %s", resp.Error.Message)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	var recordings []recorder.RecordingInfo
	if err := json.Unmarshal(result["recordings"], &recordings); err != nil {
		t.Fatalf("unmarshal recordings: %v", err)
	}
	if len(recordings) != 1 {
		t.Fatalf("expected 1 recording for job-alpha, got %d", len(recordings))
	}
	if recordings[0].JobID != "job-alpha" {
		t.Errorf("job_id = %q, want %q", recordings[0].JobID, "job-alpha")
	}
}

func TestGlobalHandleRecordingsList_FilterBySince(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	recDir := withTestRecordingsDir(t)

	createTestRecording(t, recDir, "recent-job", "claude")

	// Filter with a generous "since" that should include the recording we just created
	params, _ := json.Marshal(recordingsListParams{Since: "1h"}) //nolint:errchkjson // test data
	resp, err := g.handleRecordingsList(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleRecordingsList() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleRecordingsList() returned error: %s", resp.Error.Message)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	var recordings []recorder.RecordingInfo
	if err := json.Unmarshal(result["recordings"], &recordings); err != nil {
		t.Fatalf("unmarshal recordings: %v", err)
	}
	if len(recordings) != 1 {
		t.Errorf("expected 1 recording within last hour, got %d", len(recordings))
	}
}

func TestGlobalHandleRecordingsList_InvalidSinceDuration(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	_ = withTestRecordingsDir(t)

	params, _ := json.Marshal(recordingsListParams{Since: "notaduration"}) //nolint:errchkjson // test data
	resp, err := g.handleRecordingsList(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleRecordingsList() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for invalid since duration")
	}
	if resp.Error.Code != ErrCodeInvalidParams {
		t.Errorf("error code = %d, want %d", resp.Error.Code, ErrCodeInvalidParams)
	}
}

func TestGlobalHandleRecordingsList_InvalidJSON(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	resp, err := g.handleRecordingsList(ctx, &Request{ID: "1", Params: json.RawMessage(`invalid`)})
	if err != nil {
		t.Fatalf("handleRecordingsList() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for invalid JSON params")
	}
}

func TestGlobalHandleRecordingsList_NilParams(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	_ = withTestRecordingsDir(t)

	resp, err := g.handleRecordingsList(ctx, &Request{ID: "1", Params: nil})
	if err != nil {
		t.Fatalf("handleRecordingsList() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleRecordingsList() returned error: %s", resp.Error.Message)
	}
}

// ============================================================
// handleRecordingsView tests
// ============================================================

func TestGlobalHandleRecordingsView_InvalidJSON(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	resp, err := g.handleRecordingsView(ctx, &Request{ID: "1", Params: json.RawMessage(`invalid`)})
	if err != nil {
		t.Fatalf("handleRecordingsView() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for invalid JSON params")
	}
}

func TestGlobalHandleRecordingsView_EmptyFile(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	params, _ := json.Marshal(recordingsViewParams{File: ""}) //nolint:errchkjson // test data
	resp, err := g.handleRecordingsView(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleRecordingsView() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for empty file")
	}
	if resp.Error.Code != ErrCodeInvalidParams {
		t.Errorf("error code = %d, want %d", resp.Error.Code, ErrCodeInvalidParams)
	}
}

func TestGlobalHandleRecordingsView_NonexistentFile(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	params, _ := json.Marshal(recordingsViewParams{File: "/nonexistent/recording.jsonl"}) //nolint:errchkjson // test data
	resp, err := g.handleRecordingsView(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleRecordingsView() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for nonexistent file")
	}
	if resp.Error.Code != ErrCodeInternal {
		t.Errorf("error code = %d, want %d", resp.Error.Code, ErrCodeInternal)
	}
}

func TestGlobalHandleRecordingsView_ValidFile(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	recDir := withTestRecordingsDir(t)

	path := createTestRecording(t, recDir, "view-job", "claude")

	params, _ := json.Marshal(recordingsViewParams{File: path}) //nolint:errchkjson // test data
	resp, err := g.handleRecordingsView(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleRecordingsView() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleRecordingsView() returned error: %s", resp.Error.Message)
	}

	var result recordingViewResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if result.Header == nil {
		t.Fatal("expected non-nil header")
	}
	if result.Header.JobID != "view-job" {
		t.Errorf("header job_id = %q, want %q", result.Header.JobID, "view-job")
	}
	if result.Header.Agent != "claude" {
		t.Errorf("header agent = %q, want %q", result.Header.Agent, "claude")
	}
	if len(result.Records) != 1 {
		t.Errorf("expected 1 record, got %d", len(result.Records))
	}
}

func TestGlobalHandleRecordingsView_RelativePath(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)
	recDir := withTestRecordingsDir(t)

	path := createTestRecording(t, recDir, "rel-job", "claude")
	relPath := filepath.Base(path)

	params, _ := json.Marshal(recordingsViewParams{File: relPath}) //nolint:errchkjson // test data
	resp, err := g.handleRecordingsView(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleRecordingsView() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleRecordingsView() returned error: %s", resp.Error.Message)
	}

	var result recordingViewResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Header == nil {
		t.Fatal("expected non-nil header")
	}
	if result.Header.JobID != "rel-job" {
		t.Errorf("header job_id = %q, want %q", result.Header.JobID, "rel-job")
	}
}

// ============================================================
// parseDurationString tests
// ============================================================

func TestParseDurationString_Hours(t *testing.T) {
	d, err := parseDurationString("24h")
	if err != nil {
		t.Fatalf("parseDurationString(24h) error = %v", err)
	}
	if d != 24*time.Hour {
		t.Errorf("duration = %v, want %v", d, 24*time.Hour)
	}
}

func TestParseDurationString_Days(t *testing.T) {
	d, err := parseDurationString("7d")
	if err != nil {
		t.Fatalf("parseDurationString(7d) error = %v", err)
	}
	if d != 7*24*time.Hour {
		t.Errorf("duration = %v, want %v", d, 7*24*time.Hour)
	}
}

func TestParseDurationString_Minutes(t *testing.T) {
	d, err := parseDurationString("30m")
	if err != nil {
		t.Fatalf("parseDurationString(30m) error = %v", err)
	}
	if d != 30*time.Minute {
		t.Errorf("duration = %v, want %v", d, 30*time.Minute)
	}
}

func TestParseDurationString_InvalidDays(t *testing.T) {
	_, err := parseDurationString("xd")
	if err == nil {
		t.Fatal("expected error for invalid day duration")
	}
}

func TestParseDurationString_InvalidFormat(t *testing.T) {
	_, err := parseDurationString("notaduration")
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
}
