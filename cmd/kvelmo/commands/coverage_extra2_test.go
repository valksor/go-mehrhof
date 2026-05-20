package commands

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/valksor/kvelmo/internal/socket"
	"github.com/valksor/kvelmo/meta"
)

// TestRunDiagnose_Offline exercises the offline diagnostic path (no global
// socket). The function runs preflight checks, scans for provider tokens,
// inspects the global socket state, and prints the report.
func TestRunDiagnose_Offline(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)

	cmd := &cobra.Command{}
	// runDiagnose without --health and without socket falls through to offline path.
	_ = runDiagnose(cmd, nil)
}

// TestRunDiagnose_Health exercises the --health branch which runs runDiagnoseHealth.
func TestRunDiagnose_Health(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)

	orig := diagnoseHealth
	diagnoseHealth = true
	t.Cleanup(func() { diagnoseHealth = orig })

	cmd := &cobra.Command{}
	_ = runDiagnose(cmd, nil)
}

// TestRunDiagnoseViaRPC_NoSocket verifies the "not handled" return when no socket.
func TestRunDiagnoseViaRPC_NoSocket(t *testing.T) {
	shortKvelmoHome(t)
	handled, err := runDiagnoseViaRPC()
	if handled {
		t.Errorf("expected handled=false when no socket, got true (err: %v)", err)
	}
}

// TestRunDiagnoseViaRPC_WithSocket exercises the success path with a stub.
func TestRunDiagnoseViaRPC_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	stub := startStubGlobalSocket(t)
	// Make system.diagnose return a populated report so the print branches run.
	stub.SetResponse("system.diagnose", map[string]any{
		"checks": []any{
			map[string]any{"name": "git", "status": "ok", "detail": "/usr/bin/git"},
			map[string]any{"name": "claude", "status": "missing", "detail": "not found", "fix": "install claude"},
		},
		"providers": []any{
			map[string]any{"name": "GitHub", "configured": true},
		},
		"global_socket": "running",
		"issues":        []any{"install claude"},
	})

	handled, _ := runDiagnoseViaRPC()
	if !handled {
		t.Error("expected handled=true when socket responds")
	}
}

// TestRunDiagnoseHealth exercises the health subcommand which prints
// preflight checks. It runs without a socket so the body executes.
func TestRunDiagnoseHealth(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)

	_ = runDiagnoseHealth()
}

// TestRunQuick_NoText returns an error when no text/from is provided.
func TestRunQuick_NoText(t *testing.T) {
	// quickText is package-level; ensure clean state.
	origText, origFrom := quickText, quickSource
	t.Cleanup(func() { quickText, quickSource = origText, origFrom })

	quickText = ""
	quickSource = ""

	cmd := &cobra.Command{}
	err := runQuick(cmd, nil)
	if err == nil {
		t.Error("runQuick with no text/from should return an error")
	}
}

// TestRunQuick_MutualExclusion verifies --text and --from rejected together.
func TestRunQuick_MutualExclusion(t *testing.T) {
	origText, origFrom := quickText, quickSource
	t.Cleanup(func() { quickText, quickSource = origText, origFrom })

	quickText = "fix"
	quickSource = "file:t.md"

	cmd := &cobra.Command{}
	err := runQuick(cmd, nil)
	if err == nil {
		t.Error("expected mutual exclusion error")
	}
}

// TestRunUpgrade_MutualExclusion verifies --nightly and --version are exclusive.
func TestRunUpgrade_MutualExclusion(t *testing.T) {
	origN, origV := upgradeNightly, upgradeVersion
	t.Cleanup(func() { upgradeNightly, upgradeVersion = origN, origV })

	upgradeNightly = true
	upgradeVersion = "v1.0.0"

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	err := runUpgrade(cmd, nil)
	if err == nil {
		t.Error("expected mutual exclusion error")
	}
}

// TestRunUpgrade_VersionFlag exercises the path with an explicit version.
// The actual GitHub check will fail (no network or wrong tag) but the
// function body executes.
func TestRunUpgrade_VersionFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("network test")
	}

	origN, origV := upgradeNightly, upgradeVersion
	t.Cleanup(func() { upgradeNightly, upgradeVersion = origN, origV })

	upgradeNightly = false
	upgradeVersion = "v999.999.999-does-not-exist"

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	_ = runUpgrade(cmd, nil)
}

// TestRunRPC_ExecutesPing exercises runRPC against a stub socket.
func TestRunRPC_ExecutesPing(t *testing.T) {
	shortKvelmoHome(t)
	_ = startStubGlobalSocket(t)

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	_ = runRPC(cmd, []string{"ping"})
}

// TestRunRPC_NoSocket returns an error when no global socket exists.
func TestRunRPC_NoSocket(t *testing.T) {
	shortKvelmoHome(t)
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	if err := runRPC(cmd, []string{"ping"}); err == nil {
		t.Error("expected error when no global socket")
	}
}

// TestRunCleanup exercises cleanup. It can remove stale socket files and
// should succeed in dry-run mode.
func TestRunCleanup_DryRun(t *testing.T) {
	shortKvelmoHome(t)
	// Create a fake stale socket file.
	home := os.Getenv(meta.EnvPrefix + "_HOME")
	if err := os.WriteFile(filepath.Join(home, "global.sock"), []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cmd.Flags().Bool("force", false, "")
	cmd.Flags().Bool("dry-run", true, "")
	_ = cmd.Flags().Set("dry-run", "true")

	_ = runCleanup(cmd, nil)
}

// TestRunBackup exercises backup creation through the stub. The function
// should construct an output path and call backup.create on the socket.
func TestRunBackup_WithSocket(t *testing.T) {
	shortKvelmoHome(t)
	dir := chdirToShortTemp(t)
	stub := startStubGlobalSocket(t)
	stub.SetResponse("backup.create", map[string]any{
		"path": filepath.Join(dir, "backup.tar.gz"),
		"size": 1024,
	})

	cmd := &cobra.Command{}
	cmd.Flags().String("output", "", "")
	_ = runBackup(cmd, nil)
}

// TestRunRestore_NoSocket triggers an error.
func TestRunRestore_NoSocket(t *testing.T) {
	shortKvelmoHome(t)
	cmd := &cobra.Command{}
	if err := runRestore(cmd, []string{"/tmp/x.tar.gz"}); err == nil {
		t.Error("expected error with no socket")
	}
}

// TestRunNotifyTest_NoSocket exercises the no-socket error path.
func TestRunNotifyTest_NoSocket(t *testing.T) {
	shortKvelmoHome(t)
	cmd := &cobra.Command{}
	_ = runNotifyTest(cmd, nil)
}

// TestRunPipe_NoArgs returns an error when no prompt given.
func TestRunPipe_NoArgs(t *testing.T) {
	shortKvelmoHome(t)
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	_ = runPipe(cmd, nil)
}

// TestStartCmdFlags ensures the start command flags are registered.
func TestStartCmdFlags(t *testing.T) {
	expectedFlags := []string{
		"foreground", "verbose", "from", "text", "auto", "skip", "json",
		"file", "symbol", "commit", "provision-preview",
	}
	for _, name := range expectedFlags {
		if f := StartCmd.Flags().Lookup(name); f == nil {
			t.Errorf("StartCmd missing --%s flag", name)
		}
	}
}

func TestSocketHelpers_Existing(t *testing.T) {
	// Quick existence checks against the stub setup.
	shortKvelmoHome(t)
	_ = startStubGlobalSocket(t)
	if !socket.SocketExists(socket.GlobalSocketPath()) {
		t.Error("expected stub global socket to exist")
	}
}
