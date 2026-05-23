package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sha256Hex returns the lowercase hex SHA256 of b.
func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)

	return hex.EncodeToString(sum[:])
}

func TestNewDownloaderAndSetStageDir(t *testing.T) {
	d := NewDownloader()
	if d == nil {
		t.Fatal("NewDownloader() returned nil")
	}
	if d.client == nil {
		t.Fatal("NewDownloader() did not set http client")
	}
	if d.stageDir != "" {
		t.Fatalf("stageDir = %q, want empty", d.stageDir)
	}

	dir := t.TempDir()
	d.SetStageDir(dir)
	if d.stageDir != dir {
		t.Fatalf("SetStageDir did not set stageDir, got %q want %q", d.stageDir, dir)
	}
}

func TestDownload_SuccessNoChecksum(t *testing.T) {
	payload := []byte("the-binary-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	d := NewDownloader()
	stage := t.TempDir()
	d.SetStageDir(stage)

	path, err := d.Download(context.Background(), srv.URL, "")
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	defer func() { _ = os.Remove(path) }()

	if filepath.Dir(path) != stage {
		t.Errorf("downloaded file = %q, expected inside stage dir %q", path, stage)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Errorf("downloaded content = %q, want %q", got, payload)
	}
}

func TestDownload_SuccessWithMatchingChecksum(t *testing.T) {
	payload := []byte("verified-binary")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	d := NewDownloader()
	d.SetStageDir(t.TempDir())

	// Case-insensitive: pass uppercase checksum.
	path, err := d.Download(context.Background(), srv.URL, strings.ToUpper(sha256Hex(payload)))
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	_ = os.Remove(path)
}

func TestDownload_ChecksumMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("actual-bytes"))
	}))
	defer srv.Close()

	d := NewDownloader()
	stage := t.TempDir()
	d.SetStageDir(stage)

	_, err := d.Download(context.Background(), srv.URL, sha256Hex([]byte("different-bytes")))
	if !errors.Is(err, ErrChecksumFailed) {
		t.Fatalf("Download() error = %v, want ErrChecksumFailed", err)
	}

	// Temp file must be cleaned up on checksum failure.
	entries, _ := os.ReadDir(stage)
	if len(entries) != 0 {
		t.Errorf("stage dir not cleaned up, has %d entries", len(entries))
	}
}

func TestDownload_Non200Status(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	d := NewDownloader()
	stage := t.TempDir()
	d.SetStageDir(stage)

	_, err := d.Download(context.Background(), srv.URL, "")
	if !errors.Is(err, ErrDownloadFailed) {
		t.Fatalf("Download() error = %v, want ErrDownloadFailed", err)
	}
	entries, _ := os.ReadDir(stage)
	if len(entries) != 0 {
		t.Errorf("stage dir not cleaned up, has %d entries", len(entries))
	}
}

func TestDownload_BadURL(t *testing.T) {
	d := NewDownloader()
	d.SetStageDir(t.TempDir())

	// Control characters make NewRequestWithContext fail.
	_, err := d.Download(context.Background(), "http://\x00bad", "")
	if !errors.Is(err, ErrDownloadFailed) {
		t.Fatalf("Download() error = %v, want ErrDownloadFailed", err)
	}
}

func TestDownload_ConnectionRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // Close immediately so the request fails.

	d := NewDownloader()
	d.SetStageDir(t.TempDir())

	_, err := d.Download(context.Background(), url, "")
	if !errors.Is(err, ErrDownloadFailed) {
		t.Fatalf("Download() error = %v, want ErrDownloadFailed", err)
	}
}

func TestDownloadWithSignature_FullSuccess(t *testing.T) {
	binary := []byte("kvelmo-binary-content")
	assetName := "kvelmo-linux-amd64"
	checksums := sha256Hex(binary) + "  " + assetName + "\n"

	mux := http.NewServeMux()
	mux.HandleFunc("/binary", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(binary)
	})
	mux.HandleFunc("/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(checksums))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	d := NewDownloader()
	d.SetStageDir(t.TempDir())

	// requireSignature=false and a 404 signature URL -> signature skipped, checksum still verified.
	path, result, err := d.DownloadWithSignature(
		context.Background(),
		srv.URL+"/binary", srv.URL+"/checksums.txt", srv.URL+"/missing.minisig",
		assetName, testMinisignPublicKey, false,
	)
	if err != nil {
		t.Fatalf("DownloadWithSignature() error = %v", err)
	}
	defer func() { _ = os.Remove(path) }()

	if !result.SignatureSkipped {
		t.Error("expected SignatureSkipped=true when signature 404s and not required")
	}
	if result.SignatureError == "" {
		t.Error("expected SignatureError to be populated when skipped")
	}
	if !result.ChecksumVerified {
		t.Error("expected ChecksumVerified=true")
	}
	if result.SignatureVerified {
		t.Error("expected SignatureVerified=false when skipped")
	}
}

