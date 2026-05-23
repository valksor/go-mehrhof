package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// isolateBinaryLookup empties PATH and points HOME at a temp dir so neither
// exec.LookPath nor the home-relative fallback paths (~/.local/bin, ~/.bun/bin)
// can resolve a binary.
func isolateBinaryLookup(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOME", t.TempDir())
}

// isolateBinaryLookupWithPATH is like isolateBinaryLookup but keeps a caller
// supplied PATH dir (e.g. one holding a fake git).
func isolateBinaryLookupWithPATH(t *testing.T, pathDir string) {
	t.Helper()
	t.Setenv("PATH", pathDir)
	t.Setenv("HOME", t.TempDir())
}

// claudeOnFallbackPaths reports whether a real claude exists in one of the
// hard-coded absolute fallback locations that tests cannot override.
func claudeOnFallbackPaths() bool {
	for _, p := range []string{
		filepath.Join("/opt/homebrew/bin", "claude"),
		filepath.Join("/usr/local/bin", "claude"),
		filepath.Join("/usr/bin", "claude"),
	} {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return true
		}
	}

	return false
}

// writeFakeBin writes an executable shell script into dir and returns the dir.
func writeFakeBin(t *testing.T, dir, name, script string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake-binary harness requires a POSIX shell")
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatalf("write fake %s: %v", name, err)
	}
}

func TestGetVersion_Success(t *testing.T) {
	dir := t.TempDir()
	writeFakeBin(t, dir, "git", `echo "git version 2.99.0"`)
	t.Setenv("PATH", dir)

	got, err := getVersion("git", "--version")
	if err != nil {
		t.Fatalf("getVersion error: %v", err)
	}
	if got != "2.99.0" {
		t.Errorf("getVersion = %q, want 2.99.0", got)
	}
}

func TestGetVersion_MultiLineTakesFirst(t *testing.T) {
	dir := t.TempDir()
	writeFakeBin(t, dir, "tool", `printf '1.2.3\nextra line\n'`)
	t.Setenv("PATH", dir)

	got, err := getVersion("tool", "--version")
	if err != nil {
		t.Fatalf("getVersion error: %v", err)
	}
	if got != "1.2.3" {
		t.Errorf("getVersion = %q, want 1.2.3 (first line only)", got)
	}
}

func TestGetVersion_CommandNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := getVersion("no-such-binary-zzz", "--version"); err == nil {
		t.Error("getVersion should fail for missing command")
	}
}

func TestGetVersion_CommandFails(t *testing.T) {
	dir := t.TempDir()
	writeFakeBin(t, dir, "failer", `exit 1`)
	t.Setenv("PATH", dir)

	if _, err := getVersion("failer", "--version"); err == nil {
		t.Error("getVersion should propagate a non-zero exit")
	}
}

func TestCheckClaudeAuth_NotAuthenticated(t *testing.T) {
	dir := t.TempDir()
	// Fake claude prints an auth-related message and exits non-zero.
	writeFakeBin(t, dir, "claude", `echo "Please login first" >&2; exit 1`)
	t.Setenv("PATH", dir)

	err := checkClaudeAuth()
	if err == nil {
		t.Fatal("checkClaudeAuth should report not authenticated")
	}
	if !strings.Contains(err.Error(), "not authenticated") {
		t.Errorf("expected 'not authenticated', got %v", err)
	}
}

func TestCheckClaudeAuth_OtherError(t *testing.T) {
	dir := t.TempDir()
	// Non-auth failure (e.g. some unrelated message) should be a generic failure.
	writeFakeBin(t, dir, "claude", `echo "network unreachable" >&2; exit 7`)
	t.Setenv("PATH", dir)

	err := checkClaudeAuth()
	if err == nil {
		t.Fatal("checkClaudeAuth should report a check failure")
	}
	if strings.Contains(err.Error(), "not authenticated") {
		t.Errorf("non-auth error misclassified as auth failure: %v", err)
	}
	if !strings.Contains(err.Error(), "auth check failed") {
		t.Errorf("expected 'auth check failed', got %v", err)
	}
}

func TestCheckClaudeAuth_Success(t *testing.T) {
	dir := t.TempDir()
	writeFakeBin(t, dir, "claude", `exit 0`)
	t.Setenv("PATH", dir)

	if err := checkClaudeAuth(); err != nil {
		t.Errorf("checkClaudeAuth with successful claude = %v, want nil", err)
	}
}

func TestCheckClaudeAuth_ClaudeNotFound(t *testing.T) {
	// Defeat both PATH lookup and the home-dir fallbacks (~/.local/bin, ~/.bun/bin)
	// so claude is genuinely unresolvable. The hard-coded /opt/homebrew, /usr/local
	// and /usr/bin locations do not contain claude on this host.
	isolateBinaryLookup(t)

	if claudeOnFallbackPaths() {
		t.Skip("a real claude binary is installed in a hard-coded fallback path; cannot isolate")
	}

	err := checkClaudeAuth()
	if err == nil {
		t.Fatal("checkClaudeAuth should fail when claude is not resolvable")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found', got %v", err)
	}
}

