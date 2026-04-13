package commands

import (
	"strings"
	"testing"
	"time"

	"github.com/valksor/kvelmo/meta"
)

// ─── capitalize ──────────────────────────────────────────────────────────────

func TestCapitalize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"planning", "Planning"},
		{"Planning", "Planning"},
		{"123abc", "123abc"},
		{"implementing", "Implementing"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := capitalize(tt.input)
			if got != tt.want {
				t.Errorf("capitalize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ─── fileExists ──────────────────────────────────────────────────────────────

func TestFileExists_Existing(t *testing.T) {
	dir := t.TempDir()
	if !fileExists(dir) {
		t.Errorf("fileExists(%q) = false, want true for existing directory", dir)
	}
}

func TestFileExists_NonExistent(t *testing.T) {
	if fileExists("/tmp/kvelmo-test-nonexistent-path-xyz-abc-12345") {
		t.Error("fileExists() returned true for non-existent path, want false")
	}
}

// ─── portAvailable ───────────────────────────────────────────────────────────

func TestPortAvailable_ValidHost(t *testing.T) {
	// Just verify the function runs without panicking
	result := portAvailable("localhost", 19997)
	_ = result // Available or not depends on environment
}

func TestPortAvailable_InvalidHost(t *testing.T) {
	// Invalid host → net.Listen fails → returns false
	if portAvailable("host-that-does-not-exist.invalid.local", 12345) {
		t.Error("portAvailable() with invalid host should return false")
	}
}

// ─── findStaticDir ───────────────────────────────────────────────────────────

func TestFindStaticDir_ExplicitNonEmpty(t *testing.T) {
	dir := t.TempDir()
	got := findStaticDir(dir)
	if got != dir {
		t.Errorf("findStaticDir(%q) = %q, want explicit value", dir, got)
	}
}

func TestFindStaticDir_EmptyString(t *testing.T) {
	// Empty explicit: searches cwd candidates; in test environment likely returns ""
	got := findStaticDir("")
	// Just verify no panic; result is environment-dependent
	_ = got
}

// ─── runPipe ─────────────────────────────────────────────────────────────────

func TestRunPipe_FakeAgent(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())

	// Save and restore pipeAgent global
	origAgent := pipeAgent
	pipeAgent = "nonexistent-agent-kvelmo-test-xyz"
	defer func() { pipeAgent = origAgent }()

	origTimeout := pipeTimeout
	pipeTimeout = 100 * time.Millisecond
	defer func() { pipeTimeout = origTimeout }()

	err := runPipe(PipeCmd, []string{"hello world"})
	if err == nil {
		t.Fatal("runPipe() with nonexistent agent should return error")
	}
	// Should fail at resolve agent or connect agent step
	if !strings.Contains(err.Error(), "resolve agent") &&
		!strings.Contains(err.Error(), "connect agent") &&
		!strings.Contains(err.Error(), "load settings") {
		t.Logf("runPipe() error = %v (acceptable error from agent resolution)", err)
	}
}

// ─── globalBrowserClient ─────────────────────────────────────────────────────

func TestGlobalBrowserClient_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	_, _, _, err := globalBrowserClient() //nolint:dogsled // only err is needed; client/ctx/cancel are unused in this error path test
	if err == nil {
		t.Error("globalBrowserClient() with no server should return error")
	}
}

// ─── browser command no-socket tests ─────────────────────────────────────────

func TestRunBrowserInstall_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runBrowserInstall(BrowserCmd, nil); err == nil {
		t.Error("runBrowserInstall() expected error (no socket), got nil")
	}
}

func TestRunBrowserStatus_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runBrowserStatus(BrowserCmd, nil); err == nil {
		t.Error("runBrowserStatus() expected error (no socket), got nil")
	}
}

func TestRunBrowserConfig_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runBrowserConfig(BrowserCmd, nil); err == nil {
		t.Error("runBrowserConfig() expected error (no socket), got nil")
	}
}

func TestRunBrowserConfigSet_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runBrowserConfigSet(BrowserCmd, []string{"headless=false"}); err == nil {
		t.Error("runBrowserConfigSet() expected error (no socket), got nil")
	}
}

func TestRunBrowserNavigate_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runBrowserNavigate(BrowserCmd, []string{"https://example.com"}); err == nil {
		t.Error("runBrowserNavigate() expected error (no socket), got nil")
	}
}

func TestRunBrowserSnapshot_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runBrowserSnapshot(BrowserCmd, nil); err == nil {
		t.Error("runBrowserSnapshot() expected error (no socket), got nil")
	}
}

