package memory

import (
	"context"
	"strings"
	"testing"
	"time"
)

// --- sortedVocabKeys tests ---

func TestSortedVocabKeys_Empty(t *testing.T) {
	m := map[string]int{}
	keys := sortedVocabKeys(m)
	if len(keys) != 0 {
		t.Errorf("sortedVocabKeys(empty) = %v, want []", keys)
	}
}

func TestSortedVocabKeys_Sorted(t *testing.T) {
	m := map[string]int{"b": 1, "a": 2, "c": 3}
	keys := sortedVocabKeys(m)
	if len(keys) != 3 {
		t.Fatalf("sortedVocabKeys() len = %d, want 3", len(keys))
	}
	expected := []string{"a", "b", "c"}
	for i, k := range keys {
		if k != expected[i] {
			t.Errorf("keys[%d] = %q, want %q", i, k, expected[i])
		}
	}
}

func TestSortedVocabKeys_SingleElement(t *testing.T) {
	m := map[string]int{"only": 42}
	keys := sortedVocabKeys(m)
	if len(keys) != 1 || keys[0] != "only" {
		t.Errorf("sortedVocabKeys(single) = %v, want [only]", keys)
	}
}

// --- cosineSimilarity tests ---

func TestCosineSimilarity_MismatchedLengths(t *testing.T) {
	a := []float32{1, 2, 3}
	b := []float32{1, 2}
	score := cosineSimilarity(a, b)
	if score != 0 {
		t.Errorf("cosineSimilarity(mismatched) = %f, want 0", score)
	}
}

func TestCosineSimilarity_ZeroVector(t *testing.T) {
	a := []float32{0, 0, 0}
	b := []float32{1, 2, 3}
	score := cosineSimilarity(a, b)
	if score != 0 {
		t.Errorf("cosineSimilarity(zero, b) = %f, want 0", score)
	}
}

func TestCosineSimilarity_BothZeroVectors(t *testing.T) {
	a := []float32{0, 0, 0}
	b := []float32{0, 0, 0}
	score := cosineSimilarity(a, b)
	if score != 0 {
		t.Errorf("cosineSimilarity(zero, zero) = %f, want 0", score)
	}
}

func TestCosineSimilarity_IdenticalVectors(t *testing.T) {
	a := []float32{1, 2, 3}
	b := []float32{1, 2, 3}
	score := cosineSimilarity(a, b)
	if score < 0.999 || score > 1.001 {
		t.Errorf("cosineSimilarity(identical) = %f, want ~1.0", score)
	}
}

func TestCosineSimilarity_EmptyVectors(t *testing.T) {
	a := []float32{}
	b := []float32{}
	score := cosineSimilarity(a, b)
	if score != 0 {
		t.Errorf("cosineSimilarity(empty, empty) = %f, want 0", score)
	}
}

func TestCosineSimilarity_OrthogonalVectors(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{0, 1, 0}
	score := cosineSimilarity(a, b)
	if score > 0.001 {
		t.Errorf("cosineSimilarity(orthogonal) = %f, want ~0", score)
	}
}

// --- formatResult tests ---

func TestFormatResult_ShortContent(t *testing.T) {
	doc := &Document{
		ID:      "task-1",
		TaskID:  "task-1",
		Type:    TypeSpecification,
		Content: "short content",
	}
	result := &SearchResult{Document: doc, Score: 0.85}
	out := formatResult(result)
	if !strings.Contains(out, "task-1") {
		t.Errorf("formatResult() missing task ID: %q", out)
	}
	if !strings.Contains(out, "short content") {
		t.Errorf("formatResult() missing content: %q", out)
	}
}

func TestFormatResult_LongContentTruncated(t *testing.T) {
	// Content over 200 chars should be truncated with "..."
	longContent := strings.Repeat("x", 250)
	doc := &Document{
		ID:      "task-long",
		TaskID:  "task-long",
		Type:    TypeSolution,
		Content: longContent,
	}
	result := &SearchResult{Document: doc, Score: 0.90}
	out := formatResult(result)
	if !strings.Contains(out, "...") {
		t.Errorf("formatResult() with content>200 chars should contain '...', got: %q", out)
	}
	// Should not include the full 250-char content
	if strings.Contains(out, longContent) {
		t.Error("formatResult() should truncate content >200 chars")
	}
}

func TestFormatResult_ContentExactly200Chars(t *testing.T) {
	// Content of exactly 200 chars should NOT be truncated
	content200 := strings.Repeat("y", 200)
	doc := &Document{
		ID:      "task-200",
		TaskID:  "task-200",
		Type:    TypeSpecification,
		Content: content200,
	}
	result := &SearchResult{Document: doc, Score: 0.75}
	out := formatResult(result)
	if strings.Contains(out, "...") {
		t.Errorf("formatResult() with content==200 chars should not truncate, got: %q", out[:min(50, len(out))])
	}
}

// --- AugmentPrompt with a document that exceeds 300 chars ---

func TestAugmentPrompt_LongContentTruncated(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	embedder := NewHashEmbedder(0)
	store, err := NewVectorStore(dir, embedder)
	if err != nil {
		t.Fatalf("NewVectorStore error = %v", err)
	}

	// Build a content string long enough to trigger truncation in AugmentPrompt (>300 chars).
	// Use repeated vocabulary words so the hash embedder may score well.
	longContent := strings.Repeat("authentication login password token secure jwt refresh ", 10)
	// Trim trailing space and confirm it's >300 chars
	longContent = strings.TrimSpace(longContent)
	if len(longContent) <= 300 {
		t.Fatalf("test setup: longContent is only %d chars, need >300", len(longContent))
	}

	doc := &Document{
		ID:        "aug-long-doc",
		TaskID:    "task-aug-long",
		Type:      TypeSpecification,
		Content:   longContent,
		CreatedAt: time.Now(),
	}
	if err := store.Store(ctx, doc); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	indexer := NewIndexer(store, dir)
	adapter := NewAdapter(store, indexer)

	// Query using the same text to maximise similarity score
	result, err := adapter.AugmentPrompt(ctx, "task-aug-long", longContent)
	if err != nil {
		t.Fatalf("AugmentPrompt() error = %v", err)
	}

	// When a result is returned it should contain the truncation marker if the
	// preview exceeds 300 chars. If no result is returned (score < 0.70) that's
	// also valid for the hash embedder.
	if result != "" {
		if !strings.Contains(result, "Relevant Context") {
			t.Errorf("AugmentPrompt() result missing 'Relevant Context': %q", result)
		}
		// The preview in AugmentPrompt is capped at 300 chars + "..."
		if strings.Contains(result, longContent) {
			t.Error("AugmentPrompt() should truncate long content (>300 chars), but returned full content")
		}
	}
}
