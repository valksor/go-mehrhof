package socket

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/valksor/kvelmo/internal/backup"
	"github.com/valksor/kvelmo/paths"
)

// ============================================================
// handleBackupCreate tests
// ============================================================

func TestGlobalHandleBackupCreate_InvalidJSON(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	resp, err := g.handleBackupCreate(ctx, &Request{ID: "1", Params: json.RawMessage(`invalid`)})
	if err != nil {
		t.Fatalf("handleBackupCreate() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for invalid JSON params")
	}
	if resp.Error.Code != ErrCodeInvalidParams {
		t.Errorf("error code = %d, want %d", resp.Error.Code, ErrCodeInvalidParams)
	}
}

func TestGlobalHandleBackupCreate_NilParams(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	baseDir := t.TempDir()
	paths.SetPaths(paths.NewPathResolver(baseDir))
	t.Cleanup(func() { paths.SetPaths(nil) })

	// Create a test file so the backup has something to archive
	if err := os.WriteFile(filepath.Join(baseDir, "test.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	resp, err := g.handleBackupCreate(ctx, &Request{ID: "1", Params: nil})
	if err != nil {
		t.Fatalf("handleBackupCreate() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleBackupCreate() returned error: %s", resp.Error.Message)
	}

	var result backup.Result
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Path == "" {
		t.Error("expected non-empty backup path")
	}
	if result.Files < 1 {
		t.Errorf("expected at least 1 file in backup, got %d", result.Files)
	}

	// Clean up the created backup file
	t.Cleanup(func() { _ = os.Remove(result.Path) })
}

func TestGlobalHandleBackupCreate_WithOutputPath(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	baseDir := t.TempDir()
	paths.SetPaths(paths.NewPathResolver(baseDir))
	t.Cleanup(func() { paths.SetPaths(nil) })

	// Create a test file
	if err := os.WriteFile(filepath.Join(baseDir, "config.yaml"), []byte("key: value"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	outputPath := filepath.Join(t.TempDir(), "test-backup.tar.gz")
	params, _ := json.Marshal(backupCreateParams{OutputPath: outputPath}) //nolint:errchkjson // test data
	resp, err := g.handleBackupCreate(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleBackupCreate() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleBackupCreate() returned error: %s", resp.Error.Message)
	}

	var result backup.Result
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Path != outputPath {
		t.Errorf("path = %q, want %q", result.Path, outputPath)
	}
	if result.Size <= 0 {
		t.Errorf("expected positive backup size, got %d", result.Size)
	}

	// Verify the file actually exists
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("backup file was not created at the specified output path")
	}
}

func TestGlobalHandleBackupCreate_NonexistentBaseDir(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	paths.SetPaths(paths.NewPathResolver("/nonexistent/base/dir"))
	t.Cleanup(func() { paths.SetPaths(nil) })

	resp, err := g.handleBackupCreate(ctx, &Request{ID: "1", Params: nil})
	if err != nil {
		t.Fatalf("handleBackupCreate() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for nonexistent base directory")
	}
	if resp.Error.Code != ErrCodeInternal {
		t.Errorf("error code = %d, want %d", resp.Error.Code, ErrCodeInternal)
	}
}

// ============================================================
// handleBackupList tests
// ============================================================

func TestGlobalHandleBackupList_EmptyDir(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	baseDir := t.TempDir()
	paths.SetPaths(paths.NewPathResolver(baseDir))
	t.Cleanup(func() { paths.SetPaths(nil) })

	resp, err := g.handleBackupList(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleBackupList() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleBackupList() returned error: %s", resp.Error.Message)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := result["backups"]; !ok {
		t.Error("result should have 'backups' key")
	}

	var backups []backup.BackupInfo
	if err := json.Unmarshal(result["backups"], &backups); err != nil {
		t.Fatalf("unmarshal backups: %v", err)
	}
	if len(backups) != 0 {
		t.Errorf("expected 0 backups, got %d", len(backups))
	}
}

func TestGlobalHandleBackupList_WithBackups(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	baseDir := t.TempDir()
	paths.SetPaths(paths.NewPathResolver(baseDir))
	t.Cleanup(func() { paths.SetPaths(nil) })

	// Create a test file so backup has content
	if err := os.WriteFile(filepath.Join(baseDir, "data.txt"), []byte("test"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// Create a backup using the backup package directly
	result, err := backup.Create(baseDir, filepath.Join(baseDir, "kvelmo-backup-20260101-120000.tar.gz"))
	if err != nil {
		t.Fatalf("create test backup: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil backup result")
	}

	resp, listErr := g.handleBackupList(ctx, &Request{ID: "1"})
	if listErr != nil {
		t.Fatalf("handleBackupList() error = %v", listErr)
	}
	if resp.Error != nil {
		t.Fatalf("handleBackupList() returned error: %s", resp.Error.Message)
	}

	var listResult map[string]json.RawMessage
	if err := json.Unmarshal(resp.Result, &listResult); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	var backups []backup.BackupInfo
	if err := json.Unmarshal(listResult["backups"], &backups); err != nil {
		t.Fatalf("unmarshal backups: %v", err)
	}
	if len(backups) != 1 {
		t.Fatalf("expected 1 backup, got %d", len(backups))
	}
	if backups[0].Name != "kvelmo-backup-20260101-120000.tar.gz" {
		t.Errorf("backup name = %q, want %q", backups[0].Name, "kvelmo-backup-20260101-120000.tar.gz")
	}
	if backups[0].Size <= 0 {
		t.Errorf("expected positive backup size, got %d", backups[0].Size)
	}
}

func TestGlobalHandleBackupList_IgnoresNonBackupFiles(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	baseDir := t.TempDir()
	paths.SetPaths(paths.NewPathResolver(baseDir))
	t.Cleanup(func() { paths.SetPaths(nil) })

	// Create files that don't match the backup pattern
	for _, name := range []string{"random.txt", "other.tar.gz", "kvelmo-config.yaml"} {
		if err := os.WriteFile(filepath.Join(baseDir, name), []byte("data"), 0o644); err != nil {
			t.Fatalf("write test file: %v", err)
		}
	}

	resp, err := g.handleBackupList(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleBackupList() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleBackupList() returned error: %s", resp.Error.Message)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	var backups []backup.BackupInfo
	if err := json.Unmarshal(result["backups"], &backups); err != nil {
		t.Fatalf("unmarshal backups: %v", err)
	}
	if len(backups) != 0 {
		t.Errorf("expected 0 backups (no matching files), got %d", len(backups))
	}
}
