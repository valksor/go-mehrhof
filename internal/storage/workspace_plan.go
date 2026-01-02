package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Planned directory methods (standalone planning without a task)

// PlannedRoot returns the .mehrhof/planned directory path.
func (w *Workspace) PlannedRoot() string {
	return filepath.Join(w.taskRoot, plannedDirName)
}

// PlannedPath returns the path for a specific plan.
func (w *Workspace) PlannedPath(planID string) string {
	return filepath.Join(w.PlannedRoot(), planID)
}

// Plan represents a standalone planning session.
type Plan struct {
	Version string      `yaml:"version"`
	ID      string      `yaml:"id"`
	Title   string      `yaml:"title,omitempty"`
	Seed    string      `yaml:"seed,omitempty"`
	Created time.Time   `yaml:"created"`
	Updated time.Time   `yaml:"updated"`
	History []PlanEntry `yaml:"history,omitempty"`
}

// PlanEntry represents an entry in the planning conversation.
type PlanEntry struct {
	Timestamp time.Time `yaml:"timestamp"`
	Role      string    `yaml:"role"`
	Content   string    `yaml:"content"`
}

const (
	planFileName        = "plan.yaml"
	planHistoryFileName = "plan-history.md"
)

// CreatePlan creates a new standalone plan.
func (w *Workspace) CreatePlan(planID, seed string) (*Plan, error) {
	planPath := w.PlannedPath(planID)

	// Create plan directory
	if err := os.MkdirAll(planPath, 0o755); err != nil {
		return nil, fmt.Errorf("create plan directory: %w", err)
	}

	now := time.Now()
	plan := &Plan{
		Version: "1",
		ID:      planID,
		Seed:    seed,
		Created: now,
		Updated: now,
		History: make([]PlanEntry, 0),
	}

	// Save plan.yaml
	if err := w.SavePlan(plan); err != nil {
		return nil, fmt.Errorf("save plan: %w", err)
	}

	// Create initial plan-history.md
	historyPath := filepath.Join(planPath, planHistoryFileName)
	header := fmt.Sprintf("# Planning Session\n\nCreated: %s\n", now.Format("2006-01-02 15:04:05"))
	if seed != "" {
		header += fmt.Sprintf("\nSeed Topic: %s\n", seed)
	}
	header += "\n---\n\n"
	if err := os.WriteFile(historyPath, []byte(header), 0o644); err != nil {
		return nil, fmt.Errorf("create history file: %w", err)
	}

	return plan, nil
}

// SavePlan saves a plan's metadata.
func (w *Workspace) SavePlan(plan *Plan) error {
	plan.Updated = time.Now()
	planFile := filepath.Join(w.PlannedPath(plan.ID), planFileName)

	data, err := yaml.Marshal(plan)
	if err != nil {
		return fmt.Errorf("marshal plan: %w", err)
	}

	return os.WriteFile(planFile, data, 0o644)
}

// LoadPlan loads a plan by ID.
func (w *Workspace) LoadPlan(planID string) (*Plan, error) {
	planFile := filepath.Join(w.PlannedPath(planID), planFileName)

	data, err := os.ReadFile(planFile)
	if err != nil {
		return nil, fmt.Errorf("read plan file: %w", err)
	}

	var plan Plan
	if err := yaml.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("parse plan file: %w", err)
	}

	return &plan, nil
}

// AppendPlanHistory appends an entry to the plan history (both YAML and markdown).
func (w *Workspace) AppendPlanHistory(planID, role, content string) error {
	plan, err := w.LoadPlan(planID)
	if err != nil {
		return err
	}

	entry := PlanEntry{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}
	plan.History = append(plan.History, entry)

	// Save updated plan.yaml
	if err := w.SavePlan(plan); err != nil {
		return err
	}

	// Append to markdown history
	historyPath := filepath.Join(w.PlannedPath(planID), planHistoryFileName)
	f, err := os.OpenFile(historyPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open history file: %w", err)
	}
	defer func() { _ = f.Close() }()

	roleLabel := "User"
	if role == "assistant" {
		roleLabel = "Assistant"
	}
	_, err = fmt.Fprintf(f, "## %s (%s)\n\n%s\n\n---\n\n",
		roleLabel, entry.Timestamp.Format("15:04:05"), content)
	return err
}

// ListPlans returns all plan IDs.
func (w *Workspace) ListPlans() ([]string, error) {
	plannedRoot := w.PlannedRoot()
	if _, err := os.Stat(plannedRoot); os.IsNotExist(err) {
		return []string{}, nil
	}

	entries, err := os.ReadDir(plannedRoot)
	if err != nil {
		return nil, fmt.Errorf("read planned directory: %w", err)
	}

	var planIDs []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			planFile := filepath.Join(plannedRoot, entry.Name(), planFileName)
			if _, err := os.Stat(planFile); err == nil {
				planIDs = append(planIDs, entry.Name())
			}
		}
	}

	return planIDs, nil
}

// DeletePlan removes a plan directory.
func (w *Workspace) DeletePlan(planID string) error {
	return os.RemoveAll(w.PlannedPath(planID))
}

// GeneratePlanID generates a unique plan ID based on timestamp.
func GeneratePlanID() string {
	return time.Now().Format("2006-01-02-150405")
}
