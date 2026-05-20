package ciwatch

import (
	"strings"
	"testing"
)

func TestFailedChecksSummary_NilStatus(t *testing.T) {
	if got := FailedChecksSummary(nil); got != "" {
		t.Errorf("nil status: got %q", got)
	}
}

func TestFailedChecksSummary_NoFailures(t *testing.T) {
	s := &Status{
		Checks: []Check{
			{Name: "build", Status: "success"},
			{Name: "lint", Status: "pending"},
		},
	}
	got := FailedChecksSummary(s)
	if got == "" {
		t.Fatal("expected fallback message")
	}
	if !strings.Contains(got, "no individual check failures") {
		t.Errorf("got %q", got)
	}
}

func TestFailedChecksSummary_WithFailures(t *testing.T) {
	s := &Status{
		Checks: []Check{
			{Name: "build", Status: "failure", URL: "https://ci.example.com/build/1"},
			{Name: "lint", Status: "failure"},
			{Name: "tests", Status: "success"},
		},
	}
	got := FailedChecksSummary(s)
	if !strings.Contains(got, "build (FAILED)") {
		t.Errorf("missing build entry: %q", got)
	}
	if !strings.Contains(got, "https://ci.example.com/build/1") {
		t.Errorf("missing URL: %q", got)
	}
	if !strings.Contains(got, "lint (FAILED)") {
		t.Errorf("missing lint entry: %q", got)
	}
	if strings.Contains(got, "tests") {
		t.Errorf("success check should not appear: %q", got)
	}
}

func TestNew_DefaultInterval(t *testing.T) {
	w := New(nil, "pr-123", 0)
	if w == nil {
		t.Fatal("New returned nil")
	}
	if w.interval == 0 {
		t.Error("expected default interval to be applied")
	}
}

func TestNew_NegativeInterval(t *testing.T) {
	w := New(nil, "pr-123", -5)
	if w.interval <= 0 {
		t.Error("expected positive default interval, got non-positive")
	}
}

func TestNew_ExplicitInterval(t *testing.T) {
	w := New(nil, "pr-123", 10)
	if w.interval != 10 {
		t.Errorf("interval = %v, want 10", w.interval)
	}
}
