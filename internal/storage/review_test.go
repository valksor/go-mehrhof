package storage

import (
	"strings"
	"testing"
)

func newTestReviewStore(t *testing.T) *ReviewStore {
	t.Helper()

	return NewReviewStore(newTestStore(t))
}

func TestSaveLoadReview(t *testing.T) {
	rs := newTestReviewStore(t)

	content := "# My Review\n\nThis is the review content."
	if err := rs.SaveReview("task-1", 1, content); err != nil {
		t.Fatalf("SaveReview() error = %v", err)
	}

	got, err := rs.LoadReview("task-1", 1)
	if err != nil {
		t.Fatalf("LoadReview() error = %v", err)
	}
	if got != content {
		t.Errorf("LoadReview() = %q, want %q", got, content)
	}
}

func TestSaveReview_InvalidTaskID(t *testing.T) {
	rs := newTestReviewStore(t)

	err := rs.SaveReview("../traversal", 1, "content")
	if err == nil {
		t.Error("SaveReview() expected error for invalid task ID, got nil")
	}
}

func TestParseReview_NoFrontmatter(t *testing.T) {
	rs := newTestReviewStore(t)

	content := "# Review Title\n\nThis is the review."
	if err := rs.SaveReview("task-1", 1, content); err != nil {
		t.Fatalf("SaveReview() error = %v", err)
	}

	review, err := rs.ParseReview("task-1", 1)
	if err != nil {
		t.Fatalf("ParseReview() error = %v", err)
	}

	if review.Number != 1 {
		t.Errorf("Number = %d, want 1", review.Number)
	}
	if review.Title != "Review Title" {
		t.Errorf("Title = %q, want %q", review.Title, "Review Title")
	}
	if review.Status != ReviewStatusPending {
		t.Errorf("Status = %q, want %q", review.Status, ReviewStatusPending)
	}
}

func TestParseReview_WithFrontmatter(t *testing.T) {
	rs := newTestReviewStore(t)

	content := "---\nstatus: approved\nreviewer: bot\n---\n\n# My Review\n\nContent here."
	if err := rs.SaveReview("task-1", 1, content); err != nil {
		t.Fatalf("SaveReview() error = %v", err)
	}

	review, err := rs.ParseReview("task-1", 1)
	if err != nil {
		t.Fatalf("ParseReview() error = %v", err)
	}

	if review.Status != "approved" {
		t.Errorf("Status = %q, want approved", review.Status)
	}
	if review.Reviewer != "bot" {
		t.Errorf("Reviewer = %q, want bot", review.Reviewer)
	}
	if review.Title != "My Review" {
		t.Errorf("Title = %q, want My Review", review.Title)
	}
}

func TestSaveReviewWithMeta(t *testing.T) {
	rs := newTestReviewStore(t)

	review := &Review{
		Number:  1,
		Status:  ReviewStatusPending,
		Content: "# Review\n\nContent.",
	}

	if err := rs.SaveReviewWithMeta("task-1", review); err != nil {
		t.Fatalf("SaveReviewWithMeta() error = %v", err)
	}

	raw, err := rs.LoadReview("task-1", 1)
	if err != nil {
		t.Fatalf("LoadReview() error = %v", err)
	}

	if !strings.HasPrefix(raw, "---\n") {
		t.Error("SaveReviewWithMeta() did not write YAML frontmatter")
	}
	if review.CreatedAt.IsZero() {
		t.Error("CreatedAt not set by SaveReviewWithMeta()")
	}
	if review.UpdatedAt.IsZero() {
		t.Error("UpdatedAt not set by SaveReviewWithMeta()")
	}
}

func TestUpdateReviewStatus(t *testing.T) {
	rs := newTestReviewStore(t)

	review := &Review{
		Number:  1,
		Status:  ReviewStatusPending,
		Content: "# Review\n\nContent.",
	}
	if err := rs.SaveReviewWithMeta("task-1", review); err != nil {
		t.Fatalf("SaveReviewWithMeta() error = %v", err)
	}

	if err := rs.UpdateReviewStatus("task-1", 1, ReviewStatusApproved); err != nil {
		t.Fatalf("UpdateReviewStatus() error = %v", err)
	}

	got, err := rs.ParseReview("task-1", 1)
	if err != nil {
		t.Fatalf("ParseReview() error = %v", err)
	}

	if got.Status != ReviewStatusApproved {
		t.Errorf("Status = %q, want %q", got.Status, ReviewStatusApproved)
	}
	if got.CompletedAt.IsZero() {
		t.Error("CompletedAt is zero after approval, want non-zero")
	}
}

func TestUpdateReviewStatus_RejectedSetsCompletedAt(t *testing.T) {
	rs := newTestReviewStore(t)

	review := &Review{Number: 1, Status: ReviewStatusPending, Content: "# R\n\nContent."}
	if err := rs.SaveReviewWithMeta("task-1", review); err != nil {
		t.Fatal(err)
	}

	if err := rs.UpdateReviewStatus("task-1", 1, ReviewStatusRejected); err != nil {
		t.Fatal(err)
	}

	got, err := rs.ParseReview("task-1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.CompletedAt.IsZero() {
		t.Error("CompletedAt is zero after rejection, want non-zero")
	}
}

