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

// TestStartPTYStripsAnthropicCreds spawns a fake binary that dumps its
// environment to a file. It verifies the child env is built from kvelmo's
// config only: ANTHROPIC_* credential vars are stripped (so the session stays
// on the Max subscription), per-task vars are passed through, and the
// orchestrator's host environment (os.Environ) is never inherited.
func TestStartPTYStripsAnthropicCreds(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PTY fake-binary harness requires a POSIX shell")
	}
	dir := t.TempDir()
	outPath := filepath.Join(dir, "env.out")
	script := filepath.Join(dir, "envdump.sh")
	// Write the env to a file rather than the PTY. A large env dump (a CI runner
	// sets dozens of vars) can be truncated when the process exits and the PTY
	// slave closes before readLoop drains it, dropping the trailing injected var.
	const body = "#!/bin/sh\nenv > \"$KVELMO_ENV_OUT\"\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	// A host-only var (in the orchestrator's environment but not kvelmo's config)
	// must NOT reach the child: the child env comes from the config-dir env, never
	// os.Environ(). Isolate from any real config-dir .env via a temp KVELMO_HOME.
	t.Setenv("KVELMO_HOST_ONLY", "leaked")
	t.Setenv("KVELMO_HOME", t.TempDir())

	// The Anthropic credential is supplied as a per-task var; startPTY must strip
	// it so claude falls back to its saved login. KVELMO_FAKE_INJECTED checks a
	// normal per-task var is passed through.
	p, err := startPTY(context.Background(), []string{script}, "", map[string]string{
		"ANTHROPIC_API_KEY":    "sk-should-be-stripped",
		"KVELMO_FAKE_INJECTED": "yes",
		"KVELMO_ENV_OUT":       outPath,
	})
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
		t.Fatalf("read env dump: %v", err)
	}
	out := strings.Split(string(data), "\n")
	if containsLine(out, "ANTHROPIC_API_KEY", "sk-should-be-stripped") {
		t.Errorf("ANTHROPIC_API_KEY not stripped from child env:\n%s", data)
	}
	if containsLine(out, "KVELMO_HOST_ONLY", "leaked") {
		t.Errorf("host env var leaked into child env (os.Environ must not be inherited):\n%s", data)
	}
	if !containsLine(out, "KVELMO_FAKE_INJECTED", "yes") {
		t.Errorf("per-task var KVELMO_FAKE_INJECTED not passed to child env:\n%s", data)
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
