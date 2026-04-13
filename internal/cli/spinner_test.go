package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSpinner_StartStop(t *testing.T) {
	s := NewSpinner("Testing...")

	// Start should set running to true
	s.Start()

	// Give animation goroutine time to start
	time.Sleep(10 * time.Millisecond)

	s.mu.Lock()
	wasRunning := s.running
	s.mu.Unlock()

	// Stop should clean up
	s.Stop()

	s.mu.Lock()
	isRunning := s.running
	s.mu.Unlock()

	// In non-TTY test environment, running may not be set
	// Just verify Stop doesn't panic and sets running to false
	if isRunning {
		t.Error("spinner still running after Stop()")
	}

	// Verify double-stop is safe
	s.Stop()

	_ = wasRunning // May or may not be true depending on TTY detection
}

func TestSpinner_Success(t *testing.T) {
	// Use t.TempDir() which auto-cleans; create file inside
	tmpFile, err := os.CreateTemp(t.TempDir(), "spinner-test-*")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}

	s := NewSpinner("Working...")
	s.writer = tmpFile

	s.Success("Done!")

	// Read what was written
	if _, err := tmpFile.Seek(0, 0); err != nil {
		t.Fatalf("seek: %v", err)
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(tmpFile); err != nil {
		t.Fatalf("read: %v", err)
	}
	output := buf.String()

	// Should contain checkmark and message (ANSI codes stripped for comparison)
	if !strings.Contains(output, "✓") {
		t.Errorf("expected checkmark in output, got: %q", output)
	}
	if !strings.Contains(output, "Done!") {
		t.Errorf("expected 'Done!' in output, got: %q", output)
	}
}

func TestSpinner_Fail(t *testing.T) {
	tmpFile, err := os.CreateTemp(t.TempDir(), "spinner-test-*")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}

	s := NewSpinner("Working...")
	s.writer = tmpFile

	s.Fail("Error occurred")

	if _, err := tmpFile.Seek(0, 0); err != nil {
		t.Fatalf("seek: %v", err)
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(tmpFile); err != nil {
		t.Fatalf("read: %v", err)
	}
	output := buf.String()

	if !strings.Contains(output, "✗") {
		t.Errorf("expected X mark in output, got: %q", output)
	}
	if !strings.Contains(output, "Error occurred") {
		t.Errorf("expected error message in output, got: %q", output)
	}
}

func TestSpinner_NonTTY(t *testing.T) {
	// In a non-TTY environment (like tests), Start should print message once
	// and not animate

	tmpFile, err := os.CreateTemp(t.TempDir(), "spinner-test-*")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}

	s := NewSpinner("Loading...")
	s.writer = tmpFile

	s.Start()
	time.Sleep(50 * time.Millisecond)
	s.Stop()

	if _, err := tmpFile.Seek(0, 0); err != nil {
		t.Fatalf("seek: %v", err)
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(tmpFile); err != nil {
		t.Fatalf("read: %v", err)
	}
	output := buf.String()

	// Non-TTY should just print the message with newline, no animation
	if !strings.Contains(output, "Loading...") {
		t.Errorf("expected message in non-TTY output, got: %q", output)
	}
}

func TestSpinner_DoubleStart(t *testing.T) {
	s := NewSpinner("Test")

	// Double start should be safe
	s.Start()
	s.Start()
	s.Stop()
}

func TestSpinner_StopWithoutStart(t *testing.T) {
	s := NewSpinner("Test")

	// Stop without start should be safe
	s.Stop()
	s.Success("OK")
}
