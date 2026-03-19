package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Specification status constants.
const (
	SpecStatusDraft        = "draft"
	SpecStatusReady        = "ready"
	SpecStatusImplementing = "implementing"
	SpecStatusDone         = "done"
)

// validTaskIDRegex validates task IDs to prevent path traversal.
var validTaskIDRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Specification represents a specification with optional YAML frontmatter.
type Specification struct {
	Number      int       `yaml:"-" json:"number"`
	Title       string    `yaml:"title,omitempty" json:"title,omitempty"`
	Status      string    `yaml:"status,omitempty" json:"status,omitempty"`
	Content     string    `yaml:"-" json:"content"`
	CreatedAt   time.Time `yaml:"created_at,omitempty" json:"created_at,omitempty"`
	UpdatedAt   time.Time `yaml:"updated_at,omitempty" json:"updated_at,omitempty"`
	CompletedAt time.Time `yaml:"completed_at,omitempty" json:"completed_at,omitempty"`
}

// SpecStore manages specification persistence for tasks.
type SpecStore struct {
	store *Store
	mu    sync.RWMutex
}

// NewSpecStore creates a new SpecStore.
func NewSpecStore(store *Store) *SpecStore {
	return &SpecStore{store: store}
}

// isValidTaskID validates that a taskID is safe to use as a directory name.
func isValidTaskID(taskID string) bool {
	if taskID == "" {
		return false
	}
	if strings.Contains(taskID, "..") || strings.ContainsAny(taskID, "\\/") {
		return false
	}

	return validTaskIDRegex.MatchString(taskID)
}

// specFilename returns the filename for a specification number.
func specFilename(number int) string {
	return fmt.Sprintf("specification-%d.md", number)
}

// SpecificationPath returns the full path for a specification file.
func (s *SpecStore) SpecificationPath(taskID string, number int) string {
	return filepath.Join(s.store.SpecificationsDir(taskID), specFilename(number))
}

// SaveSpecification saves a specification file (markdown).
func (s *SpecStore) SaveSpecification(taskID string, number int, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !isValidTaskID(taskID) {
		return fmt.Errorf("invalid task ID %q: must contain only alphanumeric characters, hyphens, and underscores", taskID)
	}

	specPath := s.SpecificationPath(taskID, number)
	specDir := filepath.Dir(specPath)

	if err := EnsureDir(specDir); err != nil {
		return fmt.Errorf("create specifications directory: %w", err)
	}

	if err := os.WriteFile(specPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("save specification: %w", err)
	}

	return nil
}

// LoadSpecification loads a specification file content.
func (s *SpecStore) LoadSpecification(taskID string, number int) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	specPath := s.SpecificationPath(taskID, number)
	data, err := os.ReadFile(specPath)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// buildSpecPatternRegex builds a regex to match specification filenames.
func buildSpecPatternRegex() *regexp.Regexp {
	return regexp.MustCompile(`^specification-(\d+)\.md$`)
}

// ListSpecifications returns all specification numbers for a task.
func (s *SpecStore) ListSpecifications(taskID string) ([]int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	numbers, err := ListNumberedFiles(s.store.SpecificationsDir(taskID), buildSpecPatternRegex())
	if err != nil {
		return nil, fmt.Errorf("list specifications: %w", err)
	}

	return numbers, nil
}

// NextSpecificationNumber returns the next available specification number.
func (s *SpecStore) NextSpecificationNumber(taskID string) (int, error) {
	specs, err := s.ListSpecifications(taskID)
	if err != nil {
		return 0, err
	}

	if len(specs) == 0 {
		return 1, nil
	}

	return specs[len(specs)-1] + 1, nil
}

// GatherSpecificationsContent reads all specifications and returns combined content.
func (s *SpecStore) GatherSpecificationsContent(taskID string) (string, error) {
	specs, err := s.ListSpecifications(taskID)
	if err != nil {
		return "", err
	}

	var parts []string
	for _, num := range specs {
		content, err := s.LoadSpecification(taskID, num)
		if err != nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("### Specification %d\n\n%s", num, content))
	}

	return strings.Join(parts, "\n\n---\n\n"), nil
}

// GetLatestSpecificationContent returns the most recent specification content.
func (s *SpecStore) GetLatestSpecificationContent(taskID string) (string, int, error) {
	specs, err := s.ListSpecifications(taskID)
	if err != nil {
		return "", 0, err
	}

	if len(specs) == 0 {
		return "", 0, nil
	}

	latestNum := specs[len(specs)-1]
	content, err := s.LoadSpecification(taskID, latestNum)
	if err != nil {
		return "", 0, err
	}

	return content, latestNum, nil
}

