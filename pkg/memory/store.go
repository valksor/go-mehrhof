package memory

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

const maxAccessTimes = 50

// VectorStore is a file-backed, in-memory vector store that persists documents
// as individual JSON files under a configurable directory.  Search uses cosine
// similarity.
type VectorStore struct {
	mu        sync.RWMutex
	dir       string
	embedder  Embedder
	documents map[string]*Document
}

// NewVectorStore creates a VectorStore rooted at dir.  Existing JSON documents
// are loaded eagerly.  embedder is used when a document is stored without a
// pre-computed embedding.
func NewVectorStore(dir string, embedder Embedder) (*VectorStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create vector store dir: %w", err)
	}

	vs := &VectorStore{
		dir:       dir,
		embedder:  embedder,
		documents: make(map[string]*Document),
	}

	if err := vs.load(); err != nil {
		return nil, fmt.Errorf("load existing documents: %w", err)
	}

	return vs, nil
}

// Store saves a document to the store.  If the document has no embedding and
// non-empty content the store generates one via the configured Embedder.
//
// Deduplication: before storing, Jaccard similarity is checked against existing
// documents of the same type.  Similarity >0.85 skips the store (already exists),
// 0.6–0.85 merges into the existing document, <0.6 creates a new entry.
func (vs *VectorStore) Store(ctx context.Context, doc *Document) error {
	if doc.ID == "" {
		return errors.New("document must have an ID")
	}

	// Generate embedding when absent.
	if len(doc.Embedding) == 0 && doc.Content != "" {
		emb, err := vs.embedder.Embed(ctx, doc.Content)
		if err != nil {
			return fmt.Errorf("generate embedding: %w", err)
		}
		doc.Embedding = emb
	}

	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = time.Now()
	}

	vs.mu.Lock()

	// Dedup: check for similar existing documents of the same type.
	if doc.Content != "" {
		if existing, sim := vs.findSimilarLocked(doc.Content, doc.Type); existing != nil {
			if sim > 0.85 {
				// Near-duplicate — skip storing.
				vs.mu.Unlock()

				return nil
			}
			// Moderate overlap — merge into existing document.
			existing.Content = doc.Content
			existing.Embedding = doc.Embedding
			existing.CreatedAt = time.Now()
			persistErr := vs.persist(existing)
			vs.mu.Unlock()

			return persistErr
		}
	}

	vs.documents[doc.ID] = doc
	persistErr := vs.persist(doc)
	vs.mu.Unlock()

	return persistErr
}

// findSimilarLocked returns the most similar existing document of the same type
// with Jaccard similarity >= 0.6.  Must be called with vs.mu held.
func (vs *VectorStore) findSimilarLocked(content string, docType DocumentType) (*Document, float64) {
	newTokens := tokenize(content)
	if len(newTokens) == 0 {
		return nil, 0
	}

	var (
		bestDoc *Document
		bestSim float64
	)

	for _, doc := range vs.documents {
		if doc.Type != docType || doc.Content == "" {
			continue
		}
		sim := jaccardSimilarity(newTokens, tokenize(doc.Content))
		if sim >= 0.6 && sim > bestSim {
			bestSim = sim
			bestDoc = doc
		}
	}

	return bestDoc, bestSim
}

// tokenize splits content into a set of lowercased words.
func tokenize(content string) map[string]struct{} {
	words := strings.Fields(content)
	set := make(map[string]struct{}, len(words))
	for _, w := range words {
		set[strings.ToLower(w)] = struct{}{}
	}

	return set
}

// jaccardSimilarity computes |A ∩ B| / |A ∪ B| for two token sets.
func jaccardSimilarity(a, b map[string]struct{}) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}

	intersection := 0
	for k := range a {
		if _, ok := b[k]; ok {
			intersection++
		}
	}

	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}

	return float64(intersection) / float64(union)
}

