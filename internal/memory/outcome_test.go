package memory

import (
	"context"
	"math"
	"testing"
)

const floatTolerance = 1e-9

func TestOutcomeScoreBoost(t *testing.T) {
	tests := []struct {
		name    string
		outcome *DocumentOutcome
		want    float64
	}{
		{
			name:    "nil outcome returns zero",
			outcome: nil,
			want:    0,
		},
		{
			name: "successful task returns positive boost",
			outcome: &DocumentOutcome{
				Success: true,
			},
			want: 0.1,
		},
		{
			name: "failed task returns negative boost",
			outcome: &DocumentOutcome{
				Success: false,
			},
			want: -0.05,
		},
		{
			name: "successful with merged PR gets extra boost",
			outcome: &DocumentOutcome{
				Success:  true,
				PRMerged: true,
			},
			want: 0.12,
		},
		{
			name: "successful with CI first try gets extra boost",
			outcome: &DocumentOutcome{
				Success:          true,
				CIPassedFirstTry: true,
			},
			want: 0.12,
		},
		{
			name: "successful but human changes needed reduces boost",
			outcome: &DocumentOutcome{
				Success:            true,
				HumanChangesNeeded: true,
			},
			want: 0.07,
		},
		{
			name: "full success: merged PR, CI first try, no human changes",
			outcome: &DocumentOutcome{
				Success:          true,
				PRMerged:         true,
				CIPassedFirstTry: true,
			},
			want: 0.14,
		},
		{
			name: "partial: success with human changes and CI first try",
			outcome: &DocumentOutcome{
				Success:            true,
				CIPassedFirstTry:   true,
				HumanChangesNeeded: true,
			},
			want: 0.09,
		},
		{
			name: "failed task ignores all other flags",
			outcome: &DocumentOutcome{
				Success:          false,
				PRMerged:         true,
				CIPassedFirstTry: true,
			},
			want: -0.05,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := OutcomeScoreBoost(tt.outcome)
			if math.Abs(got-tt.want) > floatTolerance {
				t.Errorf("OutcomeScoreBoost() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLinkOutcome(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	embedder := NewHashEmbedder(0)

	store, err := NewVectorStore(dir, embedder)
	if err != nil {
		t.Fatalf("NewVectorStore() error = %v", err)
	}

	// Store two documents for the same task and one for a different task.
	docs := []*Document{
		{ID: "doc-1", TaskID: "task-abc", Type: TypeSolution, Content: "fix for login bug"},
		{ID: "doc-2", TaskID: "task-abc", Type: TypeCodeChange, Content: "diff for login fix"},
		{ID: "doc-3", TaskID: "task-other", Type: TypeSolution, Content: "unrelated task"},
	}
	for _, doc := range docs {
		if err := store.Store(ctx, doc); err != nil {
			t.Fatalf("Store(%s) error = %v", doc.ID, err)
		}
	}

	// Link outcome to task-abc.
	outcome := DocumentOutcome{
		Success:          true,
		PRMerged:         true,
		CIPassedFirstTry: true,
	}
	if err := store.LinkOutcome(ctx, "task-abc", outcome); err != nil {
		t.Fatalf("LinkOutcome() error = %v", err)
	}

	// Verify doc-1 and doc-2 have the outcome set.
	for _, id := range []string{"doc-1", "doc-2"} {
		doc, err := store.Get(ctx, id)
		if err != nil {
			t.Fatalf("Get(%s) error = %v", id, err)
		}
		if doc.Outcome == nil {
			t.Errorf("Get(%s).Outcome = nil, want non-nil", id)

			continue
		}
		if !doc.Outcome.Success {
			t.Errorf("Get(%s).Outcome.Success = false, want true", id)
		}
		if !doc.Outcome.PRMerged {
			t.Errorf("Get(%s).Outcome.PRMerged = false, want true", id)
		}
	}

	// Verify doc-3 (different task) has no outcome.
	doc3, err := store.Get(ctx, "doc-3")
	if err != nil {
		t.Fatalf("Get(doc-3) error = %v", err)
	}
	if doc3.Outcome != nil {
		t.Errorf("Get(doc-3).Outcome = %+v, want nil", doc3.Outcome)
	}
}

func TestLinkOutcome_EmptyTaskID(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	embedder := NewHashEmbedder(0)

	store, err := NewVectorStore(dir, embedder)
	if err != nil {
		t.Fatalf("NewVectorStore() error = %v", err)
	}

	err = store.LinkOutcome(ctx, "", DocumentOutcome{Success: true})
	if err == nil {
		t.Error("LinkOutcome() with empty taskID should return error")
	}
}

func TestLinkOutcome_NoMatchingDocuments(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	embedder := NewHashEmbedder(0)

	store, err := NewVectorStore(dir, embedder)
	if err != nil {
		t.Fatalf("NewVectorStore() error = %v", err)
	}

	// Should not error, just no-op.
	if err := store.LinkOutcome(ctx, "nonexistent-task", DocumentOutcome{Success: true}); err != nil {
		t.Errorf("LinkOutcome() error = %v, want nil", err)
	}
}

func TestGetDocumentsForTask(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	embedder := NewHashEmbedder(0)

	store, err := NewVectorStore(dir, embedder)
	if err != nil {
		t.Fatalf("NewVectorStore() error = %v", err)
	}

	// Store documents for two different tasks.
	docs := []*Document{
		{ID: "a1", TaskID: "task-1", Type: TypeSolution, Content: "solution one"},
		{ID: "a2", TaskID: "task-1", Type: TypeCodeChange, Content: "change one"},
		{ID: "b1", TaskID: "task-2", Type: TypeSolution, Content: "solution two"},
	}
	for _, doc := range docs {
		if err := store.Store(ctx, doc); err != nil {
			t.Fatalf("Store(%s) error = %v", doc.ID, err)
		}
	}

	results := store.GetDocumentsForTask(ctx, "task-1")
	if len(results) != 2 {
		t.Errorf("GetDocumentsForTask(task-1) returned %d documents, want 2", len(results))
	}

	results = store.GetDocumentsForTask(ctx, "task-2")
	if len(results) != 1 {
		t.Errorf("GetDocumentsForTask(task-2) returned %d documents, want 1", len(results))
	}

	results = store.GetDocumentsForTask(ctx, "nonexistent")
	if len(results) != 0 {
		t.Errorf("GetDocumentsForTask(nonexistent) returned %d documents, want 0", len(results))
	}
}

func TestLinkOutcome_PersistsAcrossReload(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	embedder := NewHashEmbedder(0)

	store, err := NewVectorStore(dir, embedder)
	if err != nil {
		t.Fatalf("NewVectorStore() error = %v", err)
	}

	doc := &Document{ID: "persist-test", TaskID: "task-persist", Type: TypeSolution, Content: "persisted solution"}
	if err := store.Store(ctx, doc); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	outcome := DocumentOutcome{Success: true, PRMerged: true}
	if err := store.LinkOutcome(ctx, "task-persist", outcome); err != nil {
		t.Fatalf("LinkOutcome() error = %v", err)
	}

	// Reload from disk.
	store2, err := NewVectorStore(dir, embedder)
	if err != nil {
		t.Fatalf("NewVectorStore() reload error = %v", err)
	}

	reloaded, err := store2.Get(ctx, "persist-test")
	if err != nil {
		t.Fatalf("Get() after reload error = %v", err)
	}
	if reloaded.Outcome == nil {
		t.Fatal("reloaded document Outcome is nil, want non-nil")
	}
	if !reloaded.Outcome.Success {
		t.Error("reloaded document Outcome.Success = false, want true")
	}
	if !reloaded.Outcome.PRMerged {
		t.Error("reloaded document Outcome.PRMerged = false, want true")
	}
}
