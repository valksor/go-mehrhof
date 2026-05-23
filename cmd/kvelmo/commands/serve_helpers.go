package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/internal/socket"
	"github.com/valksor/kvelmo/meta"
)

// resolvePort determines the actual port to use.
// If port is explicitly set, use it. Otherwise try preferred port, fallback to random.
func resolvePort(cmd *cobra.Command, explicit int) int {
	// If explicit port specified via flag, use it
	if cmd.Flags().Changed("port") {
		return explicit
	}

	// Try preferred port
	if portAvailable("localhost", DefaultPreferredPort) {
		return DefaultPreferredPort
	}

	// Fallback to random
	fmt.Printf("Port %d in use, using random available port\n", DefaultPreferredPort)

	return 0
}

// portAvailable checks if a port is available for binding.
func portAvailable(host string, port int) bool {
	addr := fmt.Sprintf("%s:%d", host, port)
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", addr)
	if err != nil {
		return false
	}
	_ = ln.Close()

	return true
}

// findStaticDir locates the static files directory.
func findStaticDir(explicit string) string {
	if explicit != "" {
		return explicit
	}

	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	candidates := []string{
		filepath.Join(cwd, "web", "dist"),
		"web/dist",
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c
		}
	}

	return ""
}

// browserOpenCommand builds (but does not start) the OS command that opens url
// in the user's default browser, returning nil if no opener is available. It is
// split out from openBrowser so the opener-selection logic can be unit-tested
// without actually launching a browser — starting it would pop a real window or
// tab in whatever the user's default browser happens to be.
//
//nolint:noctx // exec.Command is intentional: the browser process must outlive the caller; CommandContext would kill it on cancel
func browserOpenCommand(url string) *exec.Cmd {
	// Only hand http(s) URLs to the OS opener; other schemes (file://, custom URI
	// handlers) are a needless risk and are never used by callers — serve passes
	// the local http server URL.
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return nil
	}
	switch {
	case fileExists("/usr/bin/open"): // macOS
		return exec.Command("/usr/bin/open", url)
	case fileExists("/usr/bin/xdg-open"): // Linux
		return exec.Command("/usr/bin/xdg-open", url)
	default:
		// Fallback: try "open" from PATH (macOS) or "xdg-open" (Linux)
		if path, err := exec.LookPath("open"); err == nil {
			return exec.Command(path, url)
		} else if path, err := exec.LookPath("xdg-open"); err == nil {
			return exec.Command(path, url)
		}

		return nil
	}
}

// openBrowser opens the specified URL in the default browser.
func openBrowser(url string) {
	if cmd := browserOpenCommand(url); cmd != nil {
		_ = cmd.Start()
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}

// replaceOlderSocket pings the running global socket and shuts it down if its
// version/commit differs from the current binary. This ensures that after an
// update the new binary always takes over as primary.
func replaceOlderSocket(ctx context.Context, globalPath string) error {
	if !socket.SocketExists(globalPath) {
		return fmt.Errorf("no socket at %s", globalPath)
	}

	client, err := socket.NewClient(globalPath, socket.WithTimeout(2*time.Second))
	if err != nil {
		return fmt.Errorf("connect to running socket: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	resp, err := client.Call(pingCtx, "ping", nil)
	_ = client.Close()
	if err != nil {
		return fmt.Errorf("ping running socket: %w", err)
	}

	var info struct {
		Version string `json:"version"`
		Commit  string `json:"commit"`
	}
	if resp.Result != nil {
		if err := json.Unmarshal(resp.Result, &info); err != nil {
			slog.Debug("failed to parse ping response, skipping replacement", statusError, err)

			return nil
		}
	}

	// Same build — nothing to replace
	if info.Version == meta.Version && info.Commit == meta.Commit {
		return nil
	}

	fmt.Printf("Replacing older socket (running %s/%s, current %s/%s)\n",
		info.Version, info.Commit, meta.Version, meta.Commit)

	shutClient, err := socket.NewClient(globalPath, socket.WithTimeout(2*time.Second))
	if err != nil {
		// Can't connect — remove stale socket file
		_ = os.Remove(globalPath)

		return nil
	}

	shutCtx, shutCancel := context.WithTimeout(ctx, 2*time.Second)
	defer shutCancel()
	_, _ = shutClient.Call(shutCtx, "shutdown", nil)
	_ = shutClient.Close()

	// Wait for socket file to disappear
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !socket.SocketExists(globalPath) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Force remove if still present
	_ = os.Remove(globalPath)

	return nil
}