// Search performs cosine-similarity search with ACT-R activation scoring.
// The final score is a weighted combination of cosine similarity (70%) and
// activation (30%), where activation captures recency and access frequency.
// Inspired by Soul Protocol's ACT-R cognitive architecture.
func (vs *VectorStore) Search(ctx context.Context, query string, opts SearchOptions) ([]*SearchResult, error) {
	queryEmb, err := vs.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	now := time.Now()

	vs.mu.RLock()

	type candidate struct {
		doc   *Document
		score float32
	}

	var candidates []candidate
	for _, doc := range vs.documents {
		if !matchesFilter(doc, opts) {
			continue
		}
		if len(doc.Embedding) == 0 {
			continue
		}
		cosineSim := cosineSimilarity(queryEmb, doc.Embedding)
		activation := activationScore(doc, now)
		importance := doc.Importance // 0.0-1.0 (0 when unset)
		// Weighted: 60% similarity, 25% activation, 15% importance.
		// When importance is 0 (unset), effectively 60% similarity + 25% activation.
		finalScore := 0.6*float64(cosineSim) + 0.25*activation + 0.15*importance

		if opts.MinScore > 0 && float32(finalScore) < opts.MinScore {
			continue
		}
		candidates = append(candidates, candidate{doc: doc, score: float32(finalScore)})
	}

	vs.mu.RUnlock()

	slices.SortFunc(candidates, func(a, b candidate) int {
		return cmp.Compare(b.score, a.score)
	})

	count := len(candidates)
	if opts.Limit > 0 && opts.Limit < count {
		count = opts.Limit
	}

	results := make([]*SearchResult, count)
	for i := range count {
		results[i] = &SearchResult{
			Document: candidates[i].doc,
			Score:    candidates[i].score,
		}
	}

	// Update access tracking for returned results and persist changes.
	if len(results) > 0 {
		vs.mu.Lock()
		for _, r := range results {
			r.Document.AccessCount++
			r.Document.LastAccessed = now
			r.Document.AccessTimes = append(r.Document.AccessTimes, now)
			if len(r.Document.AccessTimes) > maxAccessTimes {
				r.Document.AccessTimes = r.Document.AccessTimes[len(r.Document.AccessTimes)-maxAccessTimes:]
			}
		}
		// Deep-copy documents for persistence outside the lock to avoid
		// racing with concurrent Store() calls on the same pointer.
		dirty := make([]Document, len(results))
		for i, r := range results {
			dirty[i] = *r.Document
		}
		vs.mu.Unlock()

		// Persist updated access counts to disk so they survive restarts.
		for i := range dirty {
			if err := vs.persist(&dirty[i]); err != nil {
				slog.Warn("persist access tracking", "id", dirty[i].ID, "error", err)
			}
		}
	}

	return results, nil
}

// SearchByCategory returns all documents matching a category, sorted by importance then recency.
func (vs *VectorStore) SearchByCategory(category string, limit int) []*Document {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	var matches []*Document
	for _, doc := range vs.documents {
		if doc.Category == category {
			matches = append(matches, doc)
		}
	}

	slices.SortFunc(matches, func(a, b *Document) int {
		// Higher importance first, then newer first.
		if a.Importance != b.Importance {
			return cmp.Compare(b.Importance, a.Importance)
		}

		return b.CreatedAt.Compare(a.CreatedAt)
	})

	if limit > 0 && limit < len(matches) {
		matches = matches[:limit]
	}

	return matches
}

// activationScore computes an ACT-R-inspired activation value.
// When AccessTimes are available, uses the full ACT-R formula B_i = ln(Σ t_j^(-d))
// which captures temporal distribution of accesses.
func activationScore(doc *Document, now time.Time) float64 {
	// Full ACT-R when timestamp history is available.
	if len(doc.AccessTimes) > 0 {
		var sum float64
		for _, t := range doc.AccessTimes {
			ageSec := now.Sub(t).Seconds() + 1 // +1 to avoid zero
			sum += math.Pow(ageSec, -0.5)      // decay exponent d=0.5
		}

		return math.Min(math.Log(sum+1)/3.0, 1.0) // normalize to [0, 1]
	}

	// Recency of creation with optional frequency boost from AccessCount.
	ageHours := now.Sub(doc.CreatedAt).Hours() + 1 // +1 to avoid log(0)
	if ageHours <= 1.0 {
		return 1.0 // brand-new docs get max score
	}

	recency := 1.0 / math.Log(ageHours)

	if doc.AccessCount > 0 {
		frequency := math.Log(float64(doc.AccessCount) + 1)

		return math.Min(recency*frequency, 1.0)
	}

	return math.Min(recency, 1.0)
}

