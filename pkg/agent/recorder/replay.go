package recorder

import (
	"bufio"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Reader reads recording files.
type Reader struct {
	file    *os.File
	scanner *bufio.Scanner
	header  *Header
}

// OpenReader opens a recording file for reading.
func OpenReader(path string) (*Reader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open recording: %w", err)
	}

	scanner := bufio.NewScanner(f)
	// Support lines up to 1MB
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	r := &Reader{
		file:    f,
		scanner: scanner,
	}

	// Read and validate header (first line is required)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			_ = f.Close()

			return nil, fmt.Errorf("read header: %w", err)
		}
		_ = f.Close()

		return nil, errors.New("recording file is empty")
	}

	var h Header
	if err := json.Unmarshal(scanner.Bytes(), &h); err != nil {
		_ = f.Close()

		return nil, fmt.Errorf("parse header: %w", err)
	}
	r.header = &h

	return r, nil
}

// Header returns the recording header, if present.
func (r *Reader) Header() *Header {
	return r.header
}

// Next reads the next record. Returns nil, nil at EOF.
func (r *Reader) Next() (*Record, error) {
	if !r.scanner.Scan() {
		if err := r.scanner.Err(); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}

		//nolint:nilnil // EOF is signaled by nil, nil - documented pattern
		return nil, nil
	}

	var rec Record
	if err := json.Unmarshal(r.scanner.Bytes(), &rec); err != nil {
		return nil, fmt.Errorf("unmarshal record: %w", err)
	}

	return &rec, nil
}

// Close closes the reader.
func (r *Reader) Close() error {
	return r.file.Close()
}

// ReadAll reads all records from a file.
func ReadAll(path string) ([]Record, error) {
	r, err := OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.Close() }()

	var records []Record
	for {
		rec, err := r.Next()
		if err != nil {
			return nil, err
		}
		if rec == nil {
			break
		}
		records = append(records, *rec)
	}

	return records, nil
}

// RecordingInfo contains metadata about a recording file.
type RecordingInfo struct {
	Path      string `json:"path"`
	JobID     string `json:"job_id"`
	Agent     string `json:"agent"`
	Model     string `json:"model,omitempty"`
	StartedAt string `json:"started_at"`
	Lines     int    `json:"lines"`
}

// ListRecordings lists all recordings in a directory.
func ListRecordings(dir string) ([]RecordingInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("read dir: %w", err)
	}

	var infos []RecordingInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		info, err := getRecordingInfo(path)
		if err != nil {
			continue // Skip malformed files
		}
		infos = append(infos, info)
	}

	// Sort by path (which includes timestamp)
	slices.SortFunc(infos, func(a, b RecordingInfo) int {
		return cmp.Compare(b.Path, a.Path) // Newest first
	})

	return infos, nil
}

func getRecordingInfo(path string) (RecordingInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return RecordingInfo{}, err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	// Use same buffer size as OpenReader (1MB) to handle long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	lines := 0
	var header Header

	for scanner.Scan() {
		if lines == 0 {
			// First line is header
			if err := json.Unmarshal(scanner.Bytes(), &header); err != nil {
				return RecordingInfo{}, err
			}
		}
		lines++
	}

	if err := scanner.Err(); err != nil {
		return RecordingInfo{}, fmt.Errorf("scan recording: %w", err)
	}

	return RecordingInfo{
		Path:      path,
		JobID:     header.JobID,
		Agent:     header.Agent,
		Model:     header.Model,
		StartedAt: header.StartedAt.Format(time.RFC3339),
		Lines:     lines,
	}, nil
}

// ListSessionRecordings lists recordings for a specific job.
func ListSessionRecordings(dir, jobID string) ([]RecordingInfo, error) {
	all, err := ListRecordings(dir)
	if err != nil {
		return nil, err
	}

	var filtered []RecordingInfo
	for _, info := range all {
		if info.JobID == jobID {
			filtered = append(filtered, info)
		}
	}

	return filtered, nil
}

// Filter specifies criteria for filtering records.
type Filter struct {
	JobID     string
	Direction Direction
	Types     []string
}

// FilterRecords filters a slice of records.
func FilterRecords(records []Record, f Filter) []Record {
	var result []Record
	for _, rec := range records {
		if f.JobID != "" && rec.JobID != f.JobID {
			continue
		}
		if f.Direction != "" && rec.Direction != f.Direction {
			continue
		}
		if len(f.Types) > 0 && !slices.Contains(f.Types, rec.Type) {
			continue
		}
		result = append(result, rec)
	}

	return result
}

// CleanOldRecordings removes recordings with modification time before the given
// Unix timestamp (seconds since epoch).
func CleanOldRecordings(dir string, olderThan int64) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}

		return 0, fmt.Errorf("read dir: %w", err)
	}

	cutoff := olderThan
	removed := 0

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Unix() < cutoff {
			path := filepath.Join(dir, entry.Name())
			if err := os.Remove(path); err == nil {
				removed++
			}
		}
	}

	return removed, nil
}
