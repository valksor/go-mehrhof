package log

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestConfigureDefault(t *testing.T) {
	// Just ensure Configure doesn't panic
	Configure(Options{})

	logger := Logger()
	if logger == nil {
		t.Error("Logger should not be nil after Configure")
	}
}

func TestConfigureWithOutput(t *testing.T) {
	var buf bytes.Buffer
	Configure(Options{
		Output: &buf,
		Level:  LevelInfo,
	})

	Info("test message")

	if buf.Len() == 0 {
		t.Error("expected log output")
	}
	if !strings.Contains(buf.String(), "test message") {
		t.Errorf("log output = %q, want to contain %q", buf.String(), "test message")
	}
}

func TestConfigureJSON(t *testing.T) {
	var buf bytes.Buffer
	Configure(Options{
		Output: &buf,
		JSON:   true,
		Level:  LevelInfo,
	})

	Info("json test")

	if !strings.Contains(buf.String(), "{") {
		t.Error("expected JSON output")
	}
}

func TestConfigureVerbose(t *testing.T) {
	var buf bytes.Buffer
	Configure(Options{
		Output:  &buf,
		Verbose: true, // Should enable debug level
	})

	Debug("debug message")

	if !strings.Contains(buf.String(), "debug message") {
		t.Error("debug should be visible with Verbose=true")
	}
}

func TestSetLevel(t *testing.T) {
	var buf bytes.Buffer
	Configure(Options{
		Output: &buf,
		Level:  LevelError, // Only errors
	})

	buf.Reset()
	Info("should not appear")
	if buf.Len() > 0 {
		t.Error("info should not appear at error level")
	}

	Error("should appear")
	if !strings.Contains(buf.String(), "should appear") {
		t.Error("error should appear")
	}
}

func TestEnableDebug(t *testing.T) {
	var buf bytes.Buffer
	Configure(Options{
		Output: &buf,
		Level:  LevelInfo,
	})

	buf.Reset()
	Debug("before enable")
	if strings.Contains(buf.String(), "before enable") {
		t.Error("debug should not appear before EnableDebug")
	}

	EnableDebug()
	Configure(Options{Output: &buf, Level: LevelDebug})

	Debug("after enable")
	if !strings.Contains(buf.String(), "after enable") {
		t.Error("debug should appear after EnableDebug")
	}
}

func TestLogger(t *testing.T) {
	logger := Logger()
	if logger == nil {
		t.Error("Logger() should not return nil")
	}
}

func TestWith(t *testing.T) {
	var buf bytes.Buffer
	Configure(Options{
		Output: &buf,
		Level:  LevelInfo,
	})

	withLogger := With("key", "value")
	if withLogger == nil {
		t.Error("With() should not return nil")
	}

	withLogger.Info("with attributes")
	if !strings.Contains(buf.String(), "key") || !strings.Contains(buf.String(), "value") {
		t.Error("attributes should appear in output")
	}
}

func TestDebug(t *testing.T) {
	var buf bytes.Buffer
	Configure(Options{
		Output: &buf,
		Level:  LevelDebug,
	})

	Debug("debug msg", "attr", "val")
	if !strings.Contains(buf.String(), "debug msg") {
		t.Error("Debug() should log message")
	}
}

func TestInfo(t *testing.T) {
	var buf bytes.Buffer
	Configure(Options{
		Output: &buf,
		Level:  LevelInfo,
	})

	Info("info msg")
	if !strings.Contains(buf.String(), "info msg") {
		t.Error("Info() should log message")
	}
}

func TestWarn(t *testing.T) {
	var buf bytes.Buffer
	Configure(Options{
		Output: &buf,
		Level:  LevelWarn,
	})

	Warn("warn msg")
	if !strings.Contains(buf.String(), "warn msg") {
		t.Error("Warn() should log message")
	}
}

func TestError(t *testing.T) {
	var buf bytes.Buffer
	Configure(Options{
		Output: &buf,
		Level:  LevelError,
	})

	Error("error msg")
	if !strings.Contains(buf.String(), "error msg") {
		t.Error("Error() should log message")
	}
}

func TestContextLogging(t *testing.T) {
	var buf bytes.Buffer
	Configure(Options{
		Output: &buf,
		Level:  LevelDebug,
	})

	ctx := context.Background()

	DebugContext(ctx, "debug ctx")
	if !strings.Contains(buf.String(), "debug ctx") {
		t.Error("DebugContext should log")
	}

	buf.Reset()
	InfoContext(ctx, "info ctx")
	if !strings.Contains(buf.String(), "info ctx") {
		t.Error("InfoContext should log")
	}

	buf.Reset()
	WarnContext(ctx, "warn ctx")
	if !strings.Contains(buf.String(), "warn ctx") {
		t.Error("WarnContext should log")
	}

	buf.Reset()
	ErrorContext(ctx, "error ctx")
	if !strings.Contains(buf.String(), "error ctx") {
		t.Error("ErrorContext should log")
	}
}

func TestErr(t *testing.T) {
	attr := Err(errors.New("test error"))
	if attr.Key != "error" {
		t.Errorf("Err().Key = %q, want %q", attr.Key, "error")
	}
}

func TestTaskID(t *testing.T) {
	attr := TaskID("task-123")
	if attr.Key != "task_id" {
		t.Errorf("TaskID().Key = %q, want %q", attr.Key, "task_id")
	}
	if attr.Value.String() != "task-123" {
		t.Errorf("TaskID().Value = %q, want %q", attr.Value.String(), "task-123")
	}
}

func TestState(t *testing.T) {
	attrs := State("idle", "planning")
	if len(attrs) != 2 {
		t.Errorf("State() returned %d attrs, want 2", len(attrs))
	}

	foundFrom, foundTo := false, false
	for _, attr := range attrs {
		if attr.Key == "from" && attr.Value.String() == "idle" {
			foundFrom = true
		}
		if attr.Key == "to" && attr.Value.String() == "planning" {
			foundTo = true
		}
	}

	if !foundFrom {
		t.Error("State() missing 'from' attribute")
	}
	if !foundTo {
		t.Error("State() missing 'to' attribute")
	}
}

func TestLevelConstants(t *testing.T) {
	// Verify level constants match slog levels
	if LevelDebug != -4 {
		t.Errorf("LevelDebug = %d, want -4", LevelDebug)
	}
	if LevelInfo != 0 {
		t.Errorf("LevelInfo = %d, want 0", LevelInfo)
	}
	if LevelWarn != 4 {
		t.Errorf("LevelWarn = %d, want 4", LevelWarn)
	}
	if LevelError != 8 {
		t.Errorf("LevelError = %d, want 8", LevelError)
	}
}
