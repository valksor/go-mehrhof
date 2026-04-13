package git

import (
	"errors"
	"strings"
	"testing"
)

func TestFormatGitError_NotAGitRepository(t *testing.T) {
	err := formatGitError([]string{"status"}, "fatal: not a git repository", errors.New("exit status 128"))
	if err == nil {
		t.Fatal("formatGitError() returned nil, want error")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("error = %q, want to contain 'not a git repository'", err.Error())
	}
}

func TestFormatGitError_AlreadyExists_Checkout(t *testing.T) {
	err := formatGitError([]string{"checkout", "-b", "mybranch"}, "fatal: A branch named 'mybranch' already exists.", errors.New("exit status 128"))
	if err == nil {
		t.Fatal("formatGitError() returned nil")
	}
	if !strings.Contains(err.Error(), "branch already exists:") {
		t.Errorf("error = %q, want to contain 'branch already exists:'", err.Error())
	}
}

func TestFormatGitError_AlreadyExists_NonCheckout(t *testing.T) {
	err := formatGitError([]string{"branch", "mybranch"}, "fatal: A branch named 'mybranch' already exists.", errors.New("exit status 128"))
	if err == nil {
		t.Fatal("formatGitError() returned nil")
	}
	msg := err.Error()
	if !strings.HasPrefix(msg, "already exists:") {
		t.Errorf("error = %q, want prefix 'already exists:'", msg)
	}
	if strings.HasPrefix(msg, "branch already exists:") {
		t.Errorf("error = %q, should NOT start with 'branch already exists:' for non-checkout command", msg)
	}
}

func TestFormatGitError_DidNotMatchAny(t *testing.T) {
	err := formatGitError([]string{"checkout", "nosuchbranch"}, "error: pathspec 'nosuchbranch' did not match any file(s) known to git", errors.New("exit status 1"))
	if err == nil {
		t.Fatal("formatGitError() returned nil")
	}
	if !strings.Contains(err.Error(), "branch or commit not found:") {
		t.Errorf("error = %q, want 'branch or commit not found:'", err.Error())
	}
}

func TestFormatGitError_YourLocalChanges(t *testing.T) {
	err := formatGitError([]string{"checkout", "main"}, "error: Your local changes to the following files would be overwritten", errors.New("exit status 1"))
	if err == nil {
		t.Fatal("formatGitError() returned nil")
	}
	if !strings.Contains(err.Error(), "uncommitted changes would be overwritten") {
		t.Errorf("error = %q, want 'uncommitted changes would be overwritten'", err.Error())
	}
}

func TestFormatGitError_Conflict(t *testing.T) {
	err := formatGitError([]string{"merge"}, "CONFLICT (content): Merge conflict in file.txt", errors.New("exit status 1"))
	if err == nil {
		t.Fatal("formatGitError() returned nil")
	}
	if !strings.Contains(err.Error(), "merge conflict detected") {
		t.Errorf("error = %q, want 'merge conflict detected'", err.Error())
	}
}

func TestFormatGitError_PermissionDenied(t *testing.T) {
	err := formatGitError([]string{"push"}, "Permission denied (publickey)", errors.New("exit status 128"))
	if err == nil {
		t.Fatal("formatGitError() returned nil")
	}
	if !strings.Contains(err.Error(), "permission denied:") {
		t.Errorf("error = %q, want 'permission denied:'", err.Error())
	}
}

func TestFormatGitError_CouldNotResolveHost(t *testing.T) {
	err := formatGitError([]string{"push"}, "fatal: Could not resolve host: github.com", errors.New("exit status 128"))
	if err == nil {
		t.Fatal("formatGitError() returned nil")
	}
	if !strings.Contains(err.Error(), "cannot reach remote server") {
		t.Errorf("error = %q, want 'cannot reach remote server'", err.Error())
	}
}

func TestFormatGitError_AuthenticationFailed(t *testing.T) {
	err := formatGitError([]string{"push"}, "remote: Authentication failed for 'https://github.com/foo/bar.git'", errors.New("exit status 128"))
	if err == nil {
		t.Fatal("formatGitError() returned nil")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("error = %q, want 'authentication failed'", err.Error())
	}
}

func TestFormatGitError_NoSpaceLeftOnDevice(t *testing.T) {
	err := formatGitError([]string{"add", "."}, "error: No space left on device", errors.New("exit status 1"))
	if err == nil {
		t.Fatal("formatGitError() returned nil")
	}
	if !strings.Contains(err.Error(), "disk full") {
		t.Errorf("error = %q, want 'disk full'", err.Error())
	}
}

func TestFormatGitError_ENOSPC(t *testing.T) {
	err := formatGitError([]string{"pack-objects"}, "fatal: ENOSPC, write failed", errors.New("exit status 128"))
	if err == nil {
		t.Fatal("formatGitError() returned nil")
	}
	if !strings.Contains(err.Error(), "disk full") {
		t.Errorf("error = %q, want 'disk full'", err.Error())
	}
}

func TestFormatGitError_NonEmptyStderrNoMatch(t *testing.T) {
	inner := errors.New("exit status 1")
	err := formatGitError([]string{"status"}, "some unexpected git output", inner)
	if err == nil {
		t.Fatal("formatGitError() returned nil")
	}
	if !errors.Is(err, inner) {
		t.Errorf("formatGitError() should wrap inner error via %%w; err = %v", err)
	}
	if !strings.Contains(err.Error(), "some unexpected git output") {
		t.Errorf("error = %q, want stderr included in message", err.Error())
	}
}

func TestFormatGitError_EmptyStderr(t *testing.T) {
	inner := errors.New("exit status 1")
	err := formatGitError([]string{"status"}, "", inner)
	if !errors.Is(err, inner) {
		t.Errorf("formatGitError() with empty stderr = %v, want bare inner error %v", err, inner)
	}
}

func TestParseNameStatusLine_NoTab(t *testing.T) {
	line := "somefilewithnotab"
	path, status := parseNameStatusLine(line)
	if path != line {
		t.Errorf("parseNameStatusLine(%q).path = %q, want %q", line, path, line)
	}
	if status != "modified" {
		t.Errorf("parseNameStatusLine(%q).status = %q, want 'modified'", line, status)
	}
}

func TestParseNameStatusLine_Added(t *testing.T) {
	path, status := parseNameStatusLine("A\tnewfile.go")
	if path != "newfile.go" {
		t.Errorf("path = %q, want 'newfile.go'", path)
	}
	if status != "added" {
		t.Errorf("status = %q, want 'added'", status)
	}
}

func TestParseNameStatusLine_Deleted(t *testing.T) {
	path, status := parseNameStatusLine("D\toldfile.go")
	if path != "oldfile.go" {
		t.Errorf("path = %q, want 'oldfile.go'", path)
	}
	if status != "deleted" {
		t.Errorf("status = %q, want 'deleted'", status)
	}
}

func TestParseNameStatusLine_Renamed(t *testing.T) {
	path, status := parseNameStatusLine("R100\told.go\tnew.go")
	if path != "new.go" {
		t.Errorf("path = %q, want 'new.go'", path)
	}
	if status != "renamed" {
		t.Errorf("status = %q, want 'renamed'", status)
	}
}

func TestParseNameStatusLine_Modified(t *testing.T) {
	path, status := parseNameStatusLine("M\tchanged.go")
	if path != "changed.go" {
		t.Errorf("path = %q, want 'changed.go'", path)
	}
	if status != "modified" {
		t.Errorf("status = %q, want 'modified'", status)
	}
}
