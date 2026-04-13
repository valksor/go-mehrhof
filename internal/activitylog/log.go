package activitylog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/valksor/kvelmo/paths"
)

// Entry represents a single RPC call record in the activity log.
type Entry struct {
	Timestamp     time.Time `json:"timestamp"`
	Method        string    `json:"method"`
	CorrelationID string    `json:"correlation_id"`
	DurationMs    int64     `json:"duration_ms"`
	Error         string    `json:"error,omitempty"`
	ParamsSize    int       `json:"params_size"`
	UserID        string    `json:"user_id,omitempty"`
	TaskID        string    `json:"task_id,omitempty"`
	AgentModel    string    `json:"agent_model,omitempty"`
	FilesAffected []string  `json:"files_affected,omitempty"`
	TaskTraceID   string    `json:"task_trace_id,omitempty"`   // Links all entries for a task lifecycle
	ParentEntryID string    `json:"parent_entry_id,omitempty"` // Hierarchical grouping
	PrevHash      string    `json:"prev_hash,omitempty"`
}

// Log manages append-only JSONL activity log files with non-blocking writes,
// daily file rotation, and automatic cleanup of old files.
type Log struct {
	dir       string
	maxFiles  int
	entries   chan Entry
	done      chan struct{}
	once      sync.Once
	forwarder *Forwarder

	// nowFunc allows overriding time.Now for testing.
	nowFunc func() time.Time
}

const (
	defaultMaxFiles = 30
	channelCapacity = 1000
	filePrefix      = "activity-"
	fileExt         = ".jsonl"
	dateFormat      = "2006-01-02"
	dirPermissions  = 0o750
	filePermissions = 0o640
)

// New creates a new activity Log. If dir is empty, it defaults to
// filepath.Join(paths.BaseDir(), "activity"). If maxFiles <= 0, it defaults to 30.
func New(dir string, maxFiles int) (*Log, error) {
	if dir == "" {
		dir = filepath.Join(paths.BaseDir(), "activity")
	}
	if maxFiles <= 0 {
		maxFiles = defaultMaxFiles
	}

	if err := os.MkdirAll(dir, dirPermissions); err != nil {
		return nil, fmt.Errorf("create activity log dir: %w", err)
	}

	return &Log{
		dir:      dir,
		maxFiles: maxFiles,
		entries:  make(chan Entry, channelCapacity),
		done:     make(chan struct{}),
		nowFunc:  time.Now,
	}, nil
}

// SetForwarder configures a webhook forwarder for activity entries.
// Must be called before Start.
func (l *Log) SetForwarder(f *Forwarder) {
	l.forwarder = f
}

// Record enqueues an entry for writing. It is non-blocking; if the channel is
// full the entry is silently dropped to avoid blocking RPC handlers.
func (l *Log) Record(entry Entry) {
	select {
	case l.entries <- entry:
	default:
		slog.Warn("activity log channel full, dropping entry", "method", entry.Method)
	}
}

// Start runs the background writer loop. It blocks until ctx is cancelled,
// then flushes remaining entries before returning.
func (l *Log) Start(ctx context.Context) {
	var (
		currentDay  string
		currentFile *os.File
		prevHash    string
	)

	openFileForDay := func(day string) {
		if currentFile != nil {
			_ = currentFile.Close()
		}
		name := filepath.Join(l.dir, filePrefix+day+fileExt)
		f, err := os.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, filePermissions)
		if err != nil {
			slog.Error("failed to open activity log file", "path", name, "error", err)
			currentFile = nil

			return
		}
		currentFile = f
		currentDay = day

		// Seed prevHash from last entry in the file (for appending to existing files)
		prevHash = lastHashFromFile(filepath.Join(l.dir, filePrefix+day+fileExt))
	}

	writeEntry := func(e Entry) {
		day := e.Timestamp.Format(dateFormat)
		if day != currentDay {
			openFileForDay(day)
			l.cleanup()
		}
		if currentFile == nil {
			return
		}

		// Set hash chain: this entry's PrevHash points to the hash of the previous entry
		e.PrevHash = prevHash
		data, err := json.Marshal(e)
		if err != nil {
			slog.Error("failed to marshal activity entry", "error", err)

			return
		}

		// Compute hash of this entry for the next entry's PrevHash
		h := sha256.Sum256(data)
		prevHash = hex.EncodeToString(h[:])

		data = append(data, '\n')
		if _, err := currentFile.Write(data); err != nil {
			slog.Error("failed to write activity entry", "error", err)
		}

		if l.forwarder != nil {
			l.forwarder.ForwardAsync(ctx, e)
		}
	}

	defer func() {
		// Drain remaining entries on shutdown.
		for {
			select {
			case e, ok := <-l.entries:
				if !ok {
					if currentFile != nil {
						_ = currentFile.Close()
					}
					close(l.done)

					return
				}
				writeEntry(e)
			default:
				if currentFile != nil {
					_ = currentFile.Close()
				}
				close(l.done)

				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-l.entries:
			if !ok {
				return
			}
			writeEntry(e)
		}
	}
}

// Close closes the entry channel and waits for the writer to drain.
func (l *Log) Close() {
	l.once.Do(func() {
		close(l.entries)
		<-l.done
	})
}

// cleanup removes the oldest log files when the count exceeds maxFiles.
func (l *Log) cleanup() {
	dirEntries, err := os.ReadDir(l.dir)
	if err != nil {
		slog.Error("failed to read activity log dir", "error", err)

		return
	}

	var logFiles []string
	for _, de := range dirEntries {
		if de.IsDir() {
			continue
		}
		name := de.Name()
		if strings.HasPrefix(name, filePrefix) && strings.HasSuffix(name, fileExt) {
			logFiles = append(logFiles, name)
		}
	}

	sort.Strings(logFiles)

	if len(logFiles) <= l.maxFiles {
		return
	}

	toRemove := logFiles[:len(logFiles)-l.maxFiles]
	for _, name := range toRemove {
		path := filepath.Join(l.dir, name)
		if err := os.Remove(path); err != nil {
			slog.Error("failed to remove old activity log", "path", path, "error", err)
		}
	}
}

// logFilesSorted returns all activity log file names in the directory, sorted.
func (l *Log) logFilesSorted() ([]string, error) {
	dirEntries, err := os.ReadDir(l.dir)
	if err != nil {
		return nil, fmt.Errorf("read activity log dir: %w", err)
	}

	var logFiles []string
	for _, de := range dirEntries {
		if de.IsDir() {
			continue
		}
		name := de.Name()
		if strings.HasPrefix(name, filePrefix) && strings.HasSuffix(name, fileExt) {
			logFiles = append(logFiles, name)
		}
	}

	sort.Strings(logFiles)

	return logFiles, nil
}

// dayFromFileName extracts the date string from a log file name.
func dayFromFileName(name string) string {
	name = strings.TrimPrefix(name, filePrefix)
	name = strings.TrimSuffix(name, fileExt)

	return name
}

// matchesMethodPattern checks if a method matches a pipe-separated pattern.
func matchesMethodPattern(method, pattern string) bool {
	if pattern == "" {
		return true
	}
	parts := strings.Split(pattern, "|")

	return slices.ContainsFunc(parts, func(p string) bool {
		return strings.Contains(method, strings.TrimSpace(p))
	})
}