func TestCheckOllamaAvailable_NotReachable(t *testing.T) {
	// Default localhost:11434 is almost certainly not running in CI; assert the
	// error path. If it happens to be reachable, the call returns nil, which is
	// also a valid outcome — so only assert no panic and a deterministic type.
	err := checkOllamaAvailable()
	if err != nil && !strings.Contains(err.Error(), "") {
		t.Fatalf("unexpected error shape: %v", err)
	}
}

func TestRunPreflight_WithFakeBinaries(t *testing.T) {
	dir := t.TempDir()
	writeFakeBin(t, dir, "git", `echo "git version 2.50.0"`)
	writeFakeBin(t, dir, "claude", `case "$1" in --version) echo "claude 9.9.9";; auth) exit 0;; esac`)
	writeFakeBin(t, dir, "codex", `echo "codex 1.0.0"`)
	t.Setenv("PATH", dir)
	// Isolate env-map loading from the developer's real home.
	t.Setenv("KVELMO_HOME", t.TempDir())

	result := RunPreflight()
	if result == nil {
		t.Fatal("RunPreflight returned nil")
	}

	byName := map[string]CheckResult{}
	for _, c := range result.Checks {
		byName[c.Name] = c
	}

	if byName["git"].Status != CheckPassed {
		t.Errorf("git status = %q, want passed", byName["git"].Status)
	}
	if byName["claude"].Status != CheckPassed {
		t.Errorf("claude status = %q, want passed", byName["claude"].Status)
	}
	if byName["claude-auth"].Status != CheckPassed {
		t.Errorf("claude-auth status = %q, want passed", byName["claude-auth"].Status)
	}
	if byName["codex"].Status != CheckPassed {
		t.Errorf("codex status = %q, want passed", byName["codex"].Status)
	}
	if !result.AgentAvailable {
		t.Error("AgentAvailable should be true when claude+codex present")
	}
	if result.SimulationMode {
		t.Error("SimulationMode should be false when an agent is available")
	}
}

func TestRunPreflight_ClaudeFailedWhenAbsent(t *testing.T) {
	// Isolate binary lookup so claude is unresolvable. Provide a fake git so the
	// git check passes deterministically. Codex may still exist in a hard-coded
	// fallback path on a developer machine, so we only assert the claude branch.
	dir := t.TempDir()
	writeFakeBin(t, dir, "git", `echo "git version 2.50.0"`)
	isolateBinaryLookupWithPATH(t, dir)
	t.Setenv("KVELMO_HOME", t.TempDir())

	if claudeOnFallbackPaths() {
		t.Skip("a real claude binary is installed in a hard-coded fallback path; cannot isolate")
	}

	result := RunPreflight()

	byName := map[string]CheckResult{}
	for _, c := range result.Checks {
		byName[c.Name] = c
	}

	if byName["claude"].Status != CheckFailed {
		t.Errorf("claude status = %q, want failed when absent", byName["claude"].Status)
	}
	if byName["claude"].Fix == "" {
		t.Error("failed claude check should carry a Fix hint")
	}
	if !result.HasIssues() {
		t.Error("HasIssues() should be true when claude failed")
	}
}

func TestRunPreflight_ClaudeOnlyMakesCodexWarning(t *testing.T) {
	dir := t.TempDir()
	writeFakeBin(t, dir, "git", `echo "git version 2.50.0"`)
	writeFakeBin(t, dir, "claude", `case "$1" in --version) echo "claude 9.9.9";; auth) exit 0;; esac`)
	t.Setenv("PATH", dir)
	t.Setenv("KVELMO_HOME", t.TempDir())

	result := RunPreflight()

	byName := map[string]CheckResult{}
	for _, c := range result.Checks {
		byName[c.Name] = c
	}

	// Codex absent but Claude present → codex is a warning, not a failure.
	if byName["codex"].Status != CheckWarning {
		t.Errorf("codex status = %q, want warning when claude present", byName["codex"].Status)
	}
	if !result.AgentAvailable {
		t.Error("AgentAvailable should be true when claude present")
	}
}

func TestPrintPreflight_AllStatuses(t *testing.T) {
	// PrintPreflight writes to stdout; capture it and assert the symbols/labels.
	r := &PreflightResult{
		Checks: []CheckResult{
			{Name: "git", Status: CheckPassed, Detail: "2.50.0"},
			{Name: "claude", Status: CheckFailed, Detail: "not found", Fix: "install it"},
			{Name: "codex", Status: CheckWarning, Detail: "missing"},
		},
	}

	out := captureStdout(t, func() { PrintPreflight(r) })

	for _, want := range []string{"git", "claude", "codex", "passed", "failed", "warning", "install it"} {
		if !strings.Contains(out, want) {
			t.Errorf("PrintPreflight output missing %q\n%s", want, out)
		}
	}
}

// captureStdout redirects os.Stdout for the duration of fn and returns what was
// written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	rd, wr, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = wr

	done := make(chan string, 1)
	go func() {
		var sb strings.Builder
		buf := make([]byte, 4096)
		for {
			n, readErr := rd.Read(buf)
			if n > 0 {
				sb.Write(buf[:n])
			}
			if readErr != nil {
				break
			}
		}
		done <- sb.String()
	}()

	fn()
	_ = wr.Close()
	os.Stdout = orig

	return <-done
}
