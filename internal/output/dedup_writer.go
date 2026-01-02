package output

import (
	"bytes"
	"io"
	"sync"
)

// DeduplicatingWriter wraps an io.Writer and suppresses consecutive identical lines.
// It buffers partial lines until a newline is received, then compares each complete
// line against the previous line. Only lines that differ from the previous are written.
type DeduplicatingWriter struct {
	mu         sync.Mutex
	w          io.Writer
	lastLine   string
	hasWritten bool // Tracks if we've written at least one line
	buffer     bytes.Buffer
}

// NewDeduplicatingWriter creates a new DeduplicatingWriter that wraps the given writer.
func NewDeduplicatingWriter(w io.Writer) *DeduplicatingWriter {
	return &DeduplicatingWriter{w: w}
}

// Write implements io.Writer. It buffers input until complete lines are available,
// then writes only lines that differ from the previous line.
func (d *DeduplicatingWriter) Write(p []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Track original length for return value
	originalLen := len(p)

	// Append to buffer
	d.buffer.Write(p)

	// Track if we processed any complete line (for auto-flush decision below)
	processedCompleteLine := false

	// Process complete lines
	for {
		line, err := d.buffer.ReadString('\n')
		if err != nil {
			// No complete line yet, put back in buffer
			d.buffer.WriteString(line)

			break
		}

		// We have a complete line (including \n)
		// Compare without trailing newline for dedup, but write with it
		lineContent := line[:len(line)-1] // Remove \n for comparison

		if !d.hasWritten || lineContent != d.lastLine {
			// First line or different from last - write it
			_, writeErr := d.w.Write([]byte(line))
			if writeErr != nil {
				return 0, writeErr
			}
			d.lastLine = lineContent
			d.hasWritten = true
		}
		// If same as last line, skip (deduplicate)
		processedCompleteLine = true
	}

	// If we processed at least one complete line and have remaining buffer,
	// auto-flush the partial (it's a trailing partial from a multi-line write).
	if processedCompleteLine && d.buffer.Len() > 0 {
		remaining := d.buffer.String()
		d.buffer.Reset()

		// Write remaining partial if different from last line
		if !d.hasWritten || remaining != d.lastLine {
			// Append newline when auto-flushing partial line
			_, err := d.w.Write([]byte(remaining + "\n"))
			if err != nil {
				return 0, err
			}
			d.lastLine = remaining
			d.hasWritten = true
		}
	}

	return originalLen, nil
}

// Flush writes any remaining buffered content to the underlying writer.
// Call this when done writing to ensure no content is lost.
func (d *DeduplicatingWriter) Flush() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.buffer.Len() > 0 {
		remaining := d.buffer.String()
		d.buffer.Reset()

		// Write remaining content without dedup (partial line)
		if !d.hasWritten || remaining != d.lastLine {
			_, err := d.w.Write([]byte(remaining))
			if err != nil {
				return err
			}
			d.lastLine = remaining
			d.hasWritten = true
		}
	}

	return nil
}

// Reset clears the deduplication state, allowing the next line to always be written.
func (d *DeduplicatingWriter) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.lastLine = ""
	d.hasWritten = false
	d.buffer.Reset()
}
