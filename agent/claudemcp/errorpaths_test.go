package claudemcp

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/valksor/kvelmo/internal/mcp"
	"github.com/valksor/kvelmo/internal/testutil"
)

// TestWriteMCPConfigEmptyCommand covers the guard rejecting an empty server
// command.
func TestWriteMCPConfigEmptyCommand(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "mcp-config.json")
	err := writeMCPConfig(dest, nil, "t", "/wt", "/ws.sock", "/a.sock", 0)
	if err == nil {
		t.Fatal("writeMCPConfig with empty command = nil, want error")
	}
	if _, statErr := os.Stat(dest); statErr == nil {
		t.Error("no config file should be written when the command is empty")
	}
}

// TestWriteMCPConfigMkdirFails covers the mkdir-failure branch: the parent of
// dest is a regular file, so creating the directory must fail.
func TestWriteMCPConfigMkdirFails(t *testing.T) {
	dir := t.TempDir()
	fileAsParent := filepath.Join(dir, "afile")
	if err := os.WriteFile(fileAsParent, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(fileAsParent, "sub", "mcp-config.json")
	if err := writeMCPConfig(dest, []string{"x"}, "t", "/wt", "/ws", "/a", 0); err == nil {
		t.Fatal("writeMCPConfig with file-as-parent = nil, want error")
	}
}

// TestWriteMCPConfigWriteFails covers the os.WriteFile failure branch: dest is
// itself a directory, so writing the config file fails.
func TestWriteMCPConfigWriteFails(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "iamadir")
	if err := os.MkdirAll(dest, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeMCPConfig(dest, []string{"x"}, "t", "/wt", "/ws", "/a", 0); err == nil {
		t.Fatal("writeMCPConfig with dir dest = nil, want error")
	}
}

// TestPrepareWorkDirMkdirFails covers prepareWorkDir's MkdirAll-failure branch
// by pointing WorkRoot at a read-only directory.
func TestPrepareWorkDirMkdirFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses directory permission bits")
	}
	root := t.TempDir()
	readonly := filepath.Join(root, "ro")
	if err := os.Mkdir(readonly, 0o500); err != nil { // r-x, no write
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readonly, 0o700) }) // let TempDir cleanup remove it

	cfg := Config{WorkRoot: readonly}
	a := NewWithConfig(cfg)
	if _, err := a.prepareWorkDir("task"); err == nil {
		t.Fatal("prepareWorkDir under read-only root = nil, want error")
	}
}

