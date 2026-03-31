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

	"github.com/jedisct1/go-minisign"
)

// Downloader downloads release binaries and verifies checksums.
type Downloader struct {
	client   *http.Client
	stageDir string // Directory for temp files; empty uses OS default.
}

// NewDownloader creates a new downloader.
func NewDownloader() *Downloader {
	return &Downloader{
		client: &http.Client{},
	}
}

// SetStageDir sets the directory for temporary download files.
// Use this to ensure downloads land on the same filesystem as the target binary,
// which is required for atomic os.Rename.
func (d *Downloader) SetStageDir(dir string) {
	d.stageDir = dir
}

// Download downloads the binary from the given URL to a temporary file.
// If expectedChecksum is non-empty, it will be verified after download.
func (d *Downloader) Download(ctx context.Context, url, expectedChecksum string) (string, error) {
	tmpFile, err := os.CreateTemp(d.stageDir, "kvelmo-update-*.bin")
	if err != nil {
		return "", fmt.Errorf("%w: create temp file: %w", ErrDownloadFailed, err)
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = tmpFile.Close() }()

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

	hasher := sha256.New()
	writer := io.MultiWriter(tmpFile, hasher)

	_, err = io.Copy(writer, resp.Body)
	if err != nil {
		_ = os.Remove(tmpPath)

		return "", fmt.Errorf("%w: %w", ErrDownloadFailed, err)
	}

	if expectedChecksum != "" {
		actualChecksum := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(actualChecksum, expectedChecksum) {
			_ = os.Remove(tmpPath)

			return "", fmt.Errorf("%w: expected %s, got %s", ErrChecksumFailed, expectedChecksum, actualChecksum)
		}
	}

	return tmpPath, nil
}

// VerificationResult contains the results of signature and checksum verification.
type VerificationResult struct {
	SignatureVerified bool
	SignatureSkipped  bool
	SignatureError    string
	ChecksumVerified  bool
}

// DownloadWithSignature downloads the binary with full signature and checksum verification.
// Flow: download checksums.txt + .minisig → verify signature → parse checksum → download binary → verify checksum.
//
// When requireSignature is true, a missing or unavailable signature file is treated as a hard
// error — the binary will not be downloaded. When false, the download proceeds with checksum-only
// verification and the caller can inspect VerificationResult.SignatureSkipped.
func (d *Downloader) DownloadWithSignature(
	ctx context.Context,
	binaryURL, checksumsURL, signatureURL, assetName, publicKey string,
	requireSignature bool,
) (string, *VerificationResult, error) {
	result := &VerificationResult{}

	checksumsPath, err := d.downloadFile(ctx, checksumsURL, "kvelmo-checksums-*.txt")
	if err != nil {
		return "", result, fmt.Errorf("%w: could not download checksums file: %w", ErrDownloadFailed, err)
	}
	defer func() { _ = os.Remove(checksumsPath) }()

	signaturePath, err := d.downloadFile(ctx, signatureURL, "kvelmo-sig-*.minisig")
	if err != nil {
		if requireSignature {
			return "", result, fmt.Errorf("%w: signature file not available: %w", ErrSignatureVerificationFailed, err)
		}
		result.SignatureSkipped = true
		result.SignatureError = fmt.Sprintf("signature file not available: %v", err)
	} else {
		defer func() { _ = os.Remove(signaturePath) }()

		if err := VerifyMinisignFile(checksumsPath, signaturePath, publicKey); err != nil {
			return "", result, fmt.Errorf("%w: the checksums file may have been tampered with", err)
		}
		result.SignatureVerified = true
	}

	checksum, err := FindChecksumInFile(checksumsPath, assetName)
	if err != nil {
		return "", result, fmt.Errorf("%w: %w", ErrChecksumFailed, err)
	}

	path, err := d.Download(ctx, binaryURL, checksum)
	if err != nil {
		return "", result, err
	}

	result.ChecksumVerified = true

	return path, result, nil
}

// downloadFile downloads a URL to a temporary file.
func (d *Downloader) downloadFile(ctx context.Context, url, pattern string) (string, error) {
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

	tmpFile, err := os.CreateTemp(d.stageDir, pattern)
	if err != nil {
		return "", err
	}
	defer func() { _ = tmpFile.Close() }()

	// Limit to 10 MB to prevent malicious/misconfigured servers exhausting disk.
	_, err = io.Copy(tmpFile, io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		_ = os.Remove(tmpFile.Name())

		return "", err
	}

	return tmpFile.Name(), nil
}

// ParseChecksumsFile parses a checksums file and returns the checksum for the given asset.
// Format: "checksum  filename" or "checksum *filename" (binary mode).
func ParseChecksumsFile(content, assetName string) string {
	lines := strings.SplitSeq(content, "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		filename := strings.TrimPrefix(parts[1], "*")
		if filename == assetName {
			return parts[0]
		}
	}

	return ""
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

// VerifyChecksum verifies a downloaded file against an expected checksum.
func VerifyChecksum(filePath, expectedChecksum string) error {
	if expectedChecksum == "" {
		return nil
	}

	actualChecksum, err := CalculateChecksum(filePath)
	if err != nil {
		return err
	}

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

// GetAssetName returns the expected asset name for the current platform.
func GetAssetName() string {
	return fmt.Sprintf("kvelmo-%s-%s", runtime.GOOS, runtime.GOARCH)
}

// VerifyMinisign verifies the Minisign signature of content.
func VerifyMinisign(content, signature []byte, publicKeyStr string) error {
	publicKey, err := minisign.NewPublicKey(publicKeyStr)
	if err != nil {
		return fmt.Errorf("%w: invalid public key: %w", ErrSignatureVerificationFailed, err)
	}

	sig, err := minisign.DecodeSignature(string(signature))
	if err != nil {
		return fmt.Errorf("%w: invalid signature format: %w", ErrSignatureVerificationFailed, err)
	}

	valid, err := publicKey.Verify(content, sig)
	if err != nil {
		return fmt.Errorf("%w: verification error: %w", ErrSignatureVerificationFailed, err)
	}
	if !valid {
		return fmt.Errorf("%w: signature does not match content", ErrSignatureVerificationFailed)
	}

	return nil
}

// VerifyMinisignFile verifies a file's Minisign signature.
func VerifyMinisignFile(contentPath, signaturePath, publicKeyStr string) error {
	content, err := os.ReadFile(contentPath)
	if err != nil {
		return fmt.Errorf("read content file: %w", err)
	}

	signature, err := os.ReadFile(signaturePath)
	if err != nil {
		return fmt.Errorf("read signature file: %w", err)
	}

	return VerifyMinisign(content, signature, publicKeyStr)
}
