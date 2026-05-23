package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewVectorStore_LoadsExistingDocuments(t *testing.T) {
	dir := t.TempDir()
	// Pre-seed a JSON document on disk.
	doc := Document{
		ID:        "seed-1",
		TaskID:    "task-x",
		Type:      TypeSpecification,
		Content:   "preexisting content",
		CreatedAt: time.Now(),
	}
	store1, err := NewVectorStore(dir, NewHashEmbedder(0))
	if err != nil {
		t.Fatal(err)
	}
	if err := store1.Store(context.Background(), &doc); err != nil {
		t.Fatal(err)
	}

	// A fresh store over the same dir should load the persisted doc.
	store2, err := NewVectorStore(dir, NewHashEmbedder(0))
	if err != nil {
		t.Fatalf("NewVectorStore reload error = %v", err)
	}
	got, err := store2.Get(context.Background(), "seed-1")
	if err != nil {
		t.Fatalf("Get after reload error = %v", err)
	}
	if got.Content != "preexisting content" {
		t.Errorf("loaded content = %q", got.Content)
	}
}

func TestNewVectorStore_LoadCorruptJSONFails(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewVectorStore(dir, NewHashEmbedder(0)); err == nil {
		t.Fatal("expected error loading corrupt document")
	}
}

func TestVectorStore_StoreRequiresID(t *testing.T) {
	store := newTestStore(t)
	err := store.Store(context.Background(), &Document{Content: "no id"})
	if err == nil {
		t.Fatal("expected error for missing ID")
	}
}

func TestVectorStore_StoreDedup_SkipsNearDuplicate(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	content := "the quick brown fox jumps over the lazy dog repeatedly"
	if err := store.Store(ctx, &Document{ID: "a", Type: TypeSpecification, Content: content}); err != nil {
		t.Fatal(err)
	}
	// Identical content of same type -> Jaccard ~1.0 -> skipped.
	if err := store.Store(ctx, &Document{ID: "b", Type: TypeSpecification, Content: content}); err != nil {
		t.Fatal(err)
	}

	if store.Stats().TotalDocuments != 1 {
		t.Errorf("near-duplicate should be skipped, total = %d", store.Stats().TotalDocuments)
	}
	// "b" should not exist as its own entry.
	if _, err := store.Get(ctx, "b"); err == nil {
		t.Error("duplicate document b should not have been stored")
	}
}

func TestVectorStore_StoreDedup_MergesModerateOverlap(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Original with 10 distinct tokens.
	orig := "alpha beta gamma delta epsilon zeta eta theta iota kappa"
	if err := store.Store(ctx, &Document{ID: "orig", Type: TypeSolution, Content: orig}); err != nil {
		t.Fatal(err)
	}
	// Share 8 of 10 tokens, each adds 2 unique -> intersection 8, union 12,
	// Jaccard = 0.667 which is in [0.6, 0.85) -> merge into existing doc.
	overlap := "alpha beta gamma delta epsilon zeta eta theta lambda mu"
	if err := store.Store(ctx, &Document{ID: "new", Type: TypeSolution, Content: overlap}); err != nil {
		t.Fatal(err)
	}

	// Merge keeps the original ID but updates its content.
	if store.Stats().TotalDocuments != 1 {
		t.Errorf("moderate overlap should merge into one doc, total = %d", store.Stats().TotalDocuments)
	}
	merged, err := store.Get(ctx, "orig")
	if err != nil {
		t.Fatal(err)
	}
	if merged.Content != overlap {
		t.Errorf("merged content = %q, want %q", merged.Content, overlap)
	}
}

func TestVectorStore_StoreDifferentTypesNotDeduped(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	content := "the quick brown fox jumps over the lazy dog repeatedly"
	if err := store.Store(ctx, &Document{ID: "a", Type: TypeSpecification, Content: content}); err != nil {
		t.Fatal(err)
	}
	// Same content, different type -> not deduped.
	if err := store.Store(ctx, &Document{ID: "b", Type: TypeSession, Content: content}); err != nil {
		t.Fatal(err)
	}
	if store.Stats().TotalDocuments != 2 {
		t.Errorf("different types should both be stored, total = %d", store.Stats().TotalDocuments)
	}
}

