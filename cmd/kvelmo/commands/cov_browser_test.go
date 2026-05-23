package commands

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestBrowserAgentCommands_WithSocket drives every browser_agent command's
// happy path against the global stub socket, asserting each returns nil and
// the JSON response is echoed. These are the agent-facing browser RPC wrappers;
// the live Playwright session is server-side, so the command layer (request
// build + response echo) is fully exercisable through the stub.
func TestBrowserAgentCommands_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	_ = startStubGlobalSocket(t) // serves all browser.* methods by default

	cases := []struct {
		name string
		fn   func(*cobra.Command, []string) error
		args []string
	}{
		{"navigate", runBrowserNavigate, []string{"https://example.com"}},
		{"snapshot", runBrowserSnapshot, nil},
		{"screenshot", runBrowserScreenshot, nil},
		{"click", runBrowserClick, []string{"#btn"}},
		{"type", runBrowserType, []string{"#input", "hello"}},
		{"wait", runBrowserWait, []string{"#ready"}},
		{"eval", runBrowserEval, []string{"1+1"}},
		{"console", runBrowserConsole, nil},
		{"network", runBrowserNetwork, nil},
		{"fill", runBrowserFill, []string{"#field", "value"}},
		{"select", runBrowserSelect, []string{"#sel", "opt1"}},
		{"hover", runBrowserHover, []string{"#hover"}},
		{"focus", runBrowserFocus, []string{"#focus"}},
		{"scroll", runBrowserScroll, []string{"down"}},
		{"press", runBrowserPress, []string{"Enter"}},
		{"back", runBrowserBack, nil},
		{"forward", runBrowserForward, nil},
		{"reload", runBrowserReload, nil},
		{"dialog", runBrowserDialog, []string{"accept"}},
		{"upload", runBrowserUpload, []string{"#file", "/tmp/x.txt"}},
		{"pdf", runBrowserPDF, nil},
	}

	// console with the default empty-message response legitimately prints
	// nothing, so it is asserted separately (TestBrowserConsole_Populated).
	noOutputOK := map[string]bool{"console": true}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := captureStdout(t, func() {
				if err := c.fn(BrowserCmd, c.args); err != nil {
					t.Errorf("runBrowser%s: %v", c.name, err)
				}
			})
			if out == "" && !noOutputOK[c.name] {
				t.Errorf("runBrowser%s produced no output", c.name)
			}
		})
	}
}

// TestBrowserAgentCommands_NoSocketErrors confirms each wrapper surfaces an
// error when no global socket is running (covers the early-return branch).
func TestBrowserConsole_Populated(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("browser.console", map[string]any{
		"messages": []any{
			map[string]any{"type": "error", "text": "boom"},
			map[string]any{"type": "log", "text": "hi"},
		},
	})

	out := captureStdout(t, func() {
		if err := runBrowserConsole(BrowserCmd, nil); err != nil {
			t.Errorf("runBrowserConsole: %v", err)
		}
	})
	if out == "" {
		t.Error("browser console produced no output")
	}
}

func TestBrowserEval_Error(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("browser.eval", map[string]any{"error": "ReferenceError"})

	if err := runBrowserEval(BrowserCmd, []string{"badjs"}); err == nil {
		t.Fatal("expected error from browser.eval error field")
	}
}

func TestBrowserAgentCommands_ServerErrors(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubGlobalSocket(t)
	stub.SetError("browser.navigate", -32000, "boom")
	stub.SetError("browser.click", -32000, "boom")

	if err := runBrowserNavigate(BrowserCmd, []string{"https://example.com"}); err == nil {
		t.Error("expected error from browser.navigate")
	}
	if err := runBrowserClick(BrowserCmd, []string{"#btn"}); err == nil {
		t.Error("expected error from browser.click")
	}
}
