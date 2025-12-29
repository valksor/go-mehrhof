package update

import (
	"context"
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
			content: "a1b2c3d4e5f6  mehrhof-linux-amd64\n" +
				"z9y8x7w6v5u4  mehrhof-darwin-arm64\n",
			assetName: "mehrhof-linux-amd64",
			want:      "a1b2c3d4e5f6",
		},
		{
			name: "valid checksum with binary mode",
			content: "a1b2c3d4e5f6 *mehrhof-linux-amd64\n" +
				"z9y8x7w6v5u4 *mehrhof-darwin-arm64\n",
			assetName: "mehrhof-linux-amd64",
			want:      "a1b2c3d4e5f6",
		},
		{
			name:      "asset not found",
			content:   "a1b2c3d4e5f6  other-file\n",
			assetName: "mehrhof-linux-amd64",
			want:      "",
		},
		{
			name:      "empty content",
			content:   "",
			assetName: "mehrhof-linux-amd64",
			want:      "",
		},
		{
			name:      "multiple spaces between checksum and file",
			content:   "a1b2c3d4e5f6    mehrhof-linux-amd64\n",
			assetName: "mehrhof-linux-amd64",
			want:      "a1b2c3d4e5f6",
		},
		{
			name:      "tabs instead of spaces",
			content:   "a1b2c3d4e5f6\tmehrhof-linux-amd64\n",
			assetName: "mehrhof-linux-amd64",
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
	want := "mehrhof-" + runtime.GOOS + "-" + runtime.GOARCH
	if got != want {
		t.Errorf("GetAssetName() = %q, want %q", got, want)
	}
}

func TestCalculateChecksum(t *testing.T) {
	// Create a temporary file with known content
	tmpFile, err := os.CreateTemp("", "checksum-test-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	// Write known content
	knownContent := "hello world"
	if _, err := tmpFile.WriteString(knownContent); err != nil {
		t.Fatal(err)
	}
	_ = tmpFile.Close()

	// Calculate checksum - SHA256 of "hello world"
	// This is the known SHA256 hash
	got, err := CalculateChecksum(tmpFile.Name())
	if err != nil {
		t.Fatalf("CalculateChecksum() error = %v", err)
	}

	// Verify it's a 64-character hex string (SHA256 format)
	if len(got) != 64 {
		t.Errorf("CalculateChecksum() returned length %d, want 64", len(got))
	}

	// Verify it's valid hex (De Morgan's law applied)
	for _, c := range got {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			t.Errorf("CalculateChecksum() returned invalid hex: %q", got)
			break
		}
	}
}

func TestCalculateChecksumErrors(t *testing.T) {
	tests := []struct {
		name        string
		filePath    string
		wantErr     bool
		errContains string
	}{
		{
			name:        "non-existent file",
			filePath:    "/non/existent/file.txt",
			wantErr:     true,
			errContains: "open file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := CalculateChecksum(tt.filePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("CalculateChecksum() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errContains != "" {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Error should contain %q, got %q", tt.errContains, err.Error())
				}
			}
		})
	}
}

