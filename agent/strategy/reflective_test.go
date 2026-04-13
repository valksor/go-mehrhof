package strategy

import (
	"context"
	"testing"
)

func TestReflectiveIsRunner(t *testing.T) {
	r := &Reflective{}
	if !IsRunner(r) {
		t.Error("Reflective should implement Runner")
	}
}

func TestReflectiveName(t *testing.T) {
	r := &Reflective{}
	if r.Name() != "reflective" {
		t.Errorf("expected 'reflective', got %q", r.Name())
	}
}

func TestReflectiveBuildPrompt(t *testing.T) {
	r := &Reflective{}
	result := r.BuildPrompt(Input{Task: "do something", Context: "some context"})

	if result == "" {
		t.Error("expected non-empty prompt")
	}
}

func TestReflectiveRunLoop_AllPassesSucceed(t *testing.T) {
	r := &Reflective{}
	callCount := 0

	exec := func(_ context.Context, _ string) (string, error) {
		callCount++

		switch callCount {
		case 1:
			return "initial result", nil
		case 2:
			return "found a bug: missing error check", nil
		case 3:
			return "added error check", nil
		default:
			return "", nil
		}
	}

	var events []Event

	emit := func(e Event) {
		events = append(events, e)
	}

	output, err := r.RunLoop(context.Background(), exec, Input{Task: "implement feature"}, emit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.Status != "complete" {
		t.Errorf("expected status 'complete', got %q", output.Status)
	}

	if callCount != 3 {
		t.Errorf("expected 3 exec calls (initial + reflection + correction), got %d", callCount)
	}
}

func TestReflectiveRunLoop_NoActionableItems(t *testing.T) {
	r := &Reflective{}
	callCount := 0

	exec := func(_ context.Context, _ string) (string, error) {
		callCount++

		switch callCount {
		case 1:
			return "perfect code", nil
		case 2:
			return "All good, no problems detected", nil
		default:
			return "", nil
		}
	}

	output, err := r.RunLoop(context.Background(), exec, Input{Task: "check code"}, func(Event) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.Status != "complete" {
		t.Errorf("expected status 'complete', got %q", output.Status)
	}

	if callCount != 2 {
		t.Errorf("expected 2 exec calls (initial + reflection, no correction), got %d", callCount)
	}
}

func TestHasActionableItems(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"All checks passed, no problems detected", false},
		{"Everything is correct", false},
		{"There's a bug in the error handling", true},
		{"You should add validation", true},
		{"Missing null check", true},
		{"All good", false},
		{"Looks good, but missing a null check", true}, // action phrases override approval
	}

	for _, tt := range tests {
		got := hasActionableItems(tt.input)
		if got != tt.want {
			t.Errorf("hasActionableItems(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestTruncateForReflection(t *testing.T) {
	short := "short text"
	if truncateForReflection(short) != short {
		t.Error("short text should not be truncated")
	}

	long := make([]byte, 5000)
	for i := range long {
		long[i] = 'x'
	}

	result := truncateForReflection(string(long))
	if len(result) > 4024 { // 4000 + 24 bytes for "\n\n[truncated for review]"
		t.Errorf("expected truncated output, got length %d", len(result))
	}
}

func TestReflectiveNotAutoRegistered(t *testing.T) {
	// Reflective is intentionally NOT auto-registered because RunLoop
	// integration with the conductor is not yet wired. It should not
	// appear in the registry until conductor-side Runner dispatch exists.
	_, ok := Get("reflective")
	if ok {
		t.Error("reflective should NOT be auto-registered until conductor integration exists")
	}
}

func TestDirectIsNotRunner(t *testing.T) {
	d := &Direct{}
	if IsRunner(d) {
		t.Error("Direct should not implement Runner")
	}
}

func TestIterativeIsNotRunner(t *testing.T) {
	it := &Iterative{}
	if IsRunner(it) {
		t.Error("Iterative should not implement Runner")
	}
}
