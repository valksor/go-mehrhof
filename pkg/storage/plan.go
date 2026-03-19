package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	planFileName        = "plan.yaml"
	planHistoryFileName = "plan-history.md"
)

// Plan represents a planning session for a task.
type Plan struct {
	Version string      `yaml:"version" json:"version"`
	ID      string      `yaml:"id" json:"id"`
	TaskID  string      `yaml:"task_id" json:"task_id"`
	Title   string      `yaml:"title,omitempty" json:"title,omitempty"`
	Seed    string      `yaml:"seed,omitempty" json:"seed,omitempty"`
	Created time.Time   `yaml:"created" json:"created"`
	Updated time.Time   `yaml:"updated" json:"updated"`
	History []PlanEntry `yaml:"history,omitempty" json:"history,omitempty"`
}

// PlanEntry represents an entry in the planning conversation.
type PlanEntry struct {
	Timestamp time.Time `yaml:"timestamp" json:"timestamp"`
	Role      string    `yaml:"role" json:"role"`
	Content   string    `yaml:"content" json:"content"`
}

// PlanStore manages plan persistence for tasks.
type PlanStore struct {
	store *Store
	mu    sync.RWMutex
}

// NewPlanStore creates a new PlanStore.
func NewPlanStore(store *Store) *PlanStore {
	return &PlanStore{store: store}
}

// PlanPath returns the path for a specific plan directory.
func (p *PlanStore) PlanPath(taskID, planID string) string {
	return filepath.Join(p.store.PlansDir(taskID), planID)
}

// PlanFilePath returns the path for a plan's YAML file.
func (p *PlanStore) PlanFilePath(taskID, planID string) string {
	return filepath.Join(p.PlanPath(taskID, planID), planFileName)
}

// PlanHistoryPath returns the path for a plan's history markdown file.
func (p *PlanStore) PlanHistoryPath(taskID, planID string) string {
	return filepath.Join(p.PlanPath(taskID, planID), planHistoryFileName)
}

// CreatePlan creates a new plan for a task.
func (p *PlanStore) CreatePlan(taskID, planID, seed string) (*Plan, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !isValidTaskID(taskID) {
		return nil, fmt.Errorf("invalid task ID %q", taskID)
	}

	planPath := p.PlanPath(taskID, planID)

	// Create plan directory
	if err := EnsureDir(planPath); err != nil {
		return nil, fmt.Errorf("create plan directory: %w", err)
	}

	now := time.Now()
	plan := &Plan{
		Version: "1",
		ID:      planID,
		TaskID:  taskID,
		Seed:    seed,
		Created: now,
		Updated: now,
		History: make([]PlanEntry, 0),
	}

	// Save plan.yaml
	if err := p.savePlanLocked(taskID, plan); err != nil {
		return nil, fmt.Errorf("save plan: %w", err)
	}

	// Create initial plan-history.md
	historyPath := p.PlanHistoryPath(taskID, planID)
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

// savePlanLocked saves a plan's metadata (caller must hold lock).
func (p *PlanStore) savePlanLocked(taskID string, plan *Plan) error {
	plan.Updated = time.Now()
	planFile := p.PlanFilePath(taskID, plan.ID)

	return SaveYAML(planFile, plan)
}

// SavePlan saves a plan's metadata.
func (p *PlanStore) SavePlan(taskID string, plan *Plan) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.savePlanLocked(taskID, plan)
}

// LoadPlan loads a plan by ID.
func (p *PlanStore) LoadPlan(taskID, planID string) (*Plan, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.loadPlanLocked(taskID, planID)
}

// AppendPlanHistory appends an entry to the plan history (both YAML and markdown).
func (p *PlanStore) AppendPlanHistory(taskID, planID, role, content string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	plan, err := p.loadPlanLocked(taskID, planID)
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
	if err := p.savePlanLocked(taskID, plan); err != nil {
		return err
	}

	// Append to markdown history
	historyPath := p.PlanHistoryPath(taskID, planID)
	f, err := os.OpenFile(historyPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
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

// loadPlanLocked loads a plan without acquiring lock (caller must hold lock).
func (p *PlanStore) loadPlanLocked(taskID, planID string) (*Plan, error) {
	var plan Plan
	if err := LoadYAML(p.PlanFilePath(taskID, planID), &plan); err != nil {
		return nil, fmt.Errorf("load plan: %w", err)
	}

	return &plan, nil
}

// ListPlans returns all plan IDs for a task.
func (p *PlanStore) ListPlans(taskID string) ([]string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	plansDir := p.store.PlansDir(taskID)
	if _, err := os.Stat(plansDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	entries, err := os.ReadDir(plansDir)
	if err != nil {
		return nil, fmt.Errorf("read plans directory: %w", err)
	}

	var planIDs []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			planFile := filepath.Join(plansDir, entry.Name(), planFileName)
			if _, err := os.Stat(planFile); err == nil {
				planIDs = append(planIDs, entry.Name())
			}
		}
	}

	return planIDs, nil
}

// DeletePlan removes a plan directory.
func (p *PlanStore) DeletePlan(taskID, planID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	planPath := p.PlanPath(taskID, planID)
	if err := os.RemoveAll(planPath); err != nil {
		return fmt.Errorf("remove plan directory: %w", err)
	}

	return nil
}

// GetLatestPlan returns the most recent plan for a task.
func (p *PlanStore) GetLatestPlan(taskID string) (*Plan, error) {
	planIDs, err := p.ListPlans(taskID)
	if err != nil {
		return nil, err
	}

	if len(planIDs) == 0 {
		return nil, nil //nolint:nilnil // Documented behavior: nil means no plans
	}

	// Plans are named with timestamps, so last one is latest
	return p.LoadPlan(taskID, planIDs[len(planIDs)-1])
}

// LoadPlanHistory loads the markdown history file content.
func (p *PlanStore) LoadPlanHistory(taskID, planID string) (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	historyPath := p.PlanHistoryPath(taskID, planID)
	data, err := os.ReadFile(historyPath)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// GeneratePlanID generates a unique plan ID based on timestamp.
func GeneratePlanID() string {
	return time.Now().Format("2006-01-02-150405")
}
