package claudemcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteSystemPromptDefault(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "system-prompt.md")

	if err := writeSystemPrompt(dest, "", "task-xyz", "plan", "/repo", false); err != nil {
		t.Fatalf("writeSystemPrompt: %v", err)
	}

	content, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	s := string(content)
	for _, want := range []string{
		"kvelmo orchestration session",
		"task-xyz",
		"plan",
		"/repo",
		"kvelmo_signal_complete",
		"kvelmo_signal_failure",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("system prompt missing %q\n---\n%s", want, s)
		}
	}
}

func TestWriteSystemPromptStandalone(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "system-prompt.md")

	if err := writeSystemPrompt(dest, "", "task-xyz", "plan", "/repo", true); err != nil {
		t.Fatalf("writeSystemPrompt: %v", err)
	}

	content, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	s := string(content)
	if !strings.Contains(s, "one-shot session") || !strings.Contains(s, "kvelmo_signal_complete") {
		t.Errorf("standalone prompt missing expected text\n---\n%s", s)
	}
	// Standalone must NOT instruct the orchestration workflow tools.
	if strings.Contains(s, "kvelmo orchestration session") {
		t.Errorf("standalone prompt should not be the orchestration template\n---\n%s", s)
	}
}

func TestWriteSystemPromptOverride(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "system-prompt.md")
	custom := "custom prompt body"
	if err := writeSystemPrompt(dest, custom, "x", "y", "z", false); err != nil {
		t.Fatalf("writeSystemPrompt: %v", err)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != custom {
		t.Errorf("got %q, want %q", got, custom)
	}
}
