package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
)

// Downloader downloads release binaries and verifies checksums.
type Downloader struct {
	client *http.Client
}

// NewDownloader creates a new downloader.
func NewDownloader() *Downloader {
	return &Downloader{
		client: &http.Client{},
	}
}

// Download downloads the binary from the given URL to a temporary file.
// If expectedChecksum is non-empty, it will be verified after download.
// Returns the path to the downloaded file.
func (d *Downloader) Download(ctx context.Context, url, expectedChecksum string) (string, error) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "mehrhof-update-*.bin")
	if err != nil {
		return "", fmt.Errorf("%w: create temp file: %w", ErrDownloadFailed, err)
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = tmpFile.Close() }()

	// Download the file
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		_ = os.Remove(tmpPath)

		return "", fmt.Errorf("%w: create request: %w", ErrDownloadFailed, err)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		_ = os.Remove(tmpPath)

		return "", fmt.Errorf("%w: %w", ErrDownloadFailed, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		_ = os.Remove(tmpPath)

		return "", fmt.Errorf("%w: unexpected status: %d", ErrDownloadFailed, resp.StatusCode)
	}

	// Calculate checksum while downloading
	hasher := sha256.New()
	writer := io.MultiWriter(tmpFile, hasher)

	_, err = io.Copy(writer, resp.Body)
	if err != nil {
		_ = os.Remove(tmpPath)

		return "", fmt.Errorf("%w: %w", ErrDownloadFailed, err)
	}

	// Verify checksum if provided
	if expectedChecksum != "" {
		actualChecksum := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(actualChecksum, expectedChecksum) {
			_ = os.Remove(tmpPath)

			return "", fmt.Errorf("%w: expected %s, got %s", ErrChecksumFailed, expectedChecksum, actualChecksum)
		}
	}

	return tmpPath, nil
}

// DownloadWithChecksums downloads the binary and fetches checksums from a separate URL.
// It attempts to find the matching checksum for the asset name.
// Returns the path to the downloaded file.
func (d *Downloader) DownloadWithChecksums(ctx context.Context, binaryURL, checksumsURL, assetName string) (string, error) {
	// First, try to get the checksum
	checksum := ""
	if checksumsURL != "" {
		var err error
		checksum, err = d.fetchChecksum(ctx, checksumsURL, assetName)
		if err != nil {
			// Continue without checksum - it's optional
			checksum = ""
		}
	}

	// Download the binary
	return d.Download(ctx, binaryURL, checksum)
}

// fetchChecksum downloads and parses the checksums file to find the checksum for the given asset.
// Returns empty string if checksum is not found (graceful degradation).
func (d *Downloader) fetchChecksum(ctx context.Context, url, assetName string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return ParseChecksumsFile(string(content), assetName), nil
}

// ParseChecksumsFile parses a checksums file and returns the checksum for the given asset.
// Expected format: "checksum  filename" or "checksum *filename" (binary mode)
// Returns empty string if not found.
func ParseChecksumsFile(content, assetName string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Split by whitespace
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		// Second part is the filename (possibly with * prefix for binary mode)
		filename := strings.TrimPrefix(parts[1], "*")
		if filename == assetName {
			return parts[0] // First part is the checksum
		}
	}

	return ""
}

// VerifyChecksum verifies a downloaded file against an expected checksum.
// If expectedChecksum is empty, returns nil (checksums are optional).
func VerifyChecksum(filePath, expectedChecksum string) error {
	if expectedChecksum == "" {
		return nil // Checksums are optional
	}

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer func() { _ = f.Close() }()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return fmt.Errorf("calculate checksum: %w", err)
	}

	actualChecksum := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(actualChecksum, expectedChecksum) {
		return fmt.Errorf("%w: expected %s, got %s", ErrChecksumFailed, expectedChecksum, actualChecksum)
	}

	return nil
}

// CalculateChecksum calculates the SHA256 checksum of a file.
func CalculateChecksum(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer func() { _ = f.Close() }()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", fmt.Errorf("calculate checksum: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// DownloadChecksumsFile downloads the checksums file from a URL.
// Returns the path to the downloaded file.
func (d *Downloader) DownloadChecksumsFile(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	// Create temp file for checksums
	tmpFile, err := os.CreateTemp("", "mehrhof-checksums-*.txt")
	if err != nil {
		return "", err
	}
	defer func() { _ = tmpFile.Close() }()

	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		_ = os.Remove(tmpFile.Name())

		return "", err
	}

	return tmpFile.Name(), nil
}

// FindChecksumInFile searches for the checksum of a specific asset in a checksums file.
func FindChecksumInFile(checksumsPath, assetName string) (string, error) {
	content, err := os.ReadFile(checksumsPath)
	if err != nil {
		return "", err
	}

	checksum := ParseChecksumsFile(string(content), assetName)
	if checksum == "" {
		return "", fmt.Errorf("checksum not found for %s", assetName)
	}

	return checksum, nil
}

// GetAssetName returns the expected asset name for the current platform.
func GetAssetName() string {
	return fmt.Sprintf("mehr-%s-%s", runtime.GOOS, runtime.GOARCH)
}
