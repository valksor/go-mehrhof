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

// Review status constants.
const (
	ReviewStatusPending  = "pending"
	ReviewStatusApproved = "approved"
	ReviewStatusRejected = "rejected"
	ReviewStatusRevised  = "revised"
)

// Review represents a code review with optional YAML frontmatter.
type Review struct {
	Number      int       `yaml:"-" json:"number"`
	Title       string    `yaml:"title,omitempty" json:"title,omitempty"`
	Status      string    `yaml:"status,omitempty" json:"status,omitempty"`
	Content     string    `yaml:"-" json:"content"`
	Reviewer    string    `yaml:"reviewer,omitempty" json:"reviewer,omitempty"`
	CreatedAt   time.Time `yaml:"created_at,omitempty" json:"created_at,omitzero"`
	UpdatedAt   time.Time `yaml:"updated_at,omitempty" json:"updated_at,omitzero"`
	CompletedAt time.Time `yaml:"completed_at,omitempty" json:"completed_at,omitzero"`
}

// ReviewStore manages review persistence for tasks.
type ReviewStore struct {
	store *Store
	mu    sync.RWMutex
}

// NewReviewStore creates a new ReviewStore.
func NewReviewStore(store *Store) *ReviewStore {
	return &ReviewStore{store: store}
}

// reviewFilename returns the filename for a review number.
func reviewFilename(number int) string {
	return fmt.Sprintf("review-%d.md", number)
}

// ReviewPath returns the full path for a review file.
func (r *ReviewStore) ReviewPath(taskID string, number int) string {
	return filepath.Join(r.store.ReviewsDir(taskID), reviewFilename(number))
}

// SaveReview saves a review file (markdown).
func (r *ReviewStore) SaveReview(taskID string, number int, content string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !isValidTaskID(taskID) {
		return fmt.Errorf("invalid task ID %q", taskID)
	}

	reviewPath := r.ReviewPath(taskID, number)
	reviewDir := filepath.Dir(reviewPath)

	if err := EnsureDir(reviewDir); err != nil {
		return fmt.Errorf("create reviews directory: %w", err)
	}

	if err := os.WriteFile(reviewPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("save review: %w", err)
	}

	return nil
}

// LoadReview loads a review file content.
func (r *ReviewStore) LoadReview(taskID string, number int) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	reviewPath := r.ReviewPath(taskID, number)
	data, err := os.ReadFile(reviewPath)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// buildReviewPatternRegex builds a regex to match review filenames.
func buildReviewPatternRegex() *regexp.Regexp {
	return regexp.MustCompile(`^review-(\d+)\.md$`)
}

// ListReviews returns all review numbers for a task.
func (r *ReviewStore) ListReviews(taskID string) ([]int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	numbers, err := ListNumberedFiles(r.store.ReviewsDir(taskID), buildReviewPatternRegex())
	if err != nil {
		return nil, fmt.Errorf("list reviews: %w", err)
	}

	return numbers, nil
}

// NextReviewNumber returns the next available review number.
func (r *ReviewStore) NextReviewNumber(taskID string) (int, error) {
	reviews, err := r.ListReviews(taskID)
	if err != nil {
		return 0, err
	}

	if len(reviews) == 0 {
		return 1, nil
	}

	return reviews[len(reviews)-1] + 1, nil
}

// ParseReview parses a review file with optional YAML frontmatter.
func (r *ReviewStore) ParseReview(taskID string, number int) (*Review, error) {
	content, err := r.LoadReview(taskID, number)
	if err != nil {
		return nil, err
	}

	review := &Review{
		Number: number,
		Status: ReviewStatusPending,
	}

	// Check for YAML frontmatter (starts with ---)
	if strings.HasPrefix(content, "---\n") {
		endIdx := strings.Index(content[4:], "\n---")
		if endIdx > 0 {
			frontmatter := content[4 : 4+endIdx]
			review.Content = strings.TrimSpace(content[4+endIdx+4:])

			if err := yaml.Unmarshal([]byte(frontmatter), review); err != nil {
				review.Content = content
			}
		} else {
			review.Content = content
		}
	} else {
		review.Content = content
	}

	// Extract title from first heading
	for line := range strings.SplitSeq(review.Content, "\n") {
		if title, ok := strings.CutPrefix(line, "# "); ok {
			review.Title = title

			break
		}
	}

	return review, nil
}

// SaveReviewWithMeta saves a review with YAML frontmatter.
func (r *ReviewStore) SaveReviewWithMeta(taskID string, review *Review) error {
	now := time.Now()
	if review.CreatedAt.IsZero() {
		review.CreatedAt = now
	}
	review.UpdatedAt = now

	// Build frontmatter
	frontmatter, err := yaml.Marshal(review)
	if err != nil {
		return fmt.Errorf("marshal review frontmatter: %w", err)
	}

	// Combine frontmatter and content
	var content strings.Builder
	content.WriteString("---\n")
	content.Write(frontmatter)
	content.WriteString("---\n\n")
	content.WriteString(review.Content)

	return r.SaveReview(taskID, review.Number, content.String())
}

// UpdateReviewStatus updates the status of a review file.
func (r *ReviewStore) UpdateReviewStatus(taskID string, number int, status string) error {
	review, err := r.ParseReview(taskID, number)
	if err != nil {
		return err
	}

	review.Status = status
	review.UpdatedAt = time.Now()

	if (status == ReviewStatusApproved || status == ReviewStatusRejected) && review.CompletedAt.IsZero() {
		review.CompletedAt = time.Now()
	}

	return r.SaveReviewWithMeta(taskID, review)
}

// GetLatestReview returns the most recent review for a task.
func (r *ReviewStore) GetLatestReview(taskID string) (*Review, error) {
	reviews, err := r.ListReviews(taskID)
	if err != nil {
		return nil, err
	}

	if len(reviews) == 0 {
		return nil, nil //nolint:nilnil // Documented behavior: nil means no reviews
	}

	return r.ParseReview(taskID, reviews[len(reviews)-1])
}

// DeleteReview removes a review file.
func (r *ReviewStore) DeleteReview(taskID string, number int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	reviewPath := r.ReviewPath(taskID, number)
	if err := os.Remove(reviewPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove review: %w", err)
	}

	return nil
}

// ReviewCount returns the number of reviews for a task.
func (r *ReviewStore) ReviewCount(taskID string) (int, error) {
	reviews, err := r.ListReviews(taskID)
	if err != nil {
		return 0, err
	}

	return len(reviews), nil
}

// GatherReviewsContent reads all reviews and returns combined content.
func (r *ReviewStore) GatherReviewsContent(taskID string) (string, error) {
	reviews, err := r.ListReviews(taskID)
	if err != nil {
		return "", err
	}

	var parts []string
	for _, num := range reviews {
		content, err := r.LoadReview(taskID, num)
		if err != nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("### Review %d\n\n%s", num, content))
	}

	return strings.Join(parts, "\n\n---\n\n"), nil
}