func TestVerifyChecksum(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "verify-test-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	knownContent := "test content"
	if _, err := tmpFile.WriteString(knownContent); err != nil {
		t.Fatal(err)
	}
	_ = tmpFile.Close()

	// Calculate correct checksum
	correctChecksum, err := CalculateChecksum(tmpPath)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		filePath string
		checksum string
		wantErr  bool
	}{
		{
			name:     "correct checksum",
			filePath: tmpPath,
			checksum: correctChecksum,
			wantErr:  false,
		},
		{
			name:     "incorrect checksum",
			filePath: tmpPath,
			checksum: "wrong" + correctChecksum,
			wantErr:  true,
		},
		{
			name:     "empty checksum (optional)",
			filePath: tmpPath,
			checksum: "",
			wantErr:  false,
		},
		{
			name:     "case insensitive match",
			filePath: tmpPath,
			checksum: strings.ToUpper(correctChecksum),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyChecksum(tt.filePath, tt.checksum)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyChecksum() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestVerifyChecksumErrors(t *testing.T) {
	tests := []struct {
		name        string
		filePath    string
		checksum    string
		wantErr     bool
		errContains string
	}{
		{
			name:        "non-existent file",
			filePath:    "/non/existent/file.txt",
			checksum:    "abc123",
			wantErr:     true,
			errContains: "open file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyChecksum(tt.filePath, tt.checksum)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyChecksum() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errContains != "" {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Error should contain %q, got %q", tt.errContains, err.Error())
				}
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
		// Basic comparisons
		{"a is newer major", "2.0.0", "1.0.0", true},
		{"a is newer minor", "1.2.0", "1.1.0", true},
		{"a is newer patch", "1.1.1", "1.1.0", true},
		{"same version", "1.0.0", "1.0.0", false},
		{"b is newer", "1.0.0", "1.0.1", false},
		{"b is newer major", "1.0.0", "2.0.0", false},
		{"a is older", "1.0.0", "1.1.0", false},

		// With 'v' prefix (as used by semver.Compare)
		{"with v prefix - a newer", "v2.0.0", "v1.0.0", true},
		{"with v prefix - equal", "v1.0.0", "v1.0.0", false},
		{"with v prefix - b newer", "v1.0.0", "v2.0.0", false},

		// Pre-release versions (semver lib handles these correctly)
		{"pre-release: alpha is older", "1.0.0", "1.0.0-alpha", true},
		{"pre-release: beta is older", "1.0.0", "1.0.0-beta", true},
		{"pre-release: rc is older", "1.0.0", "1.0.0-rc.1", true},

		// Versions with more than 3 parts (build metadata)
		{"build metadata ignored", "1.0.0+build1", "1.0.0+build2", false},
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

	// Test that we can check writability (should be writable for temp dir)
	writable, err := installer.IsWritable()
	if err != nil {
		t.Fatalf("IsWritable() error = %v", err)
	}

	// In most environments, the current binary's directory should be writable
	// or at least the check should complete without error
	if !writable {
		// This might fail in some test environments, so we just note it
		t.Skip("binary directory not writable, skipping writability test")
	}
}

func TestDownloader_Download(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping download test in short mode")
	}

	// We can't easily test actual downloads without a real server,
	// but we can test that a proper error is returned for invalid URLs
	d := NewDownloader()

	ctx := context.Background()

	// Test with invalid URL
	_, err := d.Download(ctx, "http://invalid.example.local/file", "")
	if err == nil {
		t.Error("Expected error for invalid URL, got nil")
	}
}

func TestNewChecker(t *testing.T) {
	// Test that checker can be created
	checker := NewChecker("", "valksor", "go-mehrhof")
	if checker == nil {
		t.Fatal("NewChecker() returned nil")
	}

	// Test with empty owner/repo (should use defaults)
	checker = NewChecker("", "", "")
	if checker == nil {
		t.Fatal("NewChecker() with empty strings returned nil")
	}
}

func TestUpdateErrors(t *testing.T) {
	tests := []struct {
		err  error
		name string
	}{
		{ErrNoUpdateAvailable, "ErrNoUpdateAvailable"},
		{ErrDownloadFailed, "ErrDownloadFailed"},
		{ErrChecksumFailed, "ErrChecksumFailed"},
		{ErrInstallFailed, "ErrInstallFailed"},
		{ErrAssetNotFound, "ErrAssetNotFound"},
		{ErrDevBuild, "ErrDevBuild"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Error("Expected non-nil error")
			}
		})
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
	// The path should be an absolute path
	if !filepath.IsAbs(path) {
		t.Errorf("BinaryPath() = %q, want absolute path", path)
	}
}

