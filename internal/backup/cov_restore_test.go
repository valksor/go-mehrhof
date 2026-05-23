package backup

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// writeRawArchive builds a tar.gz with caller-supplied headers/contents, so
// tests can craft entries (symlinks, oversized, ordering) the helper in
// backup_test.go cannot express.
func writeRawArchive(t *testing.T, path string, build func(tw *tar.Writer)) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	gzw := gzip.NewWriter(f)
	tw := tar.NewWriter(gzw)
	build(tw)
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}
}

func TestRestore_SkipsSymlinkAndHardlinkEntries(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "links.tar.gz")
	writeRawArchive(t, archivePath, func(tw *tar.Writer) {
		// A regular file that should be restored.
		body := "real"
		_ = tw.WriteHeader(&tar.Header{Name: "ok.txt", Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg})
		_, _ = tw.Write([]byte(body))
		// A symlink and a hardlink, both must be skipped.
		_ = tw.WriteHeader(&tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "ok.txt"})
		_ = tw.WriteHeader(&tar.Header{Name: "hard", Typeflag: tar.TypeLink, Linkname: "ok.txt"})
	})

	restoreDir := t.TempDir()
	result, err := Restore(archivePath, restoreDir)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if result.Files != 1 {
		t.Errorf("Files = %d, want 1", result.Files)
	}
	if result.Skipped < 2 {
		t.Errorf("Skipped = %d, want >= 2 (symlink + hardlink)", result.Skipped)
	}
	if _, err := os.Lstat(filepath.Join(restoreDir, "link")); err == nil {
		t.Error("symlink should not have been restored")
	}
}

func TestRestore_RestoresDirectoryEntries(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "dirs.tar.gz")
	writeRawArchive(t, archivePath, func(tw *tar.Writer) {
		_ = tw.WriteHeader(&tar.Header{Name: "sub", Mode: 0o755, Typeflag: tar.TypeDir})
		body := "v"
		_ = tw.WriteHeader(&tar.Header{Name: "sub/file.txt", Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg})
		_, _ = tw.Write([]byte(body))
	})

	restoreDir := t.TempDir()
	result, err := Restore(archivePath, restoreDir)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if result.Dirs != 1 {
		t.Errorf("Dirs = %d, want 1", result.Dirs)
	}
	if result.Files != 1 {
		t.Errorf("Files = %d, want 1", result.Files)
	}
	info, err := os.Stat(filepath.Join(restoreDir, "sub"))
	if err != nil || !info.IsDir() {
		t.Errorf("expected restored directory 'sub': %v", err)
	}
}

func TestRestore_SkipsUnknownTypeflag(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "fifo.tar.gz")
	writeRawArchive(t, archivePath, func(tw *tar.Writer) {
		_ = tw.WriteHeader(&tar.Header{Name: "pipe", Mode: 0o644, Typeflag: tar.TypeFifo})
	})

	result, err := Restore(archivePath, t.TempDir())
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if result.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1 (unknown typeflag)", result.Skipped)
	}
}

func TestRestore_ManifestMustBeFirstEntry(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "lateman.tar.gz")
	writeRawArchive(t, archivePath, func(tw *tar.Writer) {
		// A user file precedes the manifest — Restore must reject this.
		body := "first"
		_ = tw.WriteHeader(&tar.Header{Name: "early.txt", Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg})
		_, _ = tw.Write([]byte(body))
		man := `{"format_version":1}`
		_ = tw.WriteHeader(&tar.Header{Name: manifestName, Mode: 0o600, Size: int64(len(man)), Typeflag: tar.TypeReg})
		_, _ = tw.Write([]byte(man))
	})

	_, err := Restore(archivePath, t.TempDir())
	if err == nil {
		t.Fatal("Restore should reject manifest that is not the first entry")
	}
	if !strings.Contains(err.Error(), "first archive entry") {
		t.Errorf("error = %q, want mention of 'first archive entry'", err.Error())
	}
}

func TestRestore_FileCreateFailureSurfacesError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission-based file create error not reliable on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses directory permission checks")
	}

	archivePath := filepath.Join(t.TempDir(), "ok.tar.gz")
	writeRawArchive(t, archivePath, func(tw *tar.Writer) {
		body := "data"
		_ = tw.WriteHeader(&tar.Header{Name: "f.txt", Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg})
		_, _ = tw.Write([]byte(body))
	})

	// Restore into a read-only directory so creating the output file fails.
	restoreDir := t.TempDir()
	if err := os.Chmod(restoreDir, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(restoreDir, 0o700) })

	if _, err := Restore(archivePath, restoreDir); err == nil {
		t.Fatal("Restore should fail when the target file cannot be created")
	}
}

func TestRestore_RejectsCorruptGzip(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "corrupt.tar.gz")
	if err := os.WriteFile(archivePath, []byte("this is not gzip data"), 0o600); err != nil {
		t.Fatalf("write corrupt archive: %v", err)
	}

	_, err := Restore(archivePath, t.TempDir())
	if err == nil {
		t.Fatal("Restore should reject a non-gzip archive")
	}
	if !strings.Contains(err.Error(), "gzip") {
		t.Errorf("error = %q, want gzip read failure", err.Error())
	}
}

func TestRestore_RejectsTruncatedTar(t *testing.T) {
	// Valid gzip wrapping garbage tar bytes triggers a tar-read error.
	archivePath := filepath.Join(t.TempDir(), "badtar.tar.gz")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	gzw := gzip.NewWriter(f)
	if _, err := gzw.Write([]byte("not a valid tar stream at all, just bytes")); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	_ = gzw.Close()
	_ = f.Close()

	if _, err := Restore(archivePath, t.TempDir()); err == nil {
		t.Fatal("Restore should reject a garbage tar stream")
	}
}
