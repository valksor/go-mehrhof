package storage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// WorkPath returns the path for a specific task's work directory.
func (w *Workspace) WorkPath(taskID string) string {
	return filepath.Join(w.workRoot, taskID)
}

// WorkExists checks if a work directory exists.
func (w *Workspace) WorkExists(taskID string) bool {
	workPath := w.WorkPath(taskID)
	info, err := os.Stat(workPath)
	return err == nil && info.IsDir()
}

// GenerateTaskID generates a unique task ID.
func GenerateTaskID() string {
	bytes := make([]byte, 4)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("task-%06x", time.Now().UnixNano()&0xffffff)
	}
	return hex.EncodeToString(bytes)
}

// CreateWork creates a new work directory with initial structure.
func (w *Workspace) CreateWork(taskID string, source SourceInfo) (*TaskWork, error) {
	workPath := w.WorkPath(taskID)

	// Create work directory structure
	dirs := []string{
		workPath,
		filepath.Join(workPath, specsDirName),
		filepath.Join(workPath, sessionsDirName),
		filepath.Join(workPath, "source"), // Source files directory
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	// Create work metadata
	work := NewTaskWork(taskID, source)

	// Save work.yaml
	if err := w.SaveWork(work); err != nil {
		return nil, fmt.Errorf("save work: %w", err)
	}

	// Create empty notes.md
	notesPath := filepath.Join(workPath, notesFileName)
	if err := os.WriteFile(notesPath, []byte("# Notes\n\n"), 0o644); err != nil {
		return nil, fmt.Errorf("create notes file: %w", err)
	}

	return work, nil
}

// LoadWork loads a task's work metadata.
func (w *Workspace) LoadWork(taskID string) (*TaskWork, error) {
	workFile := filepath.Join(w.WorkPath(taskID), workFileName)

	data, err := os.ReadFile(workFile)
	if err != nil {
		return nil, fmt.Errorf("read work file: %w", err)
	}

	var work TaskWork
	if err := yaml.Unmarshal(data, &work); err != nil {
		return nil, fmt.Errorf("parse work file: %w", err)
	}

	return &work, nil
}

// SaveWork saves a task's work metadata using atomic write pattern.
func (w *Workspace) SaveWork(work *TaskWork) error {
	work.Metadata.UpdatedAt = time.Now()

	workFile := filepath.Join(w.WorkPath(work.Metadata.ID), workFileName)

	data, err := yaml.Marshal(work)
	if err != nil {
		return fmt.Errorf("marshal work: %w", err)
	}

	// Use atomic write pattern: write to temp file, then rename
	tmpFile := workFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0o644); err != nil {
		return fmt.Errorf("write work file: %w", err)
	}
	// Atomic rename is guaranteed to be atomic on POSIX systems
	if err := os.Rename(tmpFile, workFile); err != nil {
		// Clean up temp file on error, log if cleanup fails
		if removeErr := os.Remove(tmpFile); removeErr != nil {
			slog.Warn("failed to clean up temp file after rename error", "path", tmpFile, "error", removeErr)
		}
		return fmt.Errorf("save work: %w", err)
	}

	return nil
}

// AddUsage adds token usage stats to a task's work with buffering.
// Usage is accumulated in memory and flushed periodically to reduce I/O.
// Call FlushUsage() to force an immediate flush of buffered data.
func (w *Workspace) AddUsage(taskID, step string, inputTokens, outputTokens, cachedTokens int, costUSD float64) error {
	w.usageMu.Lock()
	defer w.usageMu.Unlock()

	// Initialize task buffer if needed
	if w.usageBuf[taskID] == nil {
		w.usageBuf[taskID] = make(map[string]*usageBuffer)
	}

	// Initialize step buffer if needed
	buf := w.usageBuf[taskID][step]
	if buf == nil {
		buf = &usageBuffer{}
		w.usageBuf[taskID][step] = buf
	}

	// Accumulate usage
	buf.inputTokens += inputTokens
	buf.outputTokens += outputTokens
	buf.cachedTokens += cachedTokens
	buf.costUSD += costUSD
	buf.callCount++

	// Check if we should flush
	totalCalls := 0
	for _, steps := range w.usageBuf {
		for _, stepBuf := range steps {
			totalCalls += stepBuf.callCount
		}
	}

	// Flush if threshold reached or time elapsed
	if totalCalls >= defaultUsageFlushThreshold ||
		time.Since(w.lastFlush) > defaultUsageFlushInterval {
		return w.flushUsageLocked()
	}

	return nil
}