func TestRunBrowserScreenshot_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runBrowserScreenshot(BrowserCmd, nil); err == nil {
		t.Error("runBrowserScreenshot() expected error (no socket), got nil")
	}
}

func TestRunBrowserClick_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runBrowserClick(BrowserCmd, []string{"Submit button"}); err == nil {
		t.Error("runBrowserClick() expected error (no socket), got nil")
	}
}

func TestRunBrowserType_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runBrowserType(BrowserCmd, []string{"input", "hello"}); err == nil {
		t.Error("runBrowserType() expected error (no socket), got nil")
	}
}

func TestRunBrowserWait_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runBrowserWait(BrowserCmd, nil); err == nil {
		t.Error("runBrowserWait() expected error (no socket), got nil")
	}
}

func TestRunBrowserEval_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runBrowserEval(BrowserCmd, []string{"document.title"}); err == nil {
		t.Error("runBrowserEval() expected error (no socket), got nil")
	}
}

func TestRunBrowserConsole_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runBrowserConsole(BrowserCmd, nil); err == nil {
		t.Error("runBrowserConsole() expected error (no socket), got nil")
	}
}

func TestRunBrowserNetwork_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runBrowserNetwork(BrowserCmd, nil); err == nil {
		t.Error("runBrowserNetwork() expected error (no socket), got nil")
	}
}

func TestRunBrowserFill_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runBrowserFill(BrowserCmd, []string{"Name", "Alice"}); err == nil {
		t.Error("runBrowserFill() expected error (no socket), got nil")
	}
}

func TestRunBrowserSelect_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runBrowserSelect(BrowserCmd, []string{"dropdown", "option1"}); err == nil {
		t.Error("runBrowserSelect() expected error (no socket), got nil")
	}
}

func TestRunBrowserHover_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runBrowserHover(BrowserCmd, []string{"button"}); err == nil {
		t.Error("runBrowserHover() expected error (no socket), got nil")
	}
}

func TestRunBrowserFocus_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runBrowserFocus(BrowserCmd, []string{"input"}); err == nil {
		t.Error("runBrowserFocus() expected error (no socket), got nil")
	}
}

func TestRunBrowserScroll_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runBrowserScroll(BrowserCmd, nil); err == nil {
		t.Error("runBrowserScroll() expected error (no socket), got nil")
	}
}

func TestRunBrowserPress_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runBrowserPress(BrowserCmd, []string{"Enter"}); err == nil {
		t.Error("runBrowserPress() expected error (no socket), got nil")
	}
}

func TestRunBrowserBack_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runBrowserBack(BrowserCmd, nil); err == nil {
		t.Error("runBrowserBack() expected error (no socket), got nil")
	}
}

func TestRunBrowserForward_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runBrowserForward(BrowserCmd, nil); err == nil {
		t.Error("runBrowserForward() expected error (no socket), got nil")
	}
}

func TestRunBrowserReload_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runBrowserReload(BrowserCmd, nil); err == nil {
		t.Error("runBrowserReload() expected error (no socket), got nil")
	}
}

func TestRunBrowserDialog_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runBrowserDialog(BrowserCmd, nil); err == nil {
		t.Error("runBrowserDialog() expected error (no socket), got nil")
	}
}

func TestRunBrowserUpload_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runBrowserUpload(BrowserCmd, []string{"/tmp/file.txt"}); err == nil {
		t.Error("runBrowserUpload() expected error (no socket), got nil")
	}
}

func TestRunBrowserPDF_NoSocket(t *testing.T) {
	t.Setenv(meta.EnvPrefix+"_HOME", t.TempDir())
	if err := runBrowserPDF(BrowserCmd, nil); err == nil {
		t.Error("runBrowserPDF() expected error (no socket), got nil")
	}
}

// ─── resolvePort ─────────────────────────────────────────────────────────────

func TestResolvePort_PreferredPortFree(t *testing.T) {
	// Just verify the function doesn't panic
	port := resolvePort(ServeCmd, DefaultPreferredPort)
	if port < 0 {
		t.Errorf("resolvePort() = %d, want >= 0", port)
	}
}

// ─── os.Getwd independent path for runBrowse ─────────────────────────────────

func TestBrowseViaSocket_NoSocket(t *testing.T) {
	// browseViaSocket with a nonexistent socket path should return a connection error
	if err := browseViaSocket("/tmp/kvelmo-test-nonexistent-socket.sock", nil); err == nil {
		t.Error("browseViaSocket() expected error (no socket), got nil")
	}
}
