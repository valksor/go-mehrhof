package commands

import (
	"strings"
	"testing"
)

// resetQuickFlags clears the package-level quick flags between tests.
func resetQuickFlags(t *testing.T) {
	t.Helper()
	origText, origSource := quickText, quickSource
	t.Cleanup(func() { quickText, quickSource = origText, origSource })
	quickText, quickSource = "", ""
}

// --- quick: read inline text from stdin (quickText == "-") ---

func TestRunQuick_StdinText(t *testing.T) {
	resetQuickFlags(t)
	quickText = "-"

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("start", map[string]any{"task_id": "t1", "state": "loaded"})

	feedStdin(t, "fix the bug\n")

	out := captureStdout(t, func() {
		if err := runQuick(QuickCmd, nil); err != nil {
			t.Errorf("runQuick stdin: %v", err)
		}
	})
	if !strings.Contains(out, "Quick mode") {
		t.Errorf("quick stdin output:\n%s", out)
	}
}

// --- quick: start response with non-string state -> "task loaded" fallback ---

func TestRunQuick_StateNotString(t *testing.T) {
	resetQuickFlags(t)
	quickText = "do thing"

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	// state is a number, not a string -> the inner else ("task loaded") runs.
	stub.SetResponse("start", map[string]any{"task_id": "t1", "state": 123})

	out := captureStdout(t, func() {
		if err := runQuick(QuickCmd, nil); err != nil {
			t.Errorf("runQuick state-not-string: %v", err)
		}
	})
	if !strings.Contains(out, "Quick mode: task loaded") {
		t.Errorf("quick state-not-string output:\n%s", out)
	}
}

// --- quick: start response is a scalar -> outer unmarshal-fallback ---

func TestRunQuick_UnmarshalFallback(t *testing.T) {
	resetQuickFlags(t)
	quickText = "do other thing"

	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("start", "scalar-not-an-object")

	out := captureStdout(t, func() {
		if err := runQuick(QuickCmd, nil); err != nil {
			t.Errorf("runQuick unmarshal-fallback: %v", err)
		}
	})
	if !strings.Contains(out, "Quick mode: task loaded") {
		t.Errorf("quick unmarshal-fallback output:\n%s", out)
	}
}

// --- quick: mutual exclusion of --text and --from ---

func TestRunQuick_MutualExclusionFill(t *testing.T) {
	resetQuickFlags(t)
	quickText, quickSource = "x", "y"

	shortKvelmoHome(t)
	chdirToShortTemp(t)

	if err := runQuick(QuickCmd, nil); err == nil {
		t.Fatal("expected error: --text and --from are mutually exclusive")
	}
}

// --- checkpoints: entry without a message -> short-form line ---

func TestRunCheckpoints_NoMessage(t *testing.T) {
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("checkpoints", map[string]any{
		"checkpoints": []any{
			map[string]any{"sha": "aaaa1111bbbb", "message": "first commit"},
			map[string]any{"sha": "cccc2222dddd"}, // no message -> else branch
		},
		"redo_stack": []any{map[string]any{"sha": "eeee"}},
	})

	out := captureStdout(t, func() {
		if err := runCheckpoints(CheckpointsCmd, nil); err != nil {
			t.Errorf("runCheckpoints no-message: %v", err)
		}
	})
	if !strings.Contains(out, "cccc2222") || !strings.Contains(out, "Redo stack") {
		t.Errorf("checkpoints no-message output:\n%s", out)
	}
}

// --- screenshots capture: no --step -> step omitted from params ---

func TestRunScreenshotsCapture_NoStep(t *testing.T) {
	if err := screenshotsCaptureCmd.Flags().Set("step", ""); err != nil {
		t.Fatal(err)
	}
	shortKvelmoHome(t)
	chdirToShortTemp(t)
	stub := startStubWorktreeSocket(t)
	stub.SetResponse("screenshots.capture", map[string]any{
		"screenshot": map[string]any{"id": "nostep", "filename": "n.png", "source": "system"},
	})

	out := captureStdout(t, func() {
		if err := runScreenshotsCapture(screenshotsCaptureCmd, nil); err != nil {
			t.Errorf("runScreenshotsCapture no-step: %v", err)
		}
	})
	if !strings.Contains(out, "nostep") {
		t.Errorf("screenshots capture no-step output:\n%s", out)
	}
}
