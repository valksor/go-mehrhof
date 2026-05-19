// Package recorder provides recording and replay of agent interactions.
package recorder

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Direction indicates whether a record is inbound or outbound.
type Direction string

const (
	// Inbound is a prompt sent to the agent.
	Inbound Direction = "in"
	// Outbound is an event received from the agent.
	Outbound Direction = "out"
)

// safeIDPattern validates safe characters for jobID and agent names.
var safeIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Record represents a single recorded message.
type Record struct {
	Timestamp time.Time       `json:"timestamp"`
	JobID     string          `json:"job_id"`
	Direction Direction       `json:"direction"`
	Type      string          `json:"type,omitempty"`
	Event     json.RawMessage `json:"event"`
}

// Header contains metadata about the recording session.
type Header struct {
	JobID     string    `json:"job_id"`
	Agent     string    `json:"agent"`
	Model     string    `json:"model,omitempty"`
	WorkDir   string    `json:"work_dir,omitempty"`
	StartedAt time.Time `json:"started_at"`
}

// Recorder records agent interactions to JSONL files.
type Recorder struct {
	dir       string
	maxLines  int
	jobID     string
	agent     string
	model     string
	workDir   string
	startedAt time.Time

	mu        sync.Mutex
	file      *os.File
	writer    *bufio.Writer
	lineCount int
	fileCount int
	filePath  string
	sanitizer *Sanitizer
}

// Config holds recorder configuration.
type Config struct {
	// Dir is the directory for recordings. Defaults to ~/.valksor/kvelmo/recordings.
	Dir string
	// MaxLines is the maximum lines per file before rotation. Defaults to 100000.
	MaxLines int
	// JobID identifies the job being recorded.
	JobID string
	// Agent is the agent name (e.g., "claude", "codex").
	Agent string
	// Model is the model being used (optional).
	Model string
	// WorkDir is the working directory (optional).
	WorkDir string
	// Sanitizer masks secrets in recorded data before writing to disk.
	Sanitizer *Sanitizer
}

// DefaultConfig returns default configuration.
func DefaultConfig() Config {
	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		// Fall back to temp directory if home dir unavailable
		homeDir = os.TempDir()
	}

	return Config{
		Dir:      filepath.Join(homeDir, ".valksor", "kvelmo", "recordings"),
		MaxLines: 100000,
	}
}

// sanitizeFilename replaces unsafe characters in filenames.
func sanitizeFilename(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r < 32 {
			return '_'
		}

		return r
	}, s)
}

// New creates a new recorder with the given configuration.
func New(cfg Config) (*Recorder, error) {
	if cfg.Dir == "" {
		cfg.Dir = DefaultConfig().Dir
	}
	if cfg.MaxLines <= 0 {
		cfg.MaxLines = DefaultConfig().MaxLines
	}
	if cfg.JobID == "" {
		return nil, errors.New("job ID is required")
	}

	// Validate jobID and agent contain only safe characters
	if !safeIDPattern.MatchString(cfg.JobID) {
		return nil, errors.New("invalid job ID: must contain only alphanumerics, dashes, underscores")
	}
	if cfg.Agent != "" && !safeIDPattern.MatchString(cfg.Agent) {
		return nil, errors.New("invalid agent name: must contain only alphanumerics, dashes, underscores")
	}

	if err := os.MkdirAll(cfg.Dir, 0o750); err != nil {
		return nil, fmt.Errorf("create recordings dir: %w", err)
	}

	r := &Recorder{
		dir:       cfg.Dir,
		maxLines:  cfg.MaxLines,
		jobID:     sanitizeFilename(cfg.JobID),
		agent:     sanitizeFilename(cfg.Agent),
		model:     cfg.Model,
		workDir:   cfg.WorkDir,
		startedAt: time.Now(),
		sanitizer: cfg.Sanitizer,
	}

	if err := r.openFile(); err != nil {
		return nil, err
	}

	if err := r.writeHeader(); err != nil {
		_ = r.Close()

		return nil, err
	}

	return r, nil
}

