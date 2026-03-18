package memory

import (
	"context"
	"math"
	"strings"
)

// SignificanceThreshold is the minimum significance score (0–1) for content
// to be stored in the vector store. Content below this threshold is silently
// dropped. Inspired by Soul Protocol's memory significance gate.
const SignificanceThreshold = 0.35

// SignificanceScorer evaluates whether content is worth storing in memory.
// It scores content along four dimensions: novelty, richness, error severity,
// and goal relevance — weighted to prioritize novel, information-dense content.
type SignificanceScorer struct {
	store    *VectorStore
	embedder Embedder
}

// NewSignificanceScorer creates a scorer backed by the given store and embedder.
func NewSignificanceScorer(store *VectorStore, embedder Embedder) *SignificanceScorer {
	return &SignificanceScorer{
		store:    store,
		embedder: embedder,
	}
}

// Score returns a 0.0–1.0 significance score for the given content.
// taskSpec is the current task specification (used for goal relevance).
// An empty taskSpec disables the goal relevance dimension.
func (s *SignificanceScorer) Score(ctx context.Context, content, taskSpec string) (float64, error) {
	novelty, err := s.novelty(ctx, content)
	if err != nil {
		// On error, assume content is novel (don't gate on embedder failures).
		novelty = 0.5
	}

	richness := contentRichness(content)
	errSeverity := errorSignificance(content)
	goalRelevance, err := s.goalRelevance(ctx, content, taskSpec)
	if err != nil {
		goalRelevance = 0.5
	}

	// Weighted combination: novelty + richness dominate.
	return 0.3*novelty + 0.3*richness + 0.2*errSeverity + 0.2*goalRelevance, nil
}

// IsSignificant returns true if content exceeds the significance threshold.
func (s *SignificanceScorer) IsSignificant(ctx context.Context, content, taskSpec string) (bool, float64) {
	score, _ := s.Score(ctx, content, taskSpec)

	return score >= SignificanceThreshold, score
}

// novelty measures how different content is from existing stored documents.
// High novelty = different from what's already stored.
func (s *SignificanceScorer) novelty(ctx context.Context, content string) (float64, error) {
	emb, err := s.embedder.Embed(ctx, content)
	if err != nil {
		return 0, err
	}

	s.store.mu.RLock()
	defer s.store.mu.RUnlock()

	if len(s.store.documents) == 0 {
		return 1.0, nil // Everything is novel in an empty store.
	}

	// Find the most similar existing document.
	var maxSim float32
	for _, doc := range s.store.documents {
		if len(doc.Embedding) == 0 {
			continue
		}
		sim := cosineSimilarity(emb, doc.Embedding)
		if sim > maxSim {
			maxSim = sim
		}
	}

	// Novelty = 1 - max_similarity.
	return float64(1 - maxSim), nil
}

// devMarkers are keywords that indicate significant development content.
// Content containing these scores a bonus even if lexical diversity is low
// (e.g., SQL migrations, config changes, error descriptions).
var devMarkers = []string{
	"migration", "endpoint", "regression", "cve", "breaking",
	"schema", "rollback", "deadlock", "race", "security",
	"vulnerability", "deprecat", "backward", "incompatible",
}

// contentRichness measures information density using vocabulary diversity,
// content length, and the presence of development-significant keywords.
// Short, repetitive content scores low; dev-relevant content gets a bonus.
func contentRichness(content string) float64 {
	words := strings.Fields(content)
	if len(words) == 0 {
		return 0
	}

	unique := make(map[string]struct{}, len(words))
	for _, w := range words {
		unique[strings.ToLower(w)] = struct{}{}
	}

	ratio := float64(len(unique)) / float64(len(words))

	// Penalize very short content.
	lengthFactor := math.Min(float64(len(words))/50.0, 1.0)

	// Bonus for dev-significant keywords.
	lower := strings.ToLower(content)
	markerBonus := 0.0
	for _, m := range devMarkers {
		if strings.Contains(lower, m) {
			markerBonus += 0.05
		}
	}
	markerBonus = math.Min(markerBonus, 0.2)

	return math.Min(ratio*lengthFactor+markerBonus, 1.0)
}

// errorKeywords are indicators of error-related content.
var errorKeywords = []string{
	"error", "Error", "ERROR",
	"panic", "PANIC",
	"FAIL", "fatal", "FATAL",
	"stack trace", "stacktrace",
	"exception", "Exception",
}

// errorSignificance scores content based on the presence of error indicators.
// Error-containing content is considered more significant to preserve.
func errorSignificance(content string) float64 {
	count := 0
	for _, kw := range errorKeywords {
		count += strings.Count(content, kw)
	}

	return math.Min(float64(count)/5.0, 1.0)
}

// goalRelevance measures cosine similarity between content and the task spec.
// Returns 0.5 if taskSpec is empty (neutral contribution).
func (s *SignificanceScorer) goalRelevance(ctx context.Context, content, taskSpec string) (float64, error) {
	if taskSpec == "" {
		return 0.5, nil
	}

	contentEmb, err := s.embedder.Embed(ctx, content)
	if err != nil {
		return 0, err
	}

	specEmb, err := s.embedder.Embed(ctx, taskSpec)
	if err != nil {
		return 0, err
	}

	return float64(cosineSimilarity(contentEmb, specEmb)), nil
}
