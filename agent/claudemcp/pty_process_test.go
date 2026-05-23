package claudemcp

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestStartPTYEmptyArgv covers the guard rejecting an empty argv.
func TestStartPTYEmptyArgv(t *testing.T) {
	if _, err := startPTY(context.Background(), nil, "", nil); err == nil {
		t.Fatal("startPTY with empty argv = nil, want error")
	}
}

// TestStartPTYStripsAnthropicCreds spawns a fake binary that echoes its
// environment. It verifies the adapter strips ANTHROPIC_* credential vars from
// the child env (so the session stays on the Max subscription) while injecting
// the supplied extraEnv. Both branches — stripping and extraEnv injection —
// are exercised here.
func TestStartPTYStripsAnthropicCreds(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PTY fake-binary harness requires a POSIX shell")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "envdump.sh")
	const body = `#!/bin/sh
env
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	// Set credentials in the parent env; startPTY must remove them from the
	// child. Restored automatically by t.Setenv.
	t.Setenv("ANTHROPIC_API_KEY", "sk-should-be-stripped")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "tok-should-be-stripped")

	p, err := startPTY(context.Background(), []string{script}, "", map[string]string{
		"KVELMO_FAKE_INJECTED": "yes",
	})
	if err != nil {
		t.Fatalf("startPTY: %v", err)
	}
	defer func() { _ = p.Close() }()

	// Collect the env dump until the PTY closes.
	var out []string
	timeout := time.After(5 * time.Second)
collect:
	for {
		select {
		case line, ok := <-p.lines:
			if !ok {
				break collect
			}
			out = append(out, line)
		case <-timeout:
			t.Fatal("timed out collecting env dump")
		}
	}

	joined := strings.Join(out, "\n")
	if containsLine(out, "ANTHROPIC_API_KEY", "sk-should-be-stripped") {
		t.Errorf("ANTHROPIC_API_KEY leaked into child env:\n%s", joined)
	}
	if containsLine(out, "ANTHROPIC_AUTH_TOKEN", "tok-should-be-stripped") {
		t.Errorf("ANTHROPIC_AUTH_TOKEN leaked into child env:\n%s", joined)
	}
	if !containsLine(out, "KVELMO_FAKE_INJECTED", "yes") {
		t.Errorf("extraEnv KVELMO_FAKE_INJECTED not injected into child env:\n%s", joined)
	}
}

// containsLine reports whether out contains a "KEY=VALUE" env line.
func containsLine(out []string, key, value string) bool {
	return slices.Contains(out, key+"="+value)
}

// TestStartPTYWorkDir verifies the spawned process runs in the configured
// working directory.
func TestStartPTYWorkDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PTY fake-binary harness requires a POSIX shell")
	}
	dir := t.TempDir()
	workDir := t.TempDir()
	// Resolve symlinks because macOS /tmp is a symlink to /private/tmp and pwd
	// reports the resolved path.
	resolved, err := filepath.EvalSymlinks(workDir)
	if err != nil {
		t.Fatal(err)
	}
	// The child writes its cwd to a file (via an injected env var) rather than to
	// the PTY, so the assertion doesn't depend on PTY line buffering/echo timing —
	// a leading blank line raced the real output under -race on Linux.
	outPath := filepath.Join(dir, "pwd.out")
	script := filepath.Join(dir, "pwd.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\npwd > \"$KVELMO_PWD_OUT\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	p, err := startPTY(context.Background(), []string{script}, workDir, map[string]string{"KVELMO_PWD_OUT": outPath})
	if err != nil {
		t.Fatalf("startPTY: %v", err)
	}
	defer func() { _ = p.Close() }()

	// Wait for the child to exit (p.done closes after cmd.Wait), then read the file.
	select {
	case <-p.done:
	case <-time.After(5 * time.Second):
		t.Fatal("child process did not exit")
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read pwd output: %v", err)
	}
	got, _ := filepath.EvalSymlinks(strings.TrimSpace(string(data)))
	if got != resolved {
		t.Errorf("pwd in child = %q, want %q", got, resolved)
	}
}

// TestStartPTYProcessExitsWithBufferedOutput is a regression test for the
// "send on closed channel" panic / data race: a child that prints several
// lines and then exits on its own must let readLoop drain and stop sending
// before waitLoop closes p.lines. The channel must close cleanly and surface
// every emitted line.
func TestStartPTYProcessExitsWithBufferedOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PTY fake-binary harness requires a POSIX shell")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "burst.sh")
	// Emit a burst of lines then exit immediately — output is still buffered in
	// the PTY when the process dies, which is exactly the window that raced.
	const body = `#!/bin/sh
i=0
while [ $i -lt 50 ]; do
  echo "line $i"
  i=$((i + 1))
done
exit 0
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	p, err := startPTY(context.Background(), []string{script}, "", nil)
	if err != nil {
		t.Fatalf("startPTY: %v", err)
	}
	defer func() { _ = p.Close() }()

	var count int
	timeout := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-p.lines:
			if !ok {
				// Channel closed cleanly (no panic) — done.
				if count == 0 {
					t.Error("expected at least one line before close")
				}

				return
			}
			count++
		case <-timeout:
			t.Fatal("timed out; lines channel never closed")
		}
	}
}

// TestPtyProcessCloseNil verifies Close on a nil ptyProcess is a safe no-op.
func TestPtyProcessCloseNil(t *testing.T) {
	var p *ptyProcess
	if err := p.Close(); err != nil {
		t.Errorf("nil ptyProcess.Close() = %v, want nil", err)
	}
}

// TestPtyProcessInterruptNil verifies Interrupt on a nil ptyProcess is a safe
// no-op (the nil-receiver guard).
func TestPtyProcessInterruptNil(t *testing.T) {
	var p *ptyProcess
	if err := p.Interrupt(); err != nil {
		t.Errorf("nil ptyProcess.Interrupt() = %v, want nil", err)
	}
}

// TestPtyProcessCloseTerminatesProcess verifies Close kills a long-running
// child and is idempotent.
func TestPtyProcessCloseTerminatesProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PTY fake-binary harness requires a POSIX shell")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "sleep.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nwhile :; do sleep 1; done\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	p, err := startPTY(context.Background(), []string{script}, "", nil)
	if err != nil {
		t.Fatalf("startPTY: %v", err)
	}

	if err := p.Close(); err != nil {
		t.Errorf("Close() = %v", err)
	}
	// done must close once the process exits.
	select {
	case <-p.done:
	case <-time.After(5 * time.Second):
		t.Fatal("process did not exit after Close()")
	}
	// Second Close is a no-op (closeOnce).
	if err := p.Close(); err != nil {
		t.Errorf("second Close() = %v", err)
	}
}