// TestWriteSystemPromptMkdirFails covers writeSystemPrompt's mkdir-failure
// branch (parent path is a file).
func TestWriteSystemPromptMkdirFails(t *testing.T) {
	dir := t.TempDir()
	fileAsParent := filepath.Join(dir, "notadir")
	if err := os.WriteFile(fileAsParent, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(fileAsParent, "sub", "system-prompt.md")
	if err := writeSystemPrompt(dest, "", "t", "p", "/wt", false); err == nil {
		t.Fatal("writeSystemPrompt with file-as-parent = nil, want error")
	}
}

// TestWriteSystemPromptWriteFails covers the os.WriteFile failure branch: dest
// is itself a directory, so writing the file fails.
func TestWriteSystemPromptWriteFails(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "iamadir")
	if err := os.MkdirAll(dest, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeSystemPrompt(dest, "body", "t", "p", "/wt", false); err == nil {
		t.Fatal("writeSystemPrompt with dir dest = nil, want error")
	}
}

// TestRendezvousCloseNil verifies Close on a nil listener is a safe no-op.
func TestRendezvousCloseNil(t *testing.T) {
	var r *rendezvousListener
	if err := r.Close(); err != nil {
		t.Errorf("nil rendezvousListener.Close() = %v, want nil", err)
	}
}

// TestRendezvousIgnoresBadJSON covers handleRendezvousConn's malformed-frame
// skip: a junk line is dropped, and a subsequent valid frame still arrives.
func TestRendezvousIgnoresBadJSON(t *testing.T) {
	sockPath := testutil.TempSocketPath(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rdz, events, err := startRendezvous(ctx, sockPath)
	if err != nil {
		t.Fatalf("startRendezvous: %v", err)
	}
	defer func() { _ = rdz.Close() }()

	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", sockPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// First line is garbage (must be skipped), second is a valid event.
	if _, err := conn.Write([]byte("this is not json\n")); err != nil {
		t.Fatalf("write junk: %v", err)
	}
	if _, err := conn.Write([]byte(`{"type":"complete","phase":"plan","summary":"ok"}` + "\n")); err != nil {
		t.Fatalf("write valid: %v", err)
	}

	select {
	case got := <-events:
		if got.Type != "complete" || got.Phase != "plan" || got.Summary != "ok" {
			t.Errorf("unexpected event after junk: %+v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out; valid event after junk was not delivered")
	}
}

// TestRendezvousFilePermissions asserts the socket is created with 0600 perms.
func TestRendezvousFilePermissions(t *testing.T) {
	sockPath := testutil.TempSocketPath(t)
	ctx := t.Context()

	rdz, _, err := startRendezvous(ctx, sockPath)
	if err != nil {
		t.Fatalf("startRendezvous: %v", err)
	}
	defer func() { _ = rdz.Close() }()

	info, err := os.Stat(sockPath)
	if err != nil {
		t.Fatalf("stat socket: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("socket mode = %o, want 0600", mode)
	}
}

// TestStartRendezvousBindFailure covers startRendezvous's listen-failure
// branch: binding to a path under a nonexistent directory fails.
func TestStartRendezvousBindFailure(t *testing.T) {
	bad := filepath.Join(testutil.TempDir(t), "no-such-dir", "x.sock")
	ctx := t.Context()
	if _, _, err := startRendezvous(ctx, bad); err == nil {
		t.Fatal("startRendezvous on unbindable path = nil, want error")
	}
}

// TestSendPromptStartPTYFailureCleansUp drives SendPrompt all the way to the
// PTY spawn against a nonexistent binary so startPTY fails. The error must be
// surfaced and the per-session work dir cleaned up (cleanupOnError path).
func TestSendPromptStartPTYFailureCleansUp(t *testing.T) {
	worktree := testutil.TempDir(t)
	workRoot := testutil.TempDir(t)
	cfg := Config{
		WorkRoot:       workRoot,
		PermissionMode: PermissionModeAcceptEdits,
	}
	cfg.WorkDir = worktree
	cfg.Command = []string{filepath.Join(workRoot, "does-not-exist-binary")}
	a := NewWithConfig(cfg)

	_, err := a.SendPrompt(context.Background(), "hi")
	if err == nil {
		t.Fatal("SendPrompt with missing binary = nil, want startPTY error")
	}

	// cleanupOnError removes the per-session claudemcp-<rand> work dir (the
	// task dir above it may remain). The agent recorded the workDir, so assert
	// that exact directory is gone.
	if a.workDir == "" {
		t.Fatal("agent did not record a workDir")
	}
	if _, statErr := os.Stat(a.workDir); !os.IsNotExist(statErr) {
		t.Errorf("session work dir %q not removed on error (stat err = %v)", a.workDir, statErr)
	}

	// Listener and pty must not be retained on the failed agent.
	if a.rdz != nil {
		t.Error("rdz should be nil after SendPrompt failure")
	}
	if a.pty != nil {
		t.Error("pty should be nil after SendPrompt failure")
	}
}

// TestSendPromptWriteMCPConfigFailureCleansUp drives SendPrompt to the
// writeMCPConfig step and forces it to fail by clearing MCPServerCommand, then
// asserts the rendezvous listener is torn down and the work dir removed.
func TestSendPromptWriteMCPConfigFailureCleansUp(t *testing.T) {
	worktree := testutil.TempDir(t)
	workRoot := testutil.TempDir(t)
	cfg := Config{
		WorkRoot:       workRoot,
		PermissionMode: PermissionModeAcceptEdits,
	}
	cfg.WorkDir = worktree
	a := NewWithConfig(cfg)
	// NewWithConfig backfilled the default MCPServerCommand; clear it so
	// writeMCPConfig hits its empty-command guard.
	a.config.MCPServerCommand = nil

	_, err := a.SendPrompt(context.Background(), "hi")
	if err == nil {
		t.Fatal("SendPrompt with empty MCPServerCommand = nil, want write mcp config error")
	}

	// The per-session work dir is removed on the writeMCPConfig failure path.
	if a.workDir == "" {
		t.Fatal("agent did not record a workDir")
	}
	if _, statErr := os.Stat(a.workDir); !os.IsNotExist(statErr) {
		t.Errorf("session work dir %q not removed after writeMCPConfig failure (stat err = %v)", a.workDir, statErr)
	}
	if a.rdz != nil {
		t.Error("rdz should be nil after writeMCPConfig failure")
	}
}

// TestRendezvousEventFieldsRoundTrip is a sanity check on the wire shape the
// adapter and MCP server agree on — guards against accidental json tag drift.
func TestRendezvousEventFieldsRoundTrip(t *testing.T) {
	sockPath := testutil.TempSocketPath(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	rdz, events, err := startRendezvous(ctx, sockPath)
	if err != nil {
		t.Fatalf("startRendezvous: %v", err)
	}
	defer func() { _ = rdz.Close() }()

	conn := dialUnix(t, sockPath)
	defer func() { _ = conn.Close() }()

	want := mcp.RendezvousEvent{
		Type:      "failure",
		Phase:     "review",
		Reason:    "blocked",
		Retryable: true,
	}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if _, err := conn.Write(append(data, '\n')); err != nil {
		t.Fatalf("write: %v", err)
	}

	select {
	case got := <-events:
		if got != want {
			t.Errorf("round-trip mismatch: got %+v, want %+v", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for round-trip event")
	}
}