func TestListReviews_Empty(t *testing.T) {
	rs := newTestReviewStore(t)

	reviews, err := rs.ListReviews("task-1")
	if err != nil {
		t.Fatalf("ListReviews() error = %v", err)
	}
	if len(reviews) != 0 {
		t.Errorf("ListReviews() empty = %v, want []", reviews)
	}
}

func TestListReviews_Sorted(t *testing.T) {
	rs := newTestReviewStore(t)

	for _, n := range []int{3, 1, 2} {
		if err := rs.SaveReview("task-1", n, "content"); err != nil {
			t.Fatalf("SaveReview(%d) error = %v", n, err)
		}
	}

	reviews, err := rs.ListReviews("task-1")
	if err != nil {
		t.Fatalf("ListReviews() error = %v", err)
	}
	if len(reviews) != 3 {
		t.Fatalf("ListReviews() len = %d, want 3", len(reviews))
	}
	for i, want := range []int{1, 2, 3} {
		if reviews[i] != want {
			t.Errorf("reviews[%d] = %d, want %d", i, reviews[i], want)
		}
	}
}

func TestNextReviewNumber(t *testing.T) {
	rs := newTestReviewStore(t)

	n, err := rs.NextReviewNumber("task-1")
	if err != nil {
		t.Fatalf("NextReviewNumber() error = %v", err)
	}
	if n != 1 {
		t.Errorf("NextReviewNumber() empty = %d, want 1", n)
	}

	if err := rs.SaveReview("task-1", 1, "content"); err != nil {
		t.Fatal(err)
	}

	n, err = rs.NextReviewNumber("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("NextReviewNumber() after one = %d, want 2", n)
	}
}

func TestGetLatestReview_Empty(t *testing.T) {
	rs := newTestReviewStore(t)

	review, err := rs.GetLatestReview("task-1")
	if err != nil {
		t.Fatalf("GetLatestReview() error = %v", err)
	}
	if review != nil {
		t.Errorf("GetLatestReview() empty = %v, want nil", review)
	}
}

func TestGetLatestReview_ReturnsHighestNumber(t *testing.T) {
	rs := newTestReviewStore(t)

	for _, n := range []int{1, 2, 3} {
		if err := rs.SaveReview("task-1", n, "# Review "+strings.Repeat("X", n)+"\n\nContent."); err != nil {
			t.Fatal(err)
		}
	}

	review, err := rs.GetLatestReview("task-1")
	if err != nil {
		t.Fatalf("GetLatestReview() error = %v", err)
	}
	if review == nil {
		t.Fatal("GetLatestReview() = nil")
	}
	if review.Number != 3 {
		t.Errorf("Number = %d, want 3", review.Number)
	}
}

func TestDeleteReview(t *testing.T) {
	rs := newTestReviewStore(t)

	// Non-existent is not an error
	if err := rs.DeleteReview("task-1", 99); err != nil {
		t.Errorf("DeleteReview() non-existent error = %v, want nil", err)
	}

	if err := rs.SaveReview("task-1", 1, "content"); err != nil {
		t.Fatal(err)
	}

	if err := rs.DeleteReview("task-1", 1); err != nil {
		t.Fatalf("DeleteReview() error = %v", err)
	}

	reviews, _ := rs.ListReviews("task-1")
	if len(reviews) != 0 {
		t.Errorf("ListReviews() after delete = %v, want empty", reviews)
	}
}

func TestGatherReviewsContent(t *testing.T) {
	rs := newTestReviewStore(t)

	for _, n := range []int{1, 2} {
		content := "Review number content"
		if err := rs.SaveReview("task-1", n, content); err != nil {
			t.Fatal(err)
		}
	}

	gathered, err := rs.GatherReviewsContent("task-1")
	if err != nil {
		t.Fatalf("GatherReviewsContent() error = %v", err)
	}

	if !strings.Contains(gathered, "Review 1") {
		t.Error("GatherReviewsContent() missing Review 1 header")
	}
	if !strings.Contains(gathered, "Review 2") {
		t.Error("GatherReviewsContent() missing Review 2 header")
	}
	if !strings.Contains(gathered, "---") {
		t.Error("GatherReviewsContent() missing separator")
	}
}

func TestReviewCount(t *testing.T) {
	rs := newTestReviewStore(t)

	n, err := rs.ReviewCount("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("ReviewCount() empty = %d, want 0", n)
	}

	for i := 1; i <= 3; i++ {
		if err := rs.SaveReview("task-1", i, "content"); err != nil {
			t.Fatal(err)
		}
	}

	n, err = rs.ReviewCount("task-1")
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("ReviewCount() = %d, want 3", n)
	}
}