// FlushUsage flushes all buffered usage to disk.
// This is called automatically by AddUsage when thresholds are met,
// but can also be called explicitly to ensure data is persisted.
func (w *Workspace) FlushUsage() error {
	w.usageMu.Lock()
	defer w.usageMu.Unlock()
	return w.flushUsageLocked()
}

// flushUsageLocked flushes buffered usage. Must be called with usageMu held.
func (w *Workspace) flushUsageLocked() error {
	if len(w.usageBuf) == 0 {
		return nil
	}

	// Collect all task IDs that need flushing
	taskIDs := make([]string, 0, len(w.usageBuf))
	for taskID := range w.usageBuf {
		taskIDs = append(taskIDs, taskID)
	}

	// Flush each task
	var errs []error
	for _, taskID := range taskIDs {
		if err := w.flushTaskUsageLocked(taskID); err != nil {
			errs = append(errs, err)
		}
	}

	w.lastFlush = time.Now()

	if len(errs) > 0 {
		return fmt.Errorf("flush usage: %w", errors.Join(errs...))
	}
	return nil
}

// flushTaskUsageLocked flushes usage for a single task. Must be called with usageMu held.
func (w *Workspace) flushTaskUsageLocked(taskID string) error {
	steps, ok := w.usageBuf[taskID]
	if !ok || len(steps) == 0 {
		return nil
	}

	// Load current work
	work, err := w.LoadWork(taskID)
	if err != nil {
		return fmt.Errorf("load work: %w", err)
	}

	// Initialize ByStep map if needed
	if work.Costs.ByStep == nil {
		work.Costs.ByStep = make(map[string]StepCostStats)
	}

	// Accumulate all buffered steps
	for step, buf := range steps {
		// Update totals
		work.Costs.TotalInputTokens += buf.inputTokens
		work.Costs.TotalOutputTokens += buf.outputTokens
		work.Costs.TotalCachedTokens += buf.cachedTokens
		work.Costs.TotalCostUSD += buf.costUSD

		// Update step stats
		stepStats := work.Costs.ByStep[step]
		stepStats.InputTokens += buf.inputTokens
		stepStats.OutputTokens += buf.outputTokens
		stepStats.CachedTokens += buf.cachedTokens
		stepStats.CostUSD += buf.costUSD
		stepStats.Calls += buf.callCount
		work.Costs.ByStep[step] = stepStats
	}

	// Save work
	if err := w.SaveWork(work); err != nil {
		return fmt.Errorf("save work: %w", err)
	}

	// Clear buffer for this task
	delete(w.usageBuf, taskID)

	return nil
}

// DeleteWork removes a work directory.
func (w *Workspace) DeleteWork(taskID string) error {
	workPath := w.WorkPath(taskID)
	return os.RemoveAll(workPath)
}

// ListWorks returns all task IDs in the work directory.
func (w *Workspace) ListWorks() ([]string, error) {
	if _, err := os.Stat(w.workRoot); os.IsNotExist(err) {
		return []string{}, nil
	}

	entries, err := os.ReadDir(w.workRoot)
	if err != nil {
		return nil, fmt.Errorf("read work directory: %w", err)
	}

	var taskIDs []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			// Check if it has a work.yaml
			workFile := filepath.Join(w.workRoot, entry.Name(), workFileName)
			if _, err := os.Stat(workFile); err == nil {
				taskIDs = append(taskIDs, entry.Name())
			}
		}
	}

	return taskIDs, nil
}