func TestVectorStore_SearchByCategory(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	mustStore := func(id, cat string, importance float64, age time.Duration) {
		t.Helper()
		err := store.Store(ctx, &Document{
			ID:         id,
			Type:       TypeDecision,
			Content:    "content " + id,
			Category:   cat,
			Importance: importance,
			CreatedAt:  time.Now().Add(-age),
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	mustStore("low", "decisions", 0.2, time.Hour)
	mustStore("high", "decisions", 0.9, 2*time.Hour)
	mustStore("other", "requirements", 0.5, time.Hour)

	got := store.SearchByCategory("decisions", 0)
	if len(got) != 2 {
		t.Fatalf("SearchByCategory returned %d, want 2", len(got))
	}
	// Higher importance first.
	if got[0].ID != "high" {
		t.Errorf("first result ID = %q, want high (importance-sorted)", got[0].ID)
	}

	// Limit applies.
	limited := store.SearchByCategory("decisions", 1)
	if len(limited) != 1 || limited[0].ID != "high" {
		t.Errorf("limited result = %+v", limited)
	}

	// Unknown category.
	if got := store.SearchByCategory("nonexistent", 0); len(got) != 0 {
		t.Errorf("unknown category returned %d results", len(got))
	}
}

func TestVectorStore_Clear(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	for _, id := range []string{"a", "b", "c"} {
		if err := store.Store(ctx, &Document{ID: id, Type: TypeSpecification, Content: "x " + id}); err != nil {
			t.Fatal(err)
		}
	}
	if store.Stats().TotalDocuments != 3 {
		t.Fatalf("expected 3 docs before clear")
	}

	if err := store.Clear(ctx); err != nil {
		t.Fatalf("Clear error = %v", err)
	}
	if store.Stats().TotalDocuments != 0 {
		t.Errorf("after Clear total = %d, want 0", store.Stats().TotalDocuments)
	}
	// JSON files should be gone.
	entries, _ := os.ReadDir(store.dir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".json" {
			t.Errorf("leftover json file after Clear: %s", e.Name())
		}
	}
}

func TestVectorStore_Delete(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	if err := store.Store(ctx, &Document{ID: "del", Type: TypeSpecification, Content: "to be deleted"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, "del"); err != nil {
		t.Fatalf("Delete error = %v", err)
	}
	if _, err := store.Get(ctx, "del"); err == nil {
		t.Error("document should be gone after Delete")
	}
	// Deleting a missing ID is a no-op (file not found is tolerated).
	if err := store.Delete(ctx, "ghost"); err != nil {
		t.Errorf("Delete of missing doc should not error: %v", err)
	}
}

func TestVectorStore_SearchWithFiltersAndAccessTracking(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.Store(ctx, &Document{
		ID: "spec", TaskID: "t1", Type: TypeSpecification,
		Content:  "implement the search feature with cosine similarity",
		Metadata: map[string]any{"lang": "go"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Store(ctx, &Document{
		ID: "sess", TaskID: "t1", Type: TypeSession,
		Content: "unrelated session log about deployment",
	}); err != nil {
		t.Fatal(err)
	}

	// Filter to specification type only.
	results, err := store.Search(ctx, "search feature cosine", SearchOptions{
		DocumentTypes: []DocumentType{TypeSpecification},
		Limit:         5,
	})
	if err != nil {
		t.Fatalf("Search error = %v", err)
	}
	if len(results) != 1 || results[0].Document.ID != "spec" {
		t.Fatalf("expected only spec doc, got %+v", results)
	}
	// Access tracking should have incremented.
	if results[0].Document.AccessCount < 1 {
		t.Errorf("AccessCount = %d, want >= 1", results[0].Document.AccessCount)
	}

	// Metadata filter that matches.
	mr, err := store.Search(ctx, "search", SearchOptions{MetadataFilters: map[string]any{"lang": "go"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(mr) != 1 {
		t.Errorf("metadata filter matched %d, want 1", len(mr))
	}

	// Metadata filter that doesn't match.
	nr, err := store.Search(ctx, "search", SearchOptions{MetadataFilters: map[string]any{"lang": "rust"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(nr) != 0 {
		t.Errorf("non-matching metadata filter returned %d", len(nr))
	}
}

func TestVectorStore_SearchTimeRangeFilter(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	old := time.Now().Add(-48 * time.Hour)
	recent := time.Now().Add(-1 * time.Hour)
	if err := store.Store(ctx, &Document{ID: "old", Type: TypeSpecification, Content: "old spec", CreatedAt: old}); err != nil {
		t.Fatal(err)
	}
	if err := store.Store(ctx, &Document{ID: "recent", Type: TypeSpecification, Content: "recent spec", CreatedAt: recent}); err != nil {
		t.Fatal(err)
	}

	results, err := store.Search(ctx, "spec", SearchOptions{
		TimeRange: &TimeRange{From: time.Now().Add(-24 * time.Hour)},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.Document.ID == "old" {
			t.Error("old document should be excluded by TimeRange.From")
		}
	}

	results, err = store.Search(ctx, "spec", SearchOptions{
		TimeRange: &TimeRange{To: time.Now().Add(-24 * time.Hour)},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.Document.ID == "recent" {
			t.Error("recent document should be excluded by TimeRange.To")
		}
	}
}

func TestActivationScore_Branches(t *testing.T) {
	now := time.Now()

	// Branch 1: with AccessTimes -> full ACT-R, must be in (0,1].
	withTimes := &Document{AccessTimes: []time.Time{now.Add(-time.Hour), now.Add(-2 * time.Hour)}}
	if s := activationScore(withTimes, now); s <= 0 || s > 1 {
		t.Errorf("ACT-R activation = %f, want (0,1]", s)
	}

	// Branch 2: brand-new doc (age <= 1h) -> 1.0.
	fresh := &Document{CreatedAt: now.Add(-30 * time.Minute)}
	if s := activationScore(fresh, now); s != 1.0 {
		t.Errorf("fresh doc activation = %f, want 1.0", s)
	}

	// Branch 3: old doc with AccessCount -> recency*frequency.
	oldFreq := &Document{CreatedAt: now.Add(-100 * time.Hour), AccessCount: 5}
	if s := activationScore(oldFreq, now); s <= 0 || s > 1 {
		t.Errorf("old+freq activation = %f, want (0,1]", s)
	}

	// Branch 4: old doc no accesses -> recency only.
	oldOnly := &Document{CreatedAt: now.Add(-100 * time.Hour)}
	if s := activationScore(oldOnly, now); s <= 0 || s > 1 {
		t.Errorf("old-only activation = %f, want (0,1]", s)
	}
}

func TestVectorStore_GetAllDocuments(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	for _, id := range []string{"a", "b"} {
		if err := store.Store(ctx, &Document{ID: id, TaskID: "t", Type: TypeSpecification, Content: "c " + id}); err != nil {
			t.Fatal(err)
		}
	}
	all := store.GetAllDocuments(ctx)
	if len(all) != 2 {
		t.Errorf("GetAllDocuments returned %d, want 2", len(all))
	}
	// Must be copies, not internal pointers.
	all[0].Content = "mutated"
	again := store.GetAllDocuments(ctx)
	for _, d := range again {
		if d.Content == "mutated" {
			t.Error("GetAllDocuments returned mutable internal pointers")
		}
	}
}

func TestVectorStore_FilePathSanitizes(t *testing.T) {
	store := newTestStore(t)
	// IDs containing path separators must be reduced to a base name.
	p := store.filePath("../../etc/passwd")
	if filepath.Dir(p) != store.dir {
		t.Errorf("filePath escaped store dir: %q", p)
	}
	// "." and "/" collapse to "_" so they remain valid filenames.
	if got := store.filePath("."); filepath.Base(got) != "_.json" {
		t.Errorf("filePath(.) = %q, want _.json base", got)
	}
	if got := store.filePath("/"); filepath.Base(got) != "_.json" {
		t.Errorf("filePath(/) = %q, want _.json base", got)
	}
}

func TestVectorStore_ClearMissingDirIsNoop(t *testing.T) {
	dir := t.TempDir()
	store, err := NewVectorStore(dir, NewHashEmbedder(0))
	if err != nil {
		t.Fatal(err)
	}
	// Remove the backing directory entirely; Clear must tolerate it.
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := store.Clear(context.Background()); err != nil {
		t.Errorf("Clear with missing dir should be a no-op, got %v", err)
	}
}

func TestVectorStore_StoreSkipsEmbeddingWhenPresent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	// Pre-supplied embedding must be kept verbatim (no re-embed).
	custom := []float32{1, 2, 3}
	doc := &Document{ID: "pre", Type: TypeSpecification, Content: "x", Embedding: custom}
	if err := store.Store(ctx, doc); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(ctx, "pre")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Embedding) != 3 {
		t.Errorf("embedding overwritten: %v", got.Embedding)
	}
}
