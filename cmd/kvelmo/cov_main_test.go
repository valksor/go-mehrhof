package main

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/valksor/kvelmo/internal/cli"
	"github.com/valksor/kvelmo/meta"
)

// captureMainStdout runs fn while os.Stdout is redirected to a pipe and returns
// everything fn printed. The version/license commands write directly via
// fmt.Printf to os.Stdout (not cmd.OutOrStdout), so SetOut cannot capture them.
func captureMainStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close pipe writer: %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}

	return string(out)
}

// TestVersionCommandRun executes the version command's Run func and asserts it
// prints a version line containing the binary name.
func TestVersionCommandRun(t *testing.T) {
	out := captureMainStdout(t, func() {
		versionCmd.Run(versionCmd, nil)
	})
	if !strings.Contains(out, meta.Name) {
		t.Errorf("version output %q does not contain binary name %q", out, meta.Name)
	}
}

// TestLicenseCommandRun executes the license command's Run func and asserts it
// prints exactly the embedded license text.
func TestLicenseCommandRun(t *testing.T) {
	out := captureMainStdout(t, func() {
		licenseCmd.Run(licenseCmd, nil)
	})
	if out != meta.License {
		t.Errorf("license output = %q, want %q", out, meta.License)
	}
}

// TestResolveExecuteError_NonUnknownError verifies that a non "unknown command"
// error is mapped straight to an exit code with exit=true.
func TestResolveExecuteError_NonUnknownError(t *testing.T) {
	err := errors.New("connection refused")
	code, exit := resolveExecuteError(context.Background(), []string{"status"}, err)
	if !exit {
		t.Fatal("expected exit=true for non-unknown error")
	}
	if code != cli.ExitCodeFromError(err) {
		t.Errorf("code = %d, want %d", code, cli.ExitCodeFromError(err))
	}
}

// TestResolveExecuteError_UnknownNoArgs returns the error's exit code when no
// args are available to disambiguate.
func TestResolveExecuteError_UnknownNoArgs(t *testing.T) {
	err := errors.New(`unknown command "x" for "kvelmo"`)
	code, exit := resolveExecuteError(context.Background(), nil, err)
	if !exit {
		t.Fatal("expected exit=true")
	}
	if code != cli.ExitCodeFromError(err) {
		t.Errorf("code = %d, want %d", code, cli.ExitCodeFromError(err))
	}
}

// TestResolveExecuteError_UniquePrefix disambiguates a unique prefix to its
// command and runs it. "licen" uniquely matches "license", which prints and
// returns nil, so the function should report exit=false (caller just returns).
func TestResolveExecuteError_UniquePrefix(t *testing.T) {
	err := errors.New(`unknown command "licen" for "kvelmo"`)
	code, exit := resolveExecuteError(context.Background(), []string{"licen"}, err)
	if exit {
		t.Errorf("expected exit=false after successful disambiguation, got exit=true (code %d)", code)
	}
	if code != 0 {
		t.Errorf("code = %d, want 0", code)
	}
}

// TestResolveExecuteError_AmbiguousPrefix returns ExitUsage when the prefix
// matches more than one command. "s" matches start, status, stop, submit, etc.
func TestResolveExecuteError_AmbiguousPrefix(t *testing.T) {
	// Capture stderr write by confirming the ambiguous prefix path returns
	// ExitUsage. We assert against the public exit code.
	err := errors.New(`unknown command "s" for "kvelmo"`)
	code, exit := resolveExecuteError(context.Background(), []string{"s"}, err)
	if !exit {
		t.Fatal("expected exit=true for ambiguous prefix")
	}
	if code != cli.ExitUsage {
		t.Errorf("code = %d, want ExitUsage (%d)", code, cli.ExitUsage)
	}
}

// TestResolveExecuteError_NoMatch falls through to printing the original error
// when the prefix matches nothing.
func TestResolveExecuteError_NoMatch(t *testing.T) {
	err := errors.New(`unknown command "zzzznope" for "kvelmo"`)
	code, exit := resolveExecuteError(context.Background(), []string{"zzzznope"}, err)
	if !exit {
		t.Fatal("expected exit=true")
	}
	if code != cli.ExitCodeFromError(err) {
		t.Errorf("code = %d, want %d", code, cli.ExitCodeFromError(err))
	}
}

// TestDisambiguateAmbiguousFormatsNames sanity-checks our assumption that "s"
// is genuinely ambiguous across the registered command tree (guards the
// AmbiguousPrefix test above from silently degrading into the no-match path).
func TestDisambiguateAmbiguousFormatsNames(t *testing.T) {
	match, names := cli.DisambiguateCommand(rootCmd, "s")
	if match != nil {
		t.Fatalf("expected no unique match for prefix 's', got %q", match.Name())
	}
	if len(names) < 2 {
		t.Fatalf("expected multiple suggestions for 's', got %v", names)
	}
	if !strings.Contains(strings.Join(names, ","), "status") {
		t.Errorf("expected 'status' among suggestions, got %v", names)
	}
}
