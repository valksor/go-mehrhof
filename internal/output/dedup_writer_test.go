package output

import (
	"bytes"
	"strings"
	"sync"
	"testing"
)

func TestDeduplicatingWriter_BasicDedup(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		writes   []string
	}{
		{
			name:     "no duplicates",
			writes:   []string{"line1\n", "line2\n", "line3\n"},
			expected: "line1\nline2\nline3\n",
		},
		{
			name:     "consecutive duplicates",
			writes:   []string{"same\n", "same\n", "same\n"},
			expected: "same\n",
		},
		{
			name:     "non-consecutive duplicates pass through",
			writes:   []string{"a\n", "b\n", "a\n"},
			expected: "a\nb\na\n",
		},
		{
			name:     "mixed duplicates",
			writes:   []string{"x\n", "x\n", "y\n", "y\n", "x\n"},
			expected: "x\ny\nx\n",
		},
		{
			name:     "empty lines deduplicated",
			writes:   []string{"\n", "\n", "text\n", "\n"},
			expected: "\ntext\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			w := NewDeduplicatingWriter(&buf)

			for _, s := range tt.writes {
				_, err := w.Write([]byte(s))
				if err != nil {
					t.Fatalf("Write error: %v", err)
				}
			}

			if got := buf.String(); got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestDeduplicatingWriter_PartialLines(t *testing.T) {
	var buf bytes.Buffer
	w := NewDeduplicatingWriter(&buf)

	// Write partial line
	if _, err := w.Write([]byte("hel")); err != nil {
		t.Errorf("Write error: %v", err)
	}
	if _, err := w.Write([]byte("lo\n")); err != nil {
		t.Errorf("Write error: %v", err)
	}

	// Write same line in parts
	if _, err := w.Write([]byte("hel")); err != nil {
		t.Errorf("Write error: %v", err)
	}
	if _, err := w.Write([]byte("lo\n")); err != nil {
		t.Errorf("Write error: %v", err)
	}

	// Should deduplicate
	expected := "hello\n"
	if got := buf.String(); got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestDeduplicatingWriter_MultiLineWrite(t *testing.T) {
	var buf bytes.Buffer
	w := NewDeduplicatingWriter(&buf)

	// Single write with multiple lines including duplicates
	if _, err := w.Write([]byte("a\nb")); err != nil {
		t.Errorf("Write error: %v", err)
	}

	expected := "a\nb\n"
	if got := buf.String(); got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestDeduplicatingWriter_Flush(t *testing.T) {
	var buf bytes.Buffer
	w := NewDeduplicatingWriter(&buf)

	// Write partial line without newline
	if _, err := w.Write([]byte("partial")); err != nil {
		t.Errorf("Write error: %v", err)
	}

	// Nothing written yet
	if buf.Len() != 0 {
		t.Errorf("expected empty buffer before flush, got %q", buf.String())
	}

	// Flush should write partial
	err := w.Flush()
	if err != nil {
		t.Fatalf("Flush error: %v", err)
	}

	if got := buf.String(); got != "partial" {
		t.Errorf("after flush: got %q, want %q", got, "partial")
	}
}

func TestDeduplicatingWriter_FlushDedup(t *testing.T) {
	var buf bytes.Buffer
	w := NewDeduplicatingWriter(&buf)

	// Write complete line
	if _, err := w.Write([]byte("line\n")); err != nil {
		t.Errorf("Write error: %v", err)
	}

	// Try to flush same content (without newline)
	if _, err := w.Write([]byte("line")); err != nil {
		t.Errorf("Write error: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Errorf("Flush error: %v", err)
	}

	// Should deduplicate
	expected := "line\n"
	if got := buf.String(); got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestDeduplicatingWriter_Reset(t *testing.T) {
	var buf bytes.Buffer
	w := NewDeduplicatingWriter(&buf)

	if _, err := w.Write([]byte("line\n")); err != nil {
		t.Errorf("Write error: %v", err)
	}
	if _, err := w.Write([]byte("line\n")); err != nil {
		t.Errorf("Write error: %v", err)
	} // Deduplicated

	w.Reset()

	if _, err := w.Write([]byte("line\n")); err != nil {
		t.Errorf("Write error: %v", err)
	} // Should write after reset

	expected := "line\nline\n"
	if got := buf.String(); got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestDeduplicatingWriter_ThreadSafety(t *testing.T) {
	var buf bytes.Buffer
	w := NewDeduplicatingWriter(&buf)

	var wg sync.WaitGroup
	iterations := 100

	// Concurrent writes
	for range 10 {
		wg.Go(func() {
			for range iterations {
				if _, err := w.Write([]byte("concurrent\n")); err != nil {
					t.Errorf("concurrent write error: %v", err)
				}
			}
		})
	}

	wg.Wait()

	// With deduplication, we should have far fewer lines than 1000
	// The exact count depends on timing, but it should be at least 1
	output := buf.String()
	if len(output) == 0 {
		t.Error("expected some output after concurrent writes")
	}

	// Count lines
	lines := strings.Count(output, "\n")
	if lines >= 10*iterations {
		t.Errorf("deduplication not working: got %d lines, expected much fewer than %d", lines, 10*iterations)
	}
}

func TestDeduplicatingWriter_ReturnValue(t *testing.T) {
	var buf bytes.Buffer
	w := NewDeduplicatingWriter(&buf)

	// Write should return the original input length, not the written length
	input := "duplicate\nduplicate\n"
	n, err := w.Write([]byte(input))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}

	if n != len(input) {
		t.Errorf("Write returned %d, want %d", n, len(input))
	}
}
