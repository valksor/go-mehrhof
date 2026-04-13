package memory

import (
	"context"
	"maps"
	"math"
	"slices"
	"strings"
	"unicode"
)

// tfidfEmbedDim is the fixed dimension for TF-IDF embeddings.
// We use 384 to match all-MiniLM-L6-v2 so stores are interchangeable.
const tfidfEmbedDim = 384

// TFIDFEmbedder produces 384-dimensional bag-of-words TF-IDF embeddings.
// It is pure Go with no external dependencies and provides genuinely useful
// semantic similarity for code and task retrieval — far better than a hash.
//
// The vocabulary is built on-the-fly from all texts seen during the process
// lifetime.  Embeddings are stable for the same vocabulary state and input.
type TFIDFEmbedder struct {
	// vocab maps normalised term → column index (up to tfidfEmbedDim distinct terms).
	vocab map[string]int
	// df tracks the document frequency for each term (how many texts contained it).
	df map[string]int
	// docCount is the number of texts seen so far (for IDF calculation).
	docCount int
}

// NewTFIDFEmbedder creates a TFIDFEmbedder with an empty vocabulary.
func NewTFIDFEmbedder() *TFIDFEmbedder {
	return &TFIDFEmbedder{
		vocab: make(map[string]int),
		df:    make(map[string]int),
	}
}

// Dimension returns 384, matching all-MiniLM-L6-v2 output size.
func (t *TFIDFEmbedder) Dimension() int {
	return tfidfEmbedDim
}

// Embed returns a TF-IDF embedding vector for text.
func (t *TFIDFEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	terms := tokeniseText(text)
	if len(terms) == 0 {
		return make([]float32, tfidfEmbedDim), nil
	}

	// Update vocabulary and document frequencies.
	t.updateVocab(terms)
	t.docCount++

	// Term frequency for this document.
	tf := computeTF(terms)

	// Build the dense vector.
	vec := make([]float32, tfidfEmbedDim)
	for term, freq := range tf {
		idx, ok := t.vocab[term]
		if !ok || idx >= tfidfEmbedDim {
			continue
		}
		idf := t.idf(term)
		vec[idx] += float32(freq) * float32(idf) //nolint:gosec // idx bounds checked above
	}

	return l2NormFloat32(vec), nil
}

// EmbedBatch returns embeddings for multiple texts.
func (t *TFIDFEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	return batchEmbed(ctx, texts, t.Embed)
}

// updateVocab adds new terms to the vocabulary and increments document
// frequencies for terms seen in this document.
func (t *TFIDFEmbedder) updateVocab(terms []string) {
	seen := make(map[string]bool)
	for _, term := range terms {
		if _, exists := t.vocab[term]; !exists {
			// Assign column index.  When we exceed tfidfEmbedDim unique terms,
			// hash the term into an existing bucket (hashing trick).
			if len(t.vocab) < tfidfEmbedDim {
				t.vocab[term] = len(t.vocab)
			} else {
				t.vocab[term] = termHash(term, tfidfEmbedDim)
			}
		}
		if !seen[term] {
			t.df[term]++
			seen[term] = true
		}
	}
}

// idf returns the inverse document frequency for a term.
// Uses log(1 + N / (1 + df)) with +1 smoothing.
func (t *TFIDFEmbedder) idf(term string) float64 {
	df := t.df[term]
	n := t.docCount
	if n == 0 {
		n = 1
	}

	return math.Log(1.0 + float64(n)/float64(1+df))
}

// --- helpers ---

// tokeniseText splits text into normalised, filtered tokens.
func tokeniseText(text string) []string {
	text = strings.ToLower(text)

	var tokens []string
	var buf strings.Builder

	flush := func() {
		if buf.Len() >= 2 { // Skip single-char tokens
			tokens = append(tokens, buf.String())
		}
		buf.Reset()
	}

	for _, r := range text {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_':
			buf.WriteRune(r)
		case r == '-' || r == '.' || r == '/':
			// Keep as word boundary but allow compound words (e.g. "go-kit").
			buf.WriteRune(r)
		default:
			flush()
		}
	}
	flush()

	// Remove stopwords.
	return filterStopwords(tokens)
}

// computeTF returns raw term frequencies.
func computeTF(terms []string) map[string]float64 {
	freq := make(map[string]float64, len(terms))
	for _, t := range terms {
		freq[t]++
	}
	n := float64(len(terms))
	for k := range freq {
		freq[k] /= n
	}

	return freq
}

// termHash maps a term string to a bucket index in [0, dim).
func termHash(term string, dim int) int { //nolint:unparam // dim is a configuration knob; currently always 384 but callers may vary it
	var h uint32 = 2166136261
	for i := range len(term) {
		h ^= uint32(term[i])
		h *= 16777619
	}
	v := int(h) % dim
	if v < 0 {
		v += dim
	}

	return v
}

// l2NormFloat32 returns the L2-normalised copy of vec (in-place).
func l2NormFloat32(vec []float32) []float32 {
	var norm float32
	for _, v := range vec {
		norm += v * v
	}
	norm = float32(math.Sqrt(float64(norm)))
	if norm > 0 {
		for i := range vec {
			vec[i] /= norm
		}
	}

	return vec
}

// stopwords is a small set of common English words that add no signal.
var stopwords = func() map[string]bool {
	words := []string{
		"a", "an", "the", "is", "it", "in", "on", "at", "to", "of",
		"for", "and", "or", "but", "not", "be", "as", "by", "do",
		"go", "if", "no", "we", "he", "she", "so", "up", "my",
		"us", "me", "hi",
	}
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}

	return m
}()

// filterStopwords removes common stopwords from a token list.
func filterStopwords(tokens []string) []string {
	out := tokens[:0]
	for _, t := range tokens {
		if !stopwords[t] {
			out = append(out, t)
		}
	}

	return out
}

// sortedVocabKeys returns vocab keys sorted for deterministic indexing.
// Used only for testing / debugging.
func sortedVocabKeys(vocab map[string]int) []string {
	return slices.Sorted(maps.Keys(vocab))
}

// ensure sortedVocabKeys is referenced to avoid linter complaints.
var _ = sortedVocabKeys

// Name returns the embedder type identifier for stats reporting.
func (t *TFIDFEmbedder) Name() string { return "tfidf" }