// ParseSpecification parses a specification file with optional YAML frontmatter.
func (s *SpecStore) ParseSpecification(taskID string, number int) (*Specification, error) {
	content, err := s.LoadSpecification(taskID, number)
	if err != nil {
		return nil, err
	}

	spec := &Specification{
		Number: number,
		Status: SpecStatusDraft,
	}

	// Check for YAML frontmatter (starts with ---)
	if strings.HasPrefix(content, "---\n") {
		endIdx := strings.Index(content[4:], "\n---")
		if endIdx > 0 {
			frontmatter := content[4 : 4+endIdx]
			spec.Content = strings.TrimSpace(content[4+endIdx+4:])

			if err := yaml.Unmarshal([]byte(frontmatter), spec); err != nil {
				// Ignore frontmatter parsing errors, just use content
				spec.Content = content
			}
		} else {
			spec.Content = content
		}
	} else {
		spec.Content = content
	}

	// Extract title from first heading
	lines := strings.Split(spec.Content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "# ") {
			spec.Title = strings.TrimPrefix(line, "# ")

			break
		}
	}

	return spec, nil
}

// SaveSpecificationWithMeta saves a specification with YAML frontmatter.
func (s *SpecStore) SaveSpecificationWithMeta(taskID string, spec *Specification) error {
	now := time.Now()
	if spec.CreatedAt.IsZero() {
		spec.CreatedAt = now
	}
	spec.UpdatedAt = now

	// Build frontmatter
	frontmatter, err := yaml.Marshal(spec)
	if err != nil {
		return fmt.Errorf("marshal specification frontmatter: %w", err)
	}

	// Combine frontmatter and content
	var content strings.Builder
	content.WriteString("---\n")
	content.Write(frontmatter)
	content.WriteString("---\n\n")
	content.WriteString(spec.Content)

	return s.SaveSpecification(taskID, spec.Number, content.String())
}

// UpdateSpecificationStatus updates the status of a specification file.
func (s *SpecStore) UpdateSpecificationStatus(taskID string, number int, status string) error {
	spec, err := s.ParseSpecification(taskID, number)
	if err != nil {
		return err
	}

	spec.Status = status
	spec.UpdatedAt = time.Now()

	if status == SpecStatusDone && spec.CompletedAt.IsZero() {
		spec.CompletedAt = time.Now()
	}

	return s.SaveSpecificationWithMeta(taskID, spec)
}

// ListSpecificationsWithStatus returns all specifications with their parsed status.
func (s *SpecStore) ListSpecificationsWithStatus(taskID string) ([]*Specification, error) {
	numbers, err := s.ListSpecifications(taskID)
	if err != nil {
		return nil, err
	}

	if len(numbers) == 0 {
		return []*Specification{}, nil
	}

	specs := make([]*Specification, len(numbers))
	var wg sync.WaitGroup

	for i, num := range numbers {
		wg.Add(1)
		go func(idx, n int) {
			defer wg.Done()
			spec, parseErr := s.ParseSpecification(taskID, n)
			if parseErr != nil {
				specs[idx] = &Specification{Number: n, Status: "error"}

				return
			}
			specs[idx] = spec
		}(i, num)
	}
	wg.Wait()

	return specs, nil
}

// GetSpecificationsSummary returns a summary of specification statuses.
func (s *SpecStore) GetSpecificationsSummary(taskID string) (map[string]int, error) {
	specs, err := s.ListSpecificationsWithStatus(taskID)
	if err != nil {
		return nil, err
	}

	summary := map[string]int{
		SpecStatusDraft:        0,
		SpecStatusReady:        0,
		SpecStatusImplementing: 0,
		SpecStatusDone:         0,
	}

	for _, spec := range specs {
		summary[spec.Status]++
	}

	return summary, nil
}

// DeleteSpecification removes a specification file.
func (s *SpecStore) DeleteSpecification(taskID string, number int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	specPath := s.SpecificationPath(taskID, number)
	if err := os.Remove(specPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove specification: %w", err)
	}

	return nil
}

// SpecificationCount returns the number of specifications for a task.
func (s *SpecStore) SpecificationCount(taskID string) (int, error) {
	specs, err := s.ListSpecifications(taskID)
	if err != nil {
		return 0, err
	}

	return len(specs), nil
}
