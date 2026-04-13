package conductor

import (
	"context"
	"strings"
	"testing"

	"github.com/valksor/kvelmo/internal/ciwatch"
)

func TestBuildCIFixPrompt_ContainsLogs(t *testing.T) {
	prompt := buildCIFixPrompt("Fix login", "Login page broken", "error: test_login FAILED", 1)

	if !strings.Contains(prompt, "Fix login") {
		t.Error("prompt should contain task title")
	}

	if !strings.Contains(prompt, "Login page broken") {
		t.Error("prompt should contain task description")
	}

	if !strings.Contains(prompt, "error: test_login FAILED") {
		t.Error("prompt should contain CI logs")
	}

	if !strings.Contains(prompt, "attempt 1") {
		t.Error("prompt should contain attempt number")
	}
}

func TestBuildCIFixPrompt_AttemptNumber(t *testing.T) {
	prompt := buildCIFixPrompt("T", "D", "logs", 3)

	if !strings.Contains(prompt, "attempt 3") {
		t.Error("prompt should contain attempt 3")
	}
}

func TestBuildCIFixPrompt_Instructions(t *testing.T) {
	prompt := buildCIFixPrompt("T", "D", "logs", 1)

	// Should contain actionable instructions
	if !strings.Contains(prompt, "lint") {
		t.Error("prompt should mention lint failures")
	}

	if !strings.Contains(prompt, "test") {
		t.Error("prompt should mention test failures")
	}

	if !strings.Contains(prompt, "build") {
		t.Error("prompt should mention build failures")
	}
}

func TestDefaultMaxCIFixAttempts(t *testing.T) {
	if defaultMaxCIFixAttempts != 3 {
		t.Errorf("defaultMaxCIFixAttempts = %d, want 3", defaultMaxCIFixAttempts)
	}
}

func TestFailedChecksSummary_NoChecks(t *testing.T) {
	status := &ciwatch.Status{
		State: "failure",
	}

	summary := ciwatch.FailedChecksSummary(status)
	if !strings.Contains(summary, "no individual check failures") {
		t.Errorf("expected fallback message, got %q", summary)
	}
}

func TestFailedChecksSummary_WithFailures(t *testing.T) {
	status := &ciwatch.Status{
		State: "failure",
		Checks: []ciwatch.Check{
			{Name: "lint", Status: "failure", URL: "https://ci.example.com/1"},
			{Name: "test", Status: "failure"},
			{Name: "build", Status: "success"},
		},
	}

	summary := ciwatch.FailedChecksSummary(status)

	if !strings.Contains(summary, "lint") {
		t.Error("summary should contain failing lint check")
	}

	if !strings.Contains(summary, "test") {
		t.Error("summary should contain failing test check")
	}

	if strings.Contains(summary, "build") {
		t.Error("summary should not contain passing build check")
	}

	if !strings.Contains(summary, "https://ci.example.com/1") {
		t.Error("summary should contain check URL")
	}
}

func TestFailedChecksSummary_NilStatus(t *testing.T) {
	summary := ciwatch.FailedChecksSummary(nil)
	if summary != "" {
		t.Errorf("expected empty string for nil status, got %q", summary)
	}
}

func TestExtractCILogs_FallsBackToSummary(t *testing.T) {
	c, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	status := &ciwatch.Status{
		State: "failure",
		Checks: []ciwatch.Check{
			{Name: "test", Status: "failure"},
		},
	}

	// No LogsFetcher (nil) — should fall back to summary
	logs := c.extractCILogs(context.Background(), nil, "PR-1", status)

	if !strings.Contains(logs, "test") {
		t.Errorf("expected summary to contain 'test', got %q", logs)
	}
}