func TestDownloadWithSignature_VerifiedSignature(t *testing.T) {
	// The minisign fixture in update_test.go signs exactly testMinisignContent.
	// Serve that as the checksums file and its real signature.
	assetName := "test-file"

	mux := http.NewServeMux()
	mux.HandleFunc("/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(testMinisignContent))
	})
	mux.HandleFunc("/checksums.txt.minisig", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(testMinisignSignature))
	})
	// The checksum recorded in testMinisignContent is "abc123def456" which won't
	// match a real binary, so the binary download will fail at the checksum step.
	// We only assert that the signature path is exercised and verified.
	mux.HandleFunc("/binary", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("whatever"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	d := NewDownloader()
	d.SetStageDir(t.TempDir())

	_, result, err := d.DownloadWithSignature(
		context.Background(),
		srv.URL+"/binary", srv.URL+"/checksums.txt", srv.URL+"/checksums.txt.minisig",
		assetName, testMinisignPublicKey, true,
	)
	// Signature verifies, but the binary's checksum ("abc123def456") won't match.
	if !errors.Is(err, ErrChecksumFailed) {
		t.Fatalf("DownloadWithSignature() error = %v, want ErrChecksumFailed", err)
	}
	if !result.SignatureVerified {
		t.Error("expected SignatureVerified=true with valid signature")
	}
}

func TestDownloadWithSignature_TamperedChecksumsFails(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		// Content does NOT match the signed content -> signature must fail.
		_, _ = w.Write([]byte("tampered  test-file\n"))
	})
	mux.HandleFunc("/checksums.txt.minisig", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(testMinisignSignature))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	d := NewDownloader()
	d.SetStageDir(t.TempDir())

	_, _, err := d.DownloadWithSignature(
		context.Background(),
		srv.URL+"/binary", srv.URL+"/checksums.txt", srv.URL+"/checksums.txt.minisig",
		"test-file", testMinisignPublicKey, true,
	)
	if !errors.Is(err, ErrSignatureVerificationFailed) {
		t.Fatalf("DownloadWithSignature() error = %v, want ErrSignatureVerificationFailed", err)
	}
}

func TestDownloadWithSignature_RequiredSignatureMissing(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("abc  test-file\n"))
	})
	// No signature handler -> 404.
	srv := httptest.NewServer(mux)
	defer srv.Close()

	d := NewDownloader()
	d.SetStageDir(t.TempDir())

	_, _, err := d.DownloadWithSignature(
		context.Background(),
		srv.URL+"/binary", srv.URL+"/checksums.txt", srv.URL+"/missing.minisig",
		"test-file", testMinisignPublicKey, true,
	)
	if !errors.Is(err, ErrSignatureVerificationFailed) {
		t.Fatalf("DownloadWithSignature() error = %v, want ErrSignatureVerificationFailed", err)
	}
}

func TestDownloadWithSignature_ChecksumsURLFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	d := NewDownloader()
	d.SetStageDir(t.TempDir())

	_, _, err := d.DownloadWithSignature(
		context.Background(),
		srv.URL+"/binary", srv.URL+"/checksums.txt", srv.URL+"/sig.minisig",
		"test-file", testMinisignPublicKey, false,
	)
	if !errors.Is(err, ErrDownloadFailed) {
		t.Fatalf("DownloadWithSignature() error = %v, want ErrDownloadFailed", err)
	}
}

func TestDownloadWithSignature_AssetNotInChecksums(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("abc  some-other-asset\n"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	d := NewDownloader()
	d.SetStageDir(t.TempDir())

	_, _, err := d.DownloadWithSignature(
		context.Background(),
		srv.URL+"/binary", srv.URL+"/checksums.txt", srv.URL+"/missing.minisig",
		"kvelmo-linux-amd64", testMinisignPublicKey, false,
	)
	if !errors.Is(err, ErrChecksumFailed) {
		t.Fatalf("DownloadWithSignature() error = %v, want ErrChecksumFailed", err)
	}
}

func TestDownloadFile_SuccessAndErrors(t *testing.T) {
	body := []byte("file-payload")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ok" {
			_, _ = w.Write(body)

			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	d := NewDownloader()
	d.SetStageDir(t.TempDir())

	path, err := d.downloadFile(context.Background(), srv.URL+"/ok", "dl-*.txt")
	if err != nil {
		t.Fatalf("downloadFile() error = %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(body) {
		t.Errorf("content = %q, want %q", got, body)
	}
	_ = os.Remove(path)

	if _, err := d.downloadFile(context.Background(), srv.URL+"/missing", "dl-*.txt"); err == nil {
		t.Error("expected error for 404")
	}

	if _, err := d.downloadFile(context.Background(), "http://\x00bad", "dl-*.txt"); err == nil {
		t.Error("expected error for malformed URL")
	}
}
