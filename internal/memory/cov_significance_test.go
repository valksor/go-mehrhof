package memory

import (
	"context"
	"strings"
	"testing"
)

func TestSignificanceScorer_Score_EmptyStoreIsNovel(t *testing.T) {
	store := newTestStore(t)
	scorer := NewSignificanceScorer(store, store.embedder)

	score, err := scorer.Score(context.Background(),
		"a brand new piece of content with reasonable length and vocabulary diversity here", "")
	if err != nil {
		t.Fatalf("Score error = %v", err)
	}
	if score <= 0 || score > 1 {
		t.Errorf("score = %f, want (0,1]", score)
	}
}

func TestSignificanceScorer_IsSignificant(t *testing.T) {
	store := newTestStore(t)
	scorer := NewSignificanceScorer(store, store.embedder)
	ctx := context.Background()

	// Rich, error-laden, dev-significant content -> significant.
	rich := "ERROR panic FATAL stack trace migration rollback deadlock race security " +
		"vulnerability schema breaking incompatible regression with many distinct words here " +
		strings.Repeat("unique-word-x ", 30)
	sig, score := scorer.IsSignificant(ctx, rich, "")
	if !sig {
		t.Errorf("rich content should be significant, score = %f", score)
	}

	// Trivial content -> not significant.
	sig, score = scorer.IsSignificant(ctx, "ok", "task about something else entirely")
	if sig {
		t.Errorf("trivial content should not be significant, score = %f", score)
	}
}

func TestSignificanceScorer_NoveltyDecreasesWithSimilarDocs(t *testing.T) {
	store := newTestStore(t)
	scorer := NewSignificanceScorer(store, store.embedder)
	ctx := context.Background()

	content := "implement the websocket reconnection backoff with jitter and heartbeat pings"
	// Empty store: high novelty.
	n1, err := scorer.novelty(ctx, content)
	if err != nil {
		t.Fatal(err)
	}
	if n1 != 1.0 {
		t.Errorf("novelty in empty store = %f, want 1.0", n1)
	}

	// Store the same content, then novelty should drop.
	if err := store.Store(ctx, &Document{ID: "n1", Type: TypeSpecification, Content: content}); err != nil {
		t.Fatal(err)
	}
	n2, err := scorer.novelty(ctx, content)
	if err != nil {
		t.Fatal(err)
	}
	if n2 >= n1 {
		t.Errorf("novelty should decrease after storing similar doc: %f -> %f", n1, n2)
	}
}

func TestSignificanceScorer_GoalRelevance(t *testing.T) {
	store := newTestStore(t)
	scorer := NewSignificanceScorer(store, store.embedder)
	ctx := context.Background()

	// Empty spec -> neutral 0.5.
	gr, err := scorer.goalRelevance(ctx, "anything", "")
	if err != nil {
		t.Fatal(err)
	}
	if gr != 0.5 {
		t.Errorf("empty taskSpec goalRelevance = %f, want 0.5", gr)
	}

	// Identical content and spec -> high relevance (deterministic embedder).
	text := "build the rate limiter with a token bucket algorithm and burst capacity"
	gr, err = scorer.goalRelevance(ctx, text, text)
	if err != nil {
		t.Fatal(err)
	}
	if gr <= 0 {
		t.Errorf("matching content/spec goalRelevance = %f, want > 0", gr)
	}
}

func TestContentRichness(t *testing.T) {
	if contentRichness("") != 0 {
		t.Error("empty content richness should be 0")
	}
	// Dev markers should add a bonus.
	withMarkers := contentRichness("migration schema rollback deadlock race security " +
		strings.Repeat("word ", 60))
	plain := contentRichness(strings.Repeat("word ", 60))
	if withMarkers <= plain {
		t.Errorf("dev-marker content richness %f should exceed plain %f", withMarkers, plain)
	}
	if withMarkers > 1.0 {
		t.Errorf("richness should be capped at 1.0, got %f", withMarkers)
	}
}

func TestErrorSignificance(t *testing.T) {
	if errorSignificance("clean content with no problems") != 0 {
		t.Error("non-error content should score 0")
	}
	s := errorSignificance("error Error ERROR panic FATAL exception")
	if s <= 0 || s > 1.0 {
		t.Errorf("error significance = %f, want (0,1]", s)
	}
	// Many errors should saturate at 1.0.
	many := errorSignificance(strings.Repeat("error panic FATAL ", 20))
	if many != 1.0 {
		t.Errorf("saturated error significance = %f, want 1.0", many)
	}
}

// --- Cybertron embedder (pure methods only; neural model not loaded) ---

func TestCybertronEmbedder_DimensionAndName(t *testing.T) {
	// These methods are constant and don't touch the underlying model, so we
	// can exercise them on a zero-value struct without a downloaded model.
	e := &CybertronEmbedder{}
	if e.Dimension() != 384 {
		t.Errorf("Dimension() = %d, want 384", e.Dimension())
	}
	if e.Name() != "cybertron" {
		t.Errorf("Name() = %q, want cybertron", e.Name())
	}
}
