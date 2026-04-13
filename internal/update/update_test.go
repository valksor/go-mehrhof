package update

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v67/github"
)

func TestParseChecksumsFile(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		assetName string
		want      string
	}{
		{
			name: "valid checksum with text mode",
			content: "a1b2c3d4e5f6  kvelmo-linux-amd64\n" +
				"z9y8x7w6v5u4  kvelmo-darwin-arm64\n",
			assetName: "kvelmo-linux-amd64",
			want:      "a1b2c3d4e5f6",
		},
		{
			name: "valid checksum with binary mode",
			content: "a1b2c3d4e5f6 *kvelmo-linux-amd64\n" +
				"z9y8x7w6v5u4 *kvelmo-darwin-arm64\n",
			assetName: "kvelmo-linux-amd64",
			want:      "a1b2c3d4e5f6",
		},
		{
			name:      "asset not found",
			content:   "a1b2c3d4e5f6  other-file\n",
			assetName: "kvelmo-linux-amd64",
			want:      "",
		},
		{
			name:      "empty content",
			content:   "",
			assetName: "kvelmo-linux-amd64",
			want:      "",
		},
		{
			name:      "tabs instead of spaces",
			content:   "a1b2c3d4e5f6\tkvelmo-linux-amd64\n",
			assetName: "kvelmo-linux-amd64",
			want:      "a1b2c3d4e5f6",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseChecksumsFile(tt.content, tt.assetName)
			if got != tt.want {
				t.Errorf("ParseChecksumsFile() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetAssetName(t *testing.T) {
	got := GetAssetName()
	want := "kvelmo-" + runtime.GOOS + "-" + runtime.GOARCH
	if got != want {
		t.Errorf("GetAssetName() = %q, want %q", got, want)
	}
}

func TestCalculateChecksum(t *testing.T) {
	tmpFile, err := os.CreateTemp(t.TempDir(), "checksum-test-*.txt")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := tmpFile.WriteString("hello world"); err != nil {
		t.Fatal(err)
	}
	_ = tmpFile.Close()

	got, err := CalculateChecksum(tmpFile.Name())
	if err != nil {
		t.Fatalf("CalculateChecksum() error = %v", err)
	}

	if len(got) != 64 {
		t.Errorf("CalculateChecksum() returned length %d, want 64", len(got))
	}

	for _, c := range got {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			t.Errorf("CalculateChecksum() returned invalid hex: %q", got)

			break
		}
	}
}

func TestCalculateChecksumNonExistent(t *testing.T) {
	_, err := CalculateChecksum("/non/existent/file.txt")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestVerifyChecksum(t *testing.T) {
	tmpFile, err := os.CreateTemp(t.TempDir(), "verify-test-*.txt")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := tmpFile.WriteString("test content"); err != nil {
		t.Fatal(err)
	}
	_ = tmpFile.Close()

	correctChecksum, err := CalculateChecksum(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		checksum string
		wantErr  bool
	}{
		{"correct checksum", correctChecksum, false},
		{"incorrect checksum", "wrong" + correctChecksum, true},
		{"empty checksum (optional)", "", false},
		{"case insensitive match", strings.ToUpper(correctChecksum), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyChecksum(tmpFile.Name(), tt.checksum)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyChecksum() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestVersionNewer(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{"a is newer major", "2.0.0", "1.0.0", true},
		{"a is newer minor", "1.2.0", "1.1.0", true},
		{"a is newer patch", "1.1.1", "1.1.0", true},
		{"same version", "1.0.0", "1.0.0", false},
		{"b is newer", "1.0.0", "1.0.1", false},
		{"with v prefix - a newer", "v2.0.0", "v1.0.0", true},
		{"with v prefix - equal", "v1.0.0", "v1.0.0", false},
		{"pre-release: alpha is older", "1.0.0", "1.0.0-alpha", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := versionNewer(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("versionNewer(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestInstaller_IsWritable(t *testing.T) {
	installer := NewInstaller()

	writable, err := installer.IsWritable()
	if err != nil {
		t.Fatalf("IsWritable() error = %v", err)
	}

	if !writable {
		t.Skip("binary directory not writable, skipping writability test")
	}
}

func TestBinaryPath(t *testing.T) {
	path, err := BinaryPath()
	if err != nil {
		t.Fatalf("BinaryPath() error = %v", err)
	}
	if path == "" {
		t.Error("BinaryPath() returned empty string")
	}
	if !filepath.IsAbs(path) {
		t.Errorf("BinaryPath() = %q, want absolute path", path)
	}
}

func TestNewChecker(t *testing.T) {
	checker := NewChecker("", "valksor", "kvelmo")
	if checker == nil {
		t.Fatal("NewChecker() returned nil")
	}

	checker = NewChecker("", "", "")
	if checker == nil {
		t.Fatal("NewChecker() with empty strings returned nil")
	}
}

func TestCheckerCheck_DevBuildCanUpdateToLatestStable(t *testing.T) {
	assetName := "kvelmo-" + runtime.GOOS + "-" + runtime.GOARCH
	checker, closeFn := newTestChecker(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/valksor/kvelmo/releases" {
			http.NotFound(w, r)

			return
		}

		releases := []map[string]any{
			{
				"tag_name":   "v1.2.3",
				"draft":      false,
				"prerelease": false,
				"html_url":   "https://example.test/releases/v1.2.3",
				"assets": []map[string]any{
					{
						"name":                 assetName,
						"browser_download_url": "https://example.test/download/" + assetName,
						"size":                 12345,
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(releases)
	})
	defer closeFn()

	status, err := checker.Check(context.Background(), CheckOptions{
		CurrentVersion: "dev",
	})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if status == nil {
		t.Fatal("Check() returned nil status")
	}
	if status.LatestVersion != "v1.2.3" {
		t.Fatalf("LatestVersion = %q, want %q", status.LatestVersion, "v1.2.3")
	}
}

func TestCheckerCheck_TargetTagNightly(t *testing.T) {
	assetName := "kvelmo-" + runtime.GOOS + "-" + runtime.GOARCH
	checker, closeFn := newTestChecker(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/valksor/kvelmo/releases/tags/nightly" {
			http.NotFound(w, r)

			return
		}

		release := map[string]any{
			"tag_name":   "nightly",
			"draft":      false,
			"prerelease": true,
			"html_url":   "https://example.test/releases/nightly",
			"assets": []map[string]any{
				{
					"name":                 assetName,
					"browser_download_url": "https://example.test/download/" + assetName,
					"size":                 54321,
				},
			},
		}
		_ = json.NewEncoder(w).Encode(release)
	})
	defer closeFn()

	status, err := checker.Check(context.Background(), CheckOptions{
		CurrentVersion: "v1.0.0",
		TargetTag:      "nightly",
	})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if status == nil {
		t.Fatal("Check() returned nil status")
	}
	if status.LatestVersion != "nightly" {
		t.Fatalf("LatestVersion = %q, want %q", status.LatestVersion, "nightly")
	}
}

func TestCheckerCheck_TargetTagNotFound(t *testing.T) {
	checker, closeFn := newTestChecker(t, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	defer closeFn()

	_, err := checker.Check(context.Background(), CheckOptions{
		CurrentVersion: "v1.0.0",
		TargetTag:      "v9.9.9",
	})
	if !errors.Is(err, ErrReleaseNotFound) {
		t.Fatalf("Check() error = %v, want ErrReleaseNotFound", err)
	}
}

func TestFindChecksumInFile(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		assetName string
		want      string
		wantErr   bool
	}{
		{
			name: "valid checksum found",
			content: "a1b2c3d4e5f6  kvelmo-linux-amd64\n" +
				"z9y8x7w6v5u4  kvelmo-darwin-arm64\n",
			assetName: "kvelmo-linux-amd64",
			want:      "a1b2c3d4e5f6",
		},
		{
			name:      "checksum not found",
			content:   "a1b2c3d4e5f6  other-file\n",
			assetName: "kvelmo-linux-amd64",
			wantErr:   true,
		},
		{
			name:      "non-existent file",
			assetName: "kvelmo-linux-amd64",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "non-existent file" {
				_, err := FindChecksumInFile("/non/existent/path/checksums.txt", tt.assetName)
				if err == nil {
					t.Error("Expected error for non-existent file")
				}

				return
			}

			tmpFile, err := os.CreateTemp(t.TempDir(), "checksums-*.txt")
			if err != nil {
				t.Fatal(err)
			}

			if _, err := tmpFile.WriteString(tt.content); err != nil {
				t.Fatal(err)
			}
			_ = tmpFile.Close()

			got, err := FindChecksumInFile(tmpFile.Name(), tt.assetName)
			if (err != nil) != tt.wantErr {
				t.Errorf("FindChecksumInFile() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if got != tt.want {
				t.Errorf("FindChecksumInFile() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReleaseInfoFromGitHub(t *testing.T) {
	publishedAt := time.Date(2024, 1, 16, 12, 0, 0, 0, time.UTC)

	gh := &github.RepositoryRelease{
		TagName:     new("v1.2.3"),
		Name:        new("Release 1.2.3"),
		Body:        new("Release notes"),
		Prerelease:  new(false),
		PublishedAt: &github.Timestamp{Time: publishedAt},
		HTMLURL:     new("https://github.com/owner/repo/releases/tag/v1.2.3"),
		Assets: []*github.ReleaseAsset{
			{
				Name:               new("kvelmo-linux-amd64"),
				BrowserDownloadURL: new("https://example.com/linux-amd64"),
				Size:               new(1024000),
			},
		},
	}

	got := ReleaseInfoFromGitHub(gh)
	if got.TagName != "v1.2.3" {
		t.Errorf("TagName = %q, want %q", got.TagName, "v1.2.3")
	}
	if !got.PublishedAt.Equal(publishedAt) {
		t.Errorf("PublishedAt = %v, want %v", got.PublishedAt, publishedAt)
	}
	if len(got.Assets) != 1 {
		t.Fatalf("Assets length = %d, want 1", len(got.Assets))
	}
	if got.Assets[0].Name != "kvelmo-linux-amd64" {
		t.Errorf("Assets[0].Name = %q, want %q", got.Assets[0].Name, "kvelmo-linux-amd64")
	}
}

// Test fixtures for Minisign verification.
// Generated using: minisign -G -f -p test-public.key -s test-secret.key -W.
const (
	testMinisignPublicKey = "RWRf78Prk79PdQ9cTZM4m2hijMBOktAMz2k8CpNqyq54ClqZacWoJ0Pm"
	testMinisignContent   = "abc123def456  test-file\n"
	testMinisignSignature = `untrusted comment: signature from minisign secret key
RURf78Prk79PdbtjQ7d6JnyuuVPL1xHlaTiOFX10gWJNnBpihQd6ZAThBj03BbYYdd+DNfMgkTNmIvxawymW2F+hxzPdgJSvMAA=
trusted comment: timestamp:1770293603	file:test-checksums.txt	hashed
3FldcmxwJc2CBA9tnXI0OhXpO627xfBzbowbRD3jyf1D4ezZCVUOtDJewR01ZkFbsazqgHiEWuSVAAcxic6SBg==`
)

func TestVerifyMinisign(t *testing.T) {
	tests := []struct {
		name      string
		content   []byte
		signature []byte
		publicKey string
		wantErr   bool
	}{
		{
			name:      "valid signature",
			content:   []byte(testMinisignContent),
			signature: []byte(testMinisignSignature),
			publicKey: testMinisignPublicKey,
			wantErr:   false,
		},
		{
			name:      "invalid signature - wrong content",
			content:   []byte("different content"),
			signature: []byte(testMinisignSignature),
			publicKey: testMinisignPublicKey,
			wantErr:   true,
		},
		{
			name:      "invalid signature - wrong public key",
			content:   []byte(testMinisignContent),
			signature: []byte(testMinisignSignature),
			publicKey: MinisignPublicKey, // Production key, not test key
			wantErr:   true,
		},
		{
			name:      "invalid public key",
			content:   []byte(testMinisignContent),
			signature: []byte(testMinisignSignature),
			publicKey: "invalid-key",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyMinisign(tt.content, tt.signature, tt.publicKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyMinisign() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestVerifyMinisignFile(t *testing.T) {
	tmpDir := t.TempDir()

	contentPath := filepath.Join(tmpDir, "checksums.txt")
	sigPath := filepath.Join(tmpDir, "checksums.txt.minisig")

	if err := os.WriteFile(contentPath, []byte(testMinisignContent), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sigPath, []byte(testMinisignSignature), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		contentPath string
		sigPath     string
		publicKey   string
		wantErr     bool
	}{
		{"valid files", contentPath, sigPath, testMinisignPublicKey, false},
		{"content file not found", "/nonexistent/file", sigPath, testMinisignPublicKey, true},
		{"signature file not found", contentPath, "/nonexistent/file", testMinisignPublicKey, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyMinisignFile(tt.contentPath, tt.sigPath, tt.publicKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyMinisignFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMinisignPublicKeyConstant(t *testing.T) {
	if len(MinisignPublicKey) == 0 {
		t.Error("MinisignPublicKey constant is empty")
	}

	if len(MinisignPublicKey) != 56 {
		t.Errorf("MinisignPublicKey length = %d, want 56", len(MinisignPublicKey))
	}

	if !strings.HasPrefix(MinisignPublicKey, "RW") {
		t.Errorf("MinisignPublicKey should start with 'RW', got %q", MinisignPublicKey[:2])
	}
}

func TestUpdateErrors(t *testing.T) {
	errs := []error{
		ErrNoUpdateAvailable,
		ErrDownloadFailed,
		ErrChecksumFailed,
		ErrInstallFailed,
		ErrAssetNotFound,
		ErrSignatureVerificationFailed,
		ErrReleaseNotFound,
	}

	for _, err := range errs {
		if err == nil {
			t.Error("Expected non-nil error")
		}
	}
}

func newTestChecker(t *testing.T, handler http.HandlerFunc) (*Checker, func()) {
	t.Helper()

	server := httptest.NewServer(handler)
	checker := NewChecker("", "valksor", "kvelmo")
	baseURL, err := url.Parse(server.URL + "/")
	if err != nil {
		server.Close()
		t.Fatalf("parse test server URL: %v", err)
	}
	checker.ghClient.BaseURL = baseURL
	checker.ghClient.UploadURL = baseURL

	return checker, func() {
		server.Close()
	}
}
