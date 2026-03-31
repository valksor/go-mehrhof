package screenshot

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Store manages screenshot storage and metadata.
type Store struct {
	basePath string
	mu       sync.RWMutex
}

// NewStore creates a new screenshot store.
// basePath should be the .mehrhof directory path.
func NewStore(basePath string) *Store {
	return &Store{
		basePath: basePath,
	}
}

// screenshotDir returns the directory for a task's screenshots.
func (s *Store) screenshotDir(taskID string) string {
	return filepath.Join(s.basePath, "screenshots", taskID)
}

// indexPath returns the path to the index file for a task.
func (s *Store) indexPath(taskID string) string {
	return filepath.Join(s.screenshotDir(taskID), "index.json")
}

// Save stores a screenshot and returns its metadata.
func (s *Store) Save(taskID string, data []byte, opts SaveOptions) (*Screenshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := s.screenshotDir(taskID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create screenshot directory: %w", err)
	}

	// Generate unique ID and filename
	id := uuid.New().String()[:8]
	timestamp := time.Now()
	format := cmp.Or(opts.Format, FormatPNG)

	filename := fmt.Sprintf("%s-%s.%s", timestamp.Format("20060102-150405"), id, format)
	path := filepath.Join(dir, filename)

	// Write image file
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return nil, fmt.Errorf("write screenshot file: %w", err)
	}

	// Get image dimensions
	width, height := getImageDimensions(data)

	screenshot := &Screenshot{
		ID:        id,
		TaskID:    taskID,
		Path:      path,
		Filename:  filename,
		Timestamp: timestamp,
		Source:    opts.Source,
		Step:      opts.Step,
		Agent:     opts.Agent,
		Format:    format,
		Width:     width,
		Height:    height,
		SizeBytes: int64(len(data)),
	}

	// Update index
	if err := s.addToIndex(taskID, screenshot); err != nil {
		return nil, fmt.Errorf("update index: %w", err)
	}

	return screenshot, nil
}

// List returns all screenshots for a task, sorted by timestamp (newest first).
func (s *Store) List(taskID string) ([]Screenshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	screenshots, err := s.loadIndex(taskID)
	if err != nil {
		return nil, err
	}

	// Sort by timestamp descending (newest first)
	slices.SortFunc(screenshots, func(a, b Screenshot) int {
		return b.Timestamp.Compare(a.Timestamp)
	})

	return screenshots, nil
}

// Get returns a single screenshot by ID.
func (s *Store) Get(taskID, screenshotID string) (*Screenshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	screenshots, err := s.loadIndex(taskID)
	if err != nil {
		return nil, err
	}

	for _, ss := range screenshots {
		if ss.ID == screenshotID {
			return &ss, nil
		}
	}

	return nil, fmt.Errorf("screenshot not found: %s", screenshotID)
}

// Delete removes a screenshot.
func (s *Store) Delete(taskID, screenshotID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	screenshots, err := s.loadIndex(taskID)
	if err != nil {
		return err
	}

	var found *Screenshot
	var remaining []Screenshot
	for _, ss := range screenshots {
		if ss.ID == screenshotID {
			found = &ss
		} else {
			remaining = append(remaining, ss)
		}
	}

	if found == nil {
		return fmt.Errorf("screenshot not found: %s", screenshotID)
	}

	// Delete file
	if err := os.Remove(found.Path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete screenshot file: %w", err)
	}

	// Update index
	return s.saveIndex(taskID, remaining)
}

// GetPath returns the file path for a screenshot.
func (s *Store) GetPath(taskID, screenshotID string) (string, error) {
	ss, err := s.Get(taskID, screenshotID)
	if err != nil {
		return "", err
	}

	return ss.Path, nil
}

// loadIndex loads the index file for a task.
func (s *Store) loadIndex(taskID string) ([]Screenshot, error) {
	path := s.indexPath(taskID)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []Screenshot{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read index: %w", err)
	}

	var screenshots []Screenshot
	if err := json.Unmarshal(data, &screenshots); err != nil {
		return nil, fmt.Errorf("parse index: %w", err)
	}

	return screenshots, nil
}

// saveIndex writes the index file for a task.
func (s *Store) saveIndex(taskID string, screenshots []Screenshot) error {
	data, err := json.MarshalIndent(screenshots, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal index: %w", err)
	}

	path := s.indexPath(taskID)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write index: %w", err)
	}

	return nil
}

// addToIndex appends a screenshot to the index.
func (s *Store) addToIndex(taskID string, screenshot *Screenshot) error {
	screenshots, err := s.loadIndex(taskID)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	screenshots = append(screenshots, *screenshot)

	return s.saveIndex(taskID, screenshots)
}

// getImageDimensions extracts width and height from image data.
//
//nolint:nonamedreturns // Named returns document the return values
func getImageDimensions(data []byte) (width, height int) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0
	}

	return cfg.Width, cfg.Height
}
