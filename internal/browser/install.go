// Package browser provides a thin wrapper around playwright-cli for browser automation.
package browser

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	// NodeVersion is the Node.js version to download.
	NodeVersion = "22.14.0"

	// PlaywrightCLIPackage is the npm package name.
	PlaywrightCLIPackage = "@playwright/cli"

	// maxExtractBytes is the per-file extraction size limit to guard against decompression bombs.
	maxExtractBytes int64 = 512 << 20 // 512 MiB
)

// Paths returns the base directory for kvelmo browser runtime.
func Paths() string {
	home, _ := os.UserHomeDir()

	return filepath.Join(home, ".valksor", "kvelmo")
}

// RuntimeDir returns the path to the runtime directory.
func RuntimeDir() string {
	return filepath.Join(Paths(), "runtime")
}

// NodeBinaryPath returns the path to the Node.js binary.
func NodeBinaryPath() string {
	return filepath.Join(RuntimeDir(), "node")
}

// NodeModulesDir returns the path to node_modules.
func NodeModulesDir() string {
	return filepath.Join(RuntimeDir(), "node_modules")
}

// PlaywrightCLIDir returns the path to the playwright-cli package.
func PlaywrightCLIDir() string {
	return filepath.Join(NodeModulesDir(), "@playwright", "cli")
}

// BinaryPath returns the path to the playwright-cli wrapper script.
func BinaryPath() string {
	return filepath.Join(Paths(), "bin", "playwright-cli")
}

// IsInstalled checks if the runtime is fully installed.
func IsInstalled() bool {
	// Check Node.js binary
	if _, err := os.Stat(NodeBinaryPath()); err != nil {
		return false
	}

	// Check playwright-cli package (entry point is playwright-cli.js)
	cliEntry := filepath.Join(PlaywrightCLIDir(), "playwright-cli.js")
	if _, err := os.Stat(cliEntry); err != nil {
		return false
	}

	// Check wrapper script
	if _, err := os.Stat(BinaryPath()); err != nil {
		return false
	}

	return true
}

// Install downloads and installs the self-contained runtime.
func Install(ctx context.Context) error {
	slog.Info("browser: installing runtime")

	slog.Debug("browser: installing node.js", "version", NodeVersion)
	if err := installNode(ctx); err != nil {
		slog.Error("browser: node.js installation failed", "error", err)

		return fmt.Errorf("install node: %w", err)
	}

	slog.Debug("browser: installing playwright-cli")
	if err := installPlaywrightCLI(ctx); err != nil {
		slog.Error("browser: playwright-cli installation failed", "error", err)

		return fmt.Errorf("install playwright-cli: %w", err)
	}

	if err := createWrapper(); err != nil {
		return fmt.Errorf("create wrapper: %w", err)
	}

	slog.Info("browser: runtime installed successfully")

	return nil
}

// Update forces a re-download of the latest versions.
func Update(ctx context.Context) error {
	// Remove existing runtime
	if err := os.RemoveAll(RuntimeDir()); err != nil {
		return fmt.Errorf("remove runtime: %w", err)
	}

	// Remove wrapper
	if err := os.Remove(BinaryPath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove wrapper: %w", err)
	}

	return Install(ctx)
}

// Version returns the installed playwright-cli version.
func Version() (string, error) {
	if !IsInstalled() {
		return "", errors.New("playwright-cli not installed")
	}

	cmd := exec.Command(BinaryPath(), "--version") //nolint:noctx // Quick version check, no context needed
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("get version: %w", err)
	}

	return strings.TrimSpace(string(out)), nil
}

// EnsureInstalled installs the runtime if not already present.
func EnsureInstalled(ctx context.Context) error {
	if IsInstalled() {
		return nil
	}

	return Install(ctx)
}

// NpmBinaryPath returns the path to the npm script.
func NpmBinaryPath() string {
	return filepath.Join(RuntimeDir(), "lib", "node_modules", "npm", "bin", "npm-cli.js")
}