func (r *Recorder) openFile() error {
	filename := fmt.Sprintf(
		"%s_%s_%d_%d.jsonl",
		r.jobID,
		r.agent,
		r.startedAt.UnixNano(),
		r.fileCount,
	)
	path := filepath.Join(r.dir, filename)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
	if err != nil {
		return fmt.Errorf("open recording file: %w", err)
	}

	r.file = f
	r.writer = bufio.NewWriter(f)
	r.filePath = path
	r.lineCount = 0

	return nil
}

func (r *Recorder) writeHeader() error {
	header := Header{
		JobID:     r.jobID,
		Agent:     r.agent,
		Model:     r.model,
		WorkDir:   r.workDir,
		StartedAt: r.startedAt,
	}

	data, err := json.Marshal(header)
	if err != nil {
		return fmt.Errorf("marshal header: %w", err)
	}

	if _, err := r.writer.Write(data); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	if err := r.writer.WriteByte('\n'); err != nil {
		return fmt.Errorf("write header newline: %w", err)
	}
	r.lineCount++

	return nil
}

// Record writes a record to the file.
func (r *Recorder) Record(rec Record) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.file == nil {
		return errors.New("recorder is closed")
	}

	// Rotate if needed
	if r.lineCount >= r.maxLines {
		if err := r.rotate(); err != nil {
			return err
		}
	}

	if r.sanitizer != nil {
		rec.Event = json.RawMessage(r.sanitizer.Sanitize([]byte(rec.Event)))
	}

	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal record: %w", err)
	}

	if _, err := r.writer.Write(data); err != nil {
		return fmt.Errorf("write record: %w", err)
	}
	if err := r.writer.WriteByte('\n'); err != nil {
		return fmt.Errorf("write record newline: %w", err)
	}
	r.lineCount++

	return nil
}

// RecordInbound records an inbound message (prompt to agent).
// Sanitization is handled centrally by Record().
func (r *Recorder) RecordInbound(prompt string) error {
	//nolint:errchkjson // map[string]string cannot fail to marshal
	event, _ := json.Marshal(map[string]string{"prompt": prompt})

	return r.Record(Record{
		Timestamp: time.Now(),
		JobID:     r.jobID,
		Direction: Inbound,
		Type:      "prompt",
		Event:     event,
	})
}

// RecordOutbound records an outbound event (from agent).
// Sanitization is handled centrally by Record().
func (r *Recorder) RecordOutbound(eventType string, event any) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	return r.Record(Record{
		Timestamp: time.Now(),
		JobID:     r.jobID,
		Direction: Outbound,
		Type:      eventType,
		Event:     data,
	})
}

func (r *Recorder) rotate() error {
	// Flush and close current file
	if err := r.writer.Flush(); err != nil {
		return fmt.Errorf("flush before rotate: %w", err)
	}
	if err := r.file.Close(); err != nil {
		// Reset state on failure to prevent use of closed file
		r.file = nil
		r.writer = nil

		return fmt.Errorf("close before rotate: %w", err)
	}

	// Reset before opening new file
	r.file = nil
	r.writer = nil

	// Open new file (increment fileCount only after success)
	r.fileCount++
	if err := r.openFile(); err != nil {
		r.fileCount-- // Revert increment on failure

		return err
	}

	// Write header to new file
	return r.writeHeader()
}

// Flush writes buffered data to disk.
func (r *Recorder) Flush() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.writer == nil {
		return nil
	}

	return r.writer.Flush()
}

// Close flushes and closes the recorder.
func (r *Recorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.writer != nil {
		if err := r.writer.Flush(); err != nil {
			return fmt.Errorf("flush on close: %w", err)
		}
		r.writer = nil
	}
	if r.file != nil {
		if err := r.file.Close(); err != nil {
			return fmt.Errorf("close file: %w", err)
		}
		r.file = nil
	}

	return nil
}

// Path returns the current file path.
func (r *Recorder) Path() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.filePath
}

// LineCount returns the current line count.
func (r *Recorder) LineCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.lineCount
}

// Scratchpad builds a structured scratchpad from the recorder's saved records.
func (r *Recorder) Scratchpad(phase string) (*Scratchpad, error) {
	if err := r.Flush(); err != nil {
		return nil, fmt.Errorf("flush before scratchpad: %w", err)
	}

	records, err := ReadAll(r.Path())
	if err != nil {
		return nil, fmt.Errorf("read records: %w", err)
	}

	return BuildScratchpad(r.jobID, phase, records), nil
}
