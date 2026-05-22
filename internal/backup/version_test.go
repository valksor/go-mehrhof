package backup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBackupManifestIsFirstEntry(t *testing.T) {
	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(t.TempDir(), "b.tar.gz")

	result, err := Create(srcDir, outPath)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if result.FormatVersion != CurrentBackupVersion {
		t.Errorf("Result.FormatVersion = %d, want %d", result.FormatVersion, CurrentBackupVersion)
	}
	if result.Files != 1 {
		t.Errorf("Result.Files = %d, want 1 (manifest is not counted)", result.Files)
	}

	names := listArchiveEntries(t, outPath)
	if len(names) == 0 || names[0] != manifestName {
		t.Errorf("first archive entry = %v, want manifest %q first", names, manifestName)
	}
}

func TestRestoreRejectsNewerManifest(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "newer.tar.gz")
	createTestArchive(t, archivePath, map[string]string{
		manifestName: `{"format_version":9999}`,
	})

	if _, err := Restore(archivePath, t.TempDir()); err == nil {
		t.Fatal("Restore() expected error for archive newer than supported version")
	}
}

func TestRestoreLegacyArchiveNoManifest(t *testing.T) {
	// A pre-versioning archive (no manifest) restores normally.
	archivePath := filepath.Join(t.TempDir(), "legacy.tar.gz")
	createTestArchive(t, archivePath, map[string]string{"config.yaml": "k: v"})

	restoreDir := t.TempDir()
	result, err := Restore(archivePath, restoreDir)
	if err != nil {
		t.Fatalf("Restore legacy archive: %v", err)
	}
	if result.Files != 1 {
		t.Errorf("Result.Files = %d, want 1", result.Files)
	}

	data, err := os.ReadFile(filepath.Join(restoreDir, "config.yaml"))
	if err != nil || string(data) != "k: v" {
		t.Errorf("restored content = %q (err %v), want %q", data, err, "k: v")
	}
}