// Delete removes a document by ID from memory and disk.
func (vs *VectorStore) Delete(_ context.Context, id string) error {
	vs.mu.Lock()
	delete(vs.documents, id)
	vs.mu.Unlock()

	path := vs.filePath(id)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove document file: %w", err)
	}

	return nil
}

// Get retrieves a document by ID.
func (vs *VectorStore) Get(_ context.Context, id string) (*Document, error) {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	doc, ok := vs.documents[id]
	if !ok {
		return nil, fmt.Errorf("document not found: %s", id)
	}

	return doc, nil
}

// Clear removes all documents from memory and disk.
func (vs *VectorStore) Clear(_ context.Context) error {
	vs.mu.Lock()
	vs.documents = make(map[string]*Document)
	vs.mu.Unlock()

	entries, err := os.ReadDir(vs.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("read store dir: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		if err := os.Remove(filepath.Join(vs.dir, e.Name())); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", e.Name(), err)
		}
	}

	return nil
}

// Stats returns aggregate counts for the store, including the active embedder.
func (vs *VectorStore) Stats() Stats {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	byType := make(map[string]int)
	for _, doc := range vs.documents {
		byType[string(doc.Type)]++
	}

	return Stats{
		TotalDocuments: len(vs.documents),
		ByType:         byType,
		Embedder:       vs.embedder.Name(),
	}
}

// --- internal helpers ---

func (vs *VectorStore) filePath(id string) string {
	// Sanitise the ID so it is safe as a filename.
	safe := filepath.Base(id)
	if safe == "." || safe == "/" {
		safe = "_"
	}

	return filepath.Join(vs.dir, safe+".json")
}

func (vs *VectorStore) persist(doc *Document) error {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal document: %w", err)
	}
	if err := os.WriteFile(vs.filePath(doc.ID), data, 0o644); err != nil {
		return fmt.Errorf("write document: %w", err)
	}

	return nil
}

func (vs *VectorStore) load() error {
	entries, err := os.ReadDir(vs.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("read store dir: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(vs.dir, e.Name()))
		if err != nil {
			return fmt.Errorf("read %s: %w", e.Name(), err)
		}
		var doc Document
		if err := json.Unmarshal(data, &doc); err != nil {
			return fmt.Errorf("unmarshal %s: %w", e.Name(), err)
		}
		vs.documents[doc.ID] = &doc
	}

	return nil
}

// matchesFilter returns true when doc passes all SearchOptions filters.
func matchesFilter(doc *Document, opts SearchOptions) bool {
	if len(opts.DocumentTypes) > 0 {
		matched := false
		for _, dt := range opts.DocumentTypes {
			if doc.Type == dt {
				matched = true

				break
			}
		}
		if !matched {
			return false
		}
	}

	if opts.TimeRange != nil {
		if !opts.TimeRange.From.IsZero() && doc.CreatedAt.Before(opts.TimeRange.From) {
			return false
		}
		if !opts.TimeRange.To.IsZero() && doc.CreatedAt.After(opts.TimeRange.To) {
			return false
		}
	}

	for k, v := range opts.MetadataFilters {
		docVal, ok := doc.Metadata[k]
		if !ok {
			return false
		}
		if fmt.Sprintf("%v", docVal) != fmt.Sprintf("%v", v) {
			return false
		}
	}

	return true
}

// cosineSimilarity computes the cosine similarity between two vectors.
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}