// installNode downloads and extracts Node.js with npm.
func installNode(ctx context.Context) error {
	// Determine OS and arch
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	// Map Go arch to Node.js arch
	nodeArch := goarch
	if goarch == "amd64" {
		nodeArch = "x64"
	}

	// Construct download URL
	// Example: https://nodejs.org/dist/v22.14.0/node-v22.14.0-darwin-arm64.tar.gz
	url := fmt.Sprintf(
		"https://nodejs.org/dist/v%s/node-v%s-%s-%s.tar.gz",
		NodeVersion, NodeVersion, goos, nodeArch,
	)

	fmt.Printf("Downloading Node.js %s for %s/%s...\n", NodeVersion, goos, nodeArch)

	// Download
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download node: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download node: status %d", resp.StatusCode)
	}

	// Create runtime directory
	if err := os.MkdirAll(RuntimeDir(), 0o755); err != nil {
		return fmt.Errorf("create runtime dir: %w", err)
	}

	// Extract node and npm from tarball
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	prefix := fmt.Sprintf("node-v%s-%s-%s/", NodeVersion, goos, nodeArch)

	// Files/dirs we want to extract
	extractPaths := map[string]bool{
		"bin/node": true,
		"lib/":     true, // npm lives in lib/node_modules/npm
	}

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}

		// Remove the version prefix
		if !strings.HasPrefix(hdr.Name, prefix) {
			continue
		}
		relPath := strings.TrimPrefix(hdr.Name, prefix)

		// Check if we should extract this
		shouldExtract := false
		for pattern := range extractPaths {
			if strings.HasPrefix(relPath, pattern) || relPath == strings.TrimSuffix(pattern, "/") {
				shouldExtract = true

				break
			}
		}
		if !shouldExtract {
			continue
		}

		targetPath := filepath.Join(RuntimeDir(), relPath)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return fmt.Errorf("create dir %s: %w", relPath, err)
			}

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return fmt.Errorf("create parent dir: %w", err)
			}

			mode := hdr.FileInfo().Mode() & 0o7777
			if mode == 0 {
				mode = 0o644
			}

			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return fmt.Errorf("create file %s: %w", relPath, err)
			}

			// Use maxExtractBytes+1 so lr.N==0 unambiguously means file exceeded limit
			lr := &io.LimitedReader{R: tr, N: maxExtractBytes + 1}
			if _, err := io.Copy(outFile, lr); err != nil {
				_ = outFile.Close()

				return fmt.Errorf("write file %s: %w", relPath, err)
			}

			if lr.N == 0 {
				_ = outFile.Close()
				_ = os.Remove(targetPath) // Clean up partial file

				return fmt.Errorf("file %s exceeds extraction size limit", relPath)
			}
			_ = outFile.Close()

		case tar.TypeSymlink:
			// Handle symlinks (npm uses them)
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return fmt.Errorf("create parent dir: %w", err)
			}
			_ = os.Remove(targetPath) // Remove if exists
			if err := os.Symlink(hdr.Linkname, targetPath); err != nil {
				// Ignore symlink errors on Windows
				if runtime.GOOS != "windows" {
					return fmt.Errorf("create symlink %s: %w", relPath, err)
				}
			}
		}
	}

	// Move node binary to runtime root for easier access
	nodeBin := filepath.Join(RuntimeDir(), "bin", "node")
	if _, err := os.Stat(nodeBin); err == nil {
		// Copy to runtime root
		src, err := os.Open(nodeBin)
		if err == nil {
			dst, err := os.OpenFile(NodeBinaryPath(), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
			if err == nil {
				_, _ = io.Copy(dst, src)
				_ = dst.Close()
			}
			_ = src.Close()
		}
	}

	fmt.Printf("Node.js installed to %s\n", RuntimeDir())

	return nil
}

// installPlaywrightCLI uses npm to install playwright-cli with all dependencies.
func installPlaywrightCLI(ctx context.Context) error {
	fmt.Printf("Installing %s via npm...\n", PlaywrightCLIPackage)

	// Create node_modules directory
	if err := os.MkdirAll(NodeModulesDir(), 0o755); err != nil {
		return fmt.Errorf("create node_modules: %w", err)
	}

	// Run npm install
	cmd := exec.CommandContext(
		ctx,
		NodeBinaryPath(),
		NpmBinaryPath(),
		"install",
		"--prefix", RuntimeDir(),
		PlaywrightCLIPackage+"@latest",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = RuntimeDir()

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm install: %w", err)
	}

	fmt.Printf("playwright-cli installed to %s\n", PlaywrightCLIDir())

	return nil
}

// createWrapper creates the playwright-cli wrapper script.
func createWrapper() error {
	binDir := filepath.Join(Paths(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("create bin dir: %w", err)
	}

	// Create wrapper script
	wrapper := `#!/bin/sh
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
exec "$SCRIPT_DIR/../runtime/node" "$SCRIPT_DIR/../runtime/node_modules/@playwright/cli/playwright-cli.js" "$@"
`

	wrapperPath := BinaryPath()
	if err := os.WriteFile(wrapperPath, []byte(wrapper), 0o755); err != nil {
		return fmt.Errorf("write wrapper: %w", err)
	}

	fmt.Printf("Wrapper created at %s\n", wrapperPath)

	return nil
}

// InstallBrowsers installs Playwright browsers (chromium by default).
func InstallBrowsers(ctx context.Context) error {
	if err := EnsureInstalled(ctx); err != nil {
		return err
	}

	fmt.Println("Installing Chromium browser...")
	cmd := exec.CommandContext(ctx, BinaryPath(), "install", "chromium")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("install chromium: %w", err)
	}

	return nil
}
