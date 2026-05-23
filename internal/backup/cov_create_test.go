package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReadManifestEntry_TooLarge(t *testing.T) {
	// A reader is required by signature but is never read when size is rejected.
	tr := tar.NewReader(bytes.NewReader(nil))
	err := readManifestEntry(tr, maxManifestSize+1)
	if err == nil {
		t.Fatal("expected error for oversized manifest")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("error = %q, want 'too large'", err.Error())
	}
}

func TestReadManifestEntry_MalformedJSON(t *testing.T) {
	bad := []byte("{not valid json")
	tr := tar.NewReader(bytes.NewReader(buildSingleEntryTar(t, "m", bad)))
	// Advance to the entry so the reader yields its body.
	if _, err := tr.Next(); err != nil {
		t.Fatalf("tar.Next: %v", err)
	}
	err := readManifestEntry(tr, int64(len(bad)))
	if err == nil {
		t.Fatal("expected error for malformed manifest JSON")
	}
	if !strings.Contains(err.Error(), "parse backup manifest") {
		t.Errorf("error = %q, want parse failure", err.Error())
	}
}

func TestReadManifestEntry_ValidCurrentVersion(t *testing.T) {
	good := []byte(`{"format_version":1,"created_at":"now"}`)
	tr := tar.NewReader(bytes.NewReader(buildSingleEntryTar(t, "m", good)))
	if _, err := tr.Next(); err != nil {
		t.Fatalf("tar.Next: %v", err)
	}
	if err := readManifestEntry(tr, int64(len(good))); err != nil {
		t.Errorf("readManifestEntry on valid manifest: %v", err)
	}
}

// buildSingleEntryTar returns raw (uncompressed) tar bytes with one regular entry.
func buildSingleEntryTar(t *testing.T, name string, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatalf("write header: %v", err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatalf("write data: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}

	return buf.Bytes()
}

func TestCreate_OutputPathParentMissing(t *testing.T) {
	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Parent of the output path does not exist -> os.Create inside writeArchive fails.
	out := filepath.Join(srcDir, "no", "such", "dir", "out.tar.gz")
	if _, err := Create(srcDir, out); err == nil {
		t.Fatal("Create should fail when output parent directory is missing")
	}
}

func TestCreate_WalkErrorCleansUpArchive(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission-based walk error not reliable on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses directory permission checks")
	}

	srcDir := t.TempDir()
	// A subdirectory with no read/execute permission makes WalkDir return an
	// error mid-walk, which exercises closeWriters + archive removal cleanup.
	locked := filepath.Join(srcDir, "locked")
	if err := os.Mkdir(locked, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(locked, "inner.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("seed inner: %v", err)
	}
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	out := filepath.Join(t.TempDir(), "out.tar.gz")
	if _, err := Create(srcDir, out); err == nil {
		t.Fatal("Create should fail when a subdirectory is unreadable")
	}
	// On failure the partial archive must be removed.
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("partial archive should be removed on walk failure, stat err = %v", err)
	}
}

func TestCreate_GzipRoundTripIncludesManifest(t *testing.T) {
	// Sanity check that the produced archive is a valid gzip stream and that the
	// first entry is the manifest (guards the writeManifest happy path).
	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	out := filepath.Join(t.TempDir(), "ok.tar.gz")
	if _, err := Create(srcDir, out); err != nil {
		t.Fatalf("Create: %v", err)
	}

	f, err := os.Open(out)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	tr := tar.NewReader(gz)
	h, err := tr.Next()
	if err != nil {
		t.Fatalf("tar.Next: %v", err)
	}
	if h.Name != manifestName {
		t.Errorf("first entry = %q, want manifest %q", h.Name, manifestName)
	}
}
