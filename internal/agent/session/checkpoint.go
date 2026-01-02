package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// CurrentVersion is the current checkpoint format version.
	CurrentVersion = "1"

	// SessionDir is the subdirectory for session checkpoints.
	SessionDir = "sessions"

	// CheckpointExt is the file extension for checkpoints.
	CheckpointExt = ".yaml"
)

// Manager handles session checkpoint operations.
type Manager struct {
	baseDir string // Base directory (typically .mehrhof/work/<task>)
}

// NewManager creates a session manager for the given task directory.
func NewManager(taskDir string) *Manager {
	return &Manager{
		baseDir: filepath.Join(taskDir, SessionDir),
	}
}

// Save saves a session state to disk.
func (m *Manager) Save(state *State) error {
	if err := os.MkdirAll(m.baseDir, 0o755); err != nil {
		return fmt.Errorf("create sessions directory: %w", err)
	}

	// Generate ID if not set
	if state.ID == "" {
		state.ID = generateID()
	}

	// Set version and checkpoint time
	state.Version = CurrentVersion
	state.CheckpointedAt = time.Now()

	// Build filename: timestamp-phase-id.yaml
	filename := fmt.Sprintf("%s-%s-%s%s",
		state.CheckpointedAt.Format("20060102-150405"),
		sanitizeFilename(state.Phase),
		state.ID[:8], // Short ID
		CheckpointExt,
	)

	path := filepath.Join(m.baseDir, filename)

	data, err := yaml.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal session state: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write checkpoint file: %w", err)
	}

	return nil
}

// Load loads a session state by ID.
func (m *Manager) Load(id string) (*State, error) {
	// Find checkpoint file with matching ID
	files, err := m.listFiles()
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if strings.Contains(file, id[:min(8, len(id))]) {
			return m.loadFile(filepath.Join(m.baseDir, file))
		}
	}

	return nil, fmt.Errorf("session not found: %s", id)
}

// LoadLatest loads the most recent recoverable session.
func (m *Manager) LoadLatest() (*State, error) {
	sessions, err := m.List()
	if err != nil {
		return nil, err
	}

	// Find most recent recoverable session
	for _, summary := range sessions {
		state, err := m.Load(summary.ID)
		if err != nil {
			continue
		}
		if state.IsRecoverable() {
			return state, nil
		}
	}

	return nil, fmt.Errorf("no recoverable sessions found")
}

// List returns summaries of all sessions, sorted by checkpoint time (newest first).
func (m *Manager) List() ([]Summary, error) {
	files, err := m.listFiles()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var summaries []Summary
	for _, file := range files {
		state, err := m.loadFile(filepath.Join(m.baseDir, file))
		if err != nil {
			continue // Skip corrupt files
		}
		summaries = append(summaries, state.ToSummary())
	}

	// Sort by checkpoint time (newest first)
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].CheckpointedAt.After(summaries[j].CheckpointedAt)
	})

	return summaries, nil
}

// ListRecoverable returns only recoverable sessions.
func (m *Manager) ListRecoverable() ([]Summary, error) {
	all, err := m.List()
	if err != nil {
		return nil, err
	}

	var recoverable []Summary
	for _, s := range all {
		if s.Status == StatusInterrupted || s.Status == StatusRecoverable || s.Status == StatusFailed {
			recoverable = append(recoverable, s)
		}
	}

	return recoverable, nil
}

// Delete removes a session checkpoint by ID.
func (m *Manager) Delete(id string) error {
	files, err := m.listFiles()
	if err != nil {
		return err
	}

	for _, file := range files {
		if strings.Contains(file, id[:min(8, len(id))]) {
			return os.Remove(filepath.Join(m.baseDir, file))
		}
	}

	return fmt.Errorf("session not found: %s", id)
}

// Clean removes old session checkpoints
// maxAge: remove sessions older than this duration
// maxCount: keep at most this many sessions (0 = unlimited).
func (m *Manager) Clean(maxAge time.Duration, maxCount int) (int, error) {
	sessions, err := m.List()
	if err != nil {
		return 0, err
	}

	removed := 0
	cutoff := time.Now().Add(-maxAge)

	for i, s := range sessions {
		shouldRemove := false

		// Remove if too old
		if s.CheckpointedAt.Before(cutoff) {
			shouldRemove = true
		}

		// Remove if over max count (list is sorted newest first)
		if maxCount > 0 && i >= maxCount {
			shouldRemove = true
		}

		// Don't remove active sessions
		if s.Status == StatusActive {
			shouldRemove = false
		}

		if shouldRemove {
			if err := m.Delete(s.ID); err == nil {
				removed++
			}
		}
	}

	return removed, nil
}

// MarkInterrupted marks a session as interrupted.
func (m *Manager) MarkInterrupted(id string, errorMsg string) error {
	state, err := m.Load(id)
	if err != nil {
		return err
	}

	state.Status = StatusInterrupted
	state.Error = errorMsg
	return m.Save(state)
}

// MarkCompleted marks a session as completed.
func (m *Manager) MarkCompleted(id string) error {
	state, err := m.Load(id)
	if err != nil {
		return err
	}

	state.Status = StatusCompleted
	return m.Save(state)
}

// Helper methods

func (m *Manager) listFiles() ([]string, error) {
	entries, err := os.ReadDir(m.baseDir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), CheckpointExt) {
			files = append(files, entry.Name())
		}
	}

	// Sort by name (which includes timestamp) - newest first
	sort.Sort(sort.Reverse(sort.StringSlice(files)))

	return files, nil
}

func (m *Manager) loadFile(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read checkpoint: %w", err)
	}

	var state State
	if err := yaml.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshal checkpoint: %w", err)
	}

	return &state, nil
}

// generateID creates a unique session ID.
func generateID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID
		return fmt.Sprintf("ses-%d", time.Now().UnixNano())
	}
	return "ses-" + hex.EncodeToString(bytes)
}

// sanitizeFilename removes or replaces characters that are problematic in filenames.
func sanitizeFilename(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		if r == ' ' || r == '-' || r == '_' {
			return '-'
		}
		return -1
	}, s)

	// Remove consecutive hyphens
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}
