package commands

import (
	"strings"
	"testing"

	"github.com/valksor/kvelmo/internal/socket"
)

// --- rpc variants ---

func TestRunRPC_InvalidJSONParams(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	_ = startStubWorktreeSocket(t)

	if err := runRPC(RPCCmd, []string{"status", "{not json"}); err == nil {
		t.Fatal("expected error for invalid JSON params")
	}
}

func TestRunRPC_WithParams(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("status", map[string]any{"state": "planning"})

	out := captureStdout(t, func() {
		if err := runRPC(RPCCmd, []string{"status", `{"verbose":true}`}); err != nil {
			t.Errorf("runRPC with params: %v", err)
		}
	})
	if !strings.Contains(out, "planning") {
		t.Errorf("rpc with-params output:\n%s", out)
	}
}

func TestRunRPC_Global(t *testing.T) {
	setBoolPtr(t, &rpcGlobal, true)
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("ping", map[string]any{"ok": true})

	out := captureStdout(t, func() {
		if err := runRPC(RPCCmd, []string{"ping"}); err != nil {
			t.Errorf("runRPC global: %v", err)
		}
	})
	if !strings.Contains(out, "ok") {
		t.Errorf("rpc global output:\n%s", out)
	}
}

func TestRunRPC_GlobalNoSocket(t *testing.T) {
	setBoolPtr(t, &rpcGlobal, true)
	shortKvelmoHome(t)

	if err := runRPC(RPCCmd, []string{"ping"}); err == nil {
		t.Fatal("expected error with no global socket")
	}
}

func TestRunRPC_ServerError(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetError("status", -32099, "rpc boom")

	if err := runRPC(RPCCmd, []string{"status"}); err == nil {
		t.Fatal("expected rpc error to surface")
	}
}

// --- browser status: installed + version + config ---

func TestRunBrowserStatus_Installed(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("browser.status", map[string]any{
		"installed":   true,
		"runtime_dir": "/runtime",
		"binary_path": "/runtime/chromium",
		"version":     "120.0",
		"config": map[string]any{
			"headless": true, "browser": "chromium", "profile": "default", "timeout": 30,
		},
	})

	out := captureStdout(t, func() {
		if err := runBrowserStatus(BrowserCmd, nil); err != nil {
			t.Errorf("runBrowserStatus: %v", err)
		}
	})
	if !strings.Contains(out, "Installed: true") || !strings.Contains(out, "Version: 120.0") || !strings.Contains(out, "Configuration:") {
		t.Errorf("browser status installed output:\n%s", out)
	}
}

func TestRunBrowserStatus_VersionError(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("browser.status", map[string]any{
		"installed": true, "runtime_dir": "/r", "binary_path": "/r/c",
		"version_error": "exec failed", "config_error": "no config",
	})

	out := captureStdout(t, func() {
		if err := runBrowserStatus(BrowserCmd, nil); err != nil {
			t.Errorf("runBrowserStatus version-error: %v", err)
		}
	})
	if !strings.Contains(out, "Version: error") || !strings.Contains(out, "Config: error") {
		t.Errorf("browser status version-error output:\n%s", out)
	}
}

// --- retry mid-flow failures ---

func TestRunRetry_ResetFails(t *testing.T) {
	origPhase := retryPhase
	t.Cleanup(func() { retryPhase = origPhase })
	retryPhase = phaseImplement

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("status", map[string]any{"state": "failed", "last_error": "implement boom"})
	stub.SetError("reset", -32000, "reset failed")

	if err := runRetry(RetryCmd, nil); err == nil {
		t.Fatal("expected error when reset fails")
	}
}

func TestRunRetry_PhaseFails(t *testing.T) {
	origPhase := retryPhase
	t.Cleanup(func() { retryPhase = origPhase })
	retryPhase = phaseImplement

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("status", map[string]any{"state": "failed", "last_error": "boom"})
	stub.SetResponse("reset", map[string]any{"status": "ok", "state": "planned"})
	stub.SetError("implement", -32000, "phase failed")

	if err := runRetry(RetryCmd, nil); err == nil {
		t.Fatal("expected error when phase re-run fails")
	}
}

func TestRunRetry_InvalidPhaseOverride(t *testing.T) {
	origPhase := retryPhase
	t.Cleanup(func() { retryPhase = origPhase })
	retryPhase = "bogus-phase"

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("status", map[string]any{"state": "failed"})

	if err := runRetry(RetryCmd, nil); err == nil {
		t.Fatal("expected error for invalid phase override")
	}
}

// --- replaceOlderSocket: version mismatch triggers shutdown ---

func TestReplaceOlderSocket_VersionMismatch(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	// A clearly different version/commit forces the replacement branch.
	stub.SetResponse("ping", map[string]any{"version": "0.0.0-ancient", "commit": "0000000"})
	stub.SetResponse("shutdown", map[string]any{"ok": true})

	out := captureStdout(t, func() {
		_ = replaceOlderSocket(t.Context(), socket.GlobalSocketPath())
	})
	if !strings.Contains(out, "Replacing older socket") {
		t.Errorf("replaceOlderSocket mismatch output:\n%s", out)
	}
}

// --- config edit --project path ---

func TestRunConfigEdit_Project(t *testing.T) {
	t.Setenv("EDITOR", "true")
	setBoolPtr(t, &configEditProject, true)

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	configEditCmd.SetContext(t.Context())

	if err := runConfigEdit(configEditCmd, nil); err != nil {
		t.Errorf("runConfigEdit --project: %v", err)
	}
}