func TestFindChecksumInFile(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		assetName   string
		want        string
		wantErr     bool
		errContains string
	}{
		{
			name: "valid checksum found",
			content: "a1b2c3d4e5f6  mehrhof-linux-amd64\n" +
				"z9y8x7w6v5u4  mehrhof-darwin-arm64\n",
			assetName: "mehrhof-linux-amd64",
			want:      "a1b2c3d4e5f6",
			wantErr:   false,
		},
		{
			name: "valid checksum with binary mode",
			content: "a1b2c3d4e5f6 *mehrhof-linux-amd64\n" +
				"z9y8x7w6v5u4 *mehrhof-darwin-arm64\n",
			assetName: "mehrhof-linux-amd64",
			want:      "a1b2c3d4e5f6",
			wantErr:   false,
		},
		{
			name:        "checksum not found",
			content:     "a1b2c3d4e5f6  other-file\n",
			assetName:   "mehrhof-linux-amd64",
			want:        "",
			wantErr:     true,
			errContains: "checksum not found",
		},
		{
			name:        "empty file",
			content:     "",
			assetName:   "mehrhof-linux-amd64",
			want:        "",
			wantErr:     true,
			errContains: "checksum not found",
		},
		{
			name: "multiple entries finds correct one",
			content: "abc123  file1\n" +
				"def456  mehrhof-darwin-arm64\n" +
				"ghi789  file3\n",
			assetName: "mehrhof-darwin-arm64",
			want:      "def456",
			wantErr:   false,
		},
		{
			name:      "file read error - non-existent file",
			content:   "", // Not used for non-existent file test
			assetName: "mehrhof-linux-amd64",
			want:      "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "file read error - non-existent file" {
				// Test with non-existent file
				_, err := FindChecksumInFile("/non/existent/path/checksums.txt", tt.assetName)
				if err == nil {
					t.Error("Expected error for non-existent file, got nil")
				}
				return
			}

			// Create a temporary file with the test content
			tmpFile, err := os.CreateTemp("", "checksums-*.txt")
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = os.Remove(tmpFile.Name()) }()

			if _, err := tmpFile.WriteString(tt.content); err != nil {
				t.Fatal(err)
			}
			_ = tmpFile.Close()

			got, err := FindChecksumInFile(tmpFile.Name(), tt.assetName)
			if (err != nil) != tt.wantErr {
				t.Errorf("FindChecksumInFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errContains != "" {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Error should contain %q, got %q", tt.errContains, err.Error())
				}
			}
			if got != tt.want {
				t.Errorf("FindChecksumInFile() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReleaseInfoFromGitHub(t *testing.T) {
	publishedAt := time.Date(2024, 1, 16, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		gh   *github.RepositoryRelease
		want *ReleaseInfo
	}{
		{
			name: "full release with assets",
			gh: &github.RepositoryRelease{
				TagName:     github.String("v1.2.3"),
				Name:        github.String("Release 1.2.3"),
				Body:        github.String("Release notes here"),
				Prerelease:  github.Bool(false),
				PublishedAt: &github.Timestamp{Time: publishedAt},
				HTMLURL:     github.String("https://github.com/owner/repo/releases/tag/v1.2.3"),
				Assets: []*github.ReleaseAsset{
					{
						Name:               github.String("mehrhof-linux-amd64"),
						BrowserDownloadURL: github.String("https://example.com/linux-amd64"),
						Size:               github.Int(1024000),
					},
					{
						Name:               github.String("mehrhof-darwin-arm64"),
						BrowserDownloadURL: github.String("https://example.com/darwin-arm64"),
						Size:               github.Int(980000),
					},
				},
			},
			want: &ReleaseInfo{
				TagName:     "v1.2.3",
				Name:        "Release 1.2.3",
				Body:        "Release notes here",
				PreRelease:  false,
				PublishedAt: publishedAt,
				HTMLURL:     "https://github.com/owner/repo/releases/tag/v1.2.3",
				Assets: []Asset{
					{Name: "mehrhof-linux-amd64", URL: "https://example.com/linux-amd64", Size: 1024000},
					{Name: "mehrhof-darwin-arm64", URL: "https://example.com/darwin-arm64", Size: 980000},
				},
			},
		},
		{
			name: "pre-release",
			gh: &github.RepositoryRelease{
				TagName:    github.String("v2.0.0-beta.1"),
				Name:       github.String("Beta 1"),
				Body:       github.String("Beta release notes"),
				Prerelease: github.Bool(true),
				HTMLURL:    github.String("https://github.com/owner/repo/releases/tag/v2.0.0-beta.1"),
				Assets:     []*github.ReleaseAsset{},
			},
			want: &ReleaseInfo{
				TagName:     "v2.0.0-beta.1",
				Name:        "Beta 1",
				Body:        "Beta release notes",
				PreRelease:  true,
				PublishedAt: time.Time{}, // Zero time when nil
				HTMLURL:     "https://github.com/owner/repo/releases/tag/v2.0.0-beta.1",
				Assets:      []Asset{},
			},
		},
		{
			name: "release with nil published at",
			gh: &github.RepositoryRelease{
				TagName:     github.String("v1.0.0"),
				Name:        github.String("v1.0.0"),
				Prerelease:  github.Bool(false),
				PublishedAt: nil, // Nil timestamp
				Assets: []*github.ReleaseAsset{
					{
						Name:               github.String("checksums.txt"),
						BrowserDownloadURL: github.String("https://example.com/checksums.txt"),
						Size:               github.Int(256),
					},
				},
			},
			want: &ReleaseInfo{
				TagName:     "v1.0.0",
				Name:        "v1.0.0",
				Body:        "", // Nil body returns empty string
				PreRelease:  false,
				PublishedAt: time.Time{},
				Assets: []Asset{
					{Name: "checksums.txt", URL: "https://example.com/checksums.txt", Size: 256},
				},
			},
		},
		{
			name: "minimal release",
			gh: &github.RepositoryRelease{
				TagName: github.String("v0.1.0"),
				Assets:  []*github.ReleaseAsset{},
			},
			want: &ReleaseInfo{
				TagName:     "v0.1.0",
				Name:        "", // Nil name returns empty string
				Body:        "",
				PreRelease:  false, // Nil bool defaults to false
				PublishedAt: time.Time{},
				HTMLURL:     "", // Nil URL returns empty string
				Assets:      []Asset{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ReleaseInfoFromGitHub(tt.gh)
			if got == nil {
				t.Fatal("ReleaseInfoFromGitHub() returned nil")
			}

			// Compare each field
			if got.TagName != tt.want.TagName {
				t.Errorf("TagName = %q, want %q", got.TagName, tt.want.TagName)
			}
			if got.Name != tt.want.Name {
				t.Errorf("Name = %q, want %q", got.Name, tt.want.Name)
			}
			if got.Body != tt.want.Body {
				t.Errorf("Body = %q, want %q", got.Body, tt.want.Body)
			}
			if got.PreRelease != tt.want.PreRelease {
				t.Errorf("PreRelease = %v, want %v", got.PreRelease, tt.want.PreRelease)
			}
			if !got.PublishedAt.Equal(tt.want.PublishedAt) {
				t.Errorf("PublishedAt = %v, want %v", got.PublishedAt, tt.want.PublishedAt)
			}
			if got.HTMLURL != tt.want.HTMLURL {
				t.Errorf("HTMLURL = %q, want %q", got.HTMLURL, tt.want.HTMLURL)
			}
			if len(got.Assets) != len(tt.want.Assets) {
				t.Errorf("Assets length = %d, want %d", len(got.Assets), len(tt.want.Assets))
			} else {
				for i := range got.Assets {
					if got.Assets[i].Name != tt.want.Assets[i].Name {
						t.Errorf("Assets[%d].Name = %q, want %q", i, got.Assets[i].Name, tt.want.Assets[i].Name)
					}
					if got.Assets[i].URL != tt.want.Assets[i].URL {
						t.Errorf("Assets[%d].URL = %q, want %q", i, got.Assets[i].URL, tt.want.Assets[i].URL)
					}
					if got.Assets[i].Size != tt.want.Assets[i].Size {
						t.Errorf("Assets[%d].Size = %d, want %d", i, got.Assets[i].Size, tt.want.Assets[i].Size)
					}
				}
			}
		})
	}
}
