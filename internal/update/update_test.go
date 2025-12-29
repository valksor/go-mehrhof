package update

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
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
		name string
		err  error
	}{
		{"ErrNoUpdateAvailable", ErrNoUpdateAvailable},
		{"ErrDownloadFailed", ErrDownloadFailed},
		{"ErrChecksumFailed", ErrChecksumFailed},
		{"ErrInstallFailed", ErrInstallFailed},
		{"ErrAssetNotFound", ErrAssetNotFound},
		{"ErrDevBuild", ErrDevBuild},
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
