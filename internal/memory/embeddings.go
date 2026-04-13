package memory

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math"
	"strings"
)

const (
	// defaultEmbeddingDim is the dimension used by HashEmbedder.
	defaultEmbeddingDim = 384
	// maxTextLength caps text before hashing.
	maxTextLength = 8191
)

// HashEmbedder produces deterministic, SHA-256-based embedding vectors.
// It requires no external dependencies and works offline.  Semantic
// similarity is not captured — documents with similar content will not
// necessarily score higher than unrelated ones.  Use this as a fallback
// until an ONNX model is downloaded.
type HashEmbedder struct {
	dimension int
}

// NewHashEmbedder creates a HashEmbedder with the given vector dimension.
// If dimension is 0 the default (384) is used.
func NewHashEmbedder(dimension int) *HashEmbedder {
	if dimension <= 0 {
		dimension = defaultEmbeddingDim
	}

	return &HashEmbedder{dimension: dimension}
}

// Dimension returns the length of each embedding vector.
func (h *HashEmbedder) Dimension() int {
	return h.dimension
}

// Embed generates a normalised embedding vector for text.
func (h *HashEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	if len(text) > maxTextLength {
		text = text[:maxTextLength]
	}
	text = strings.ToLower(strings.TrimSpace(text))
	hash := sha256.Sum256([]byte(text))

	return hashToVector(hash[:], h.dimension), nil
}

// EmbedBatch generates embeddings for multiple texts.
func (h *HashEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	return batchEmbed(ctx, texts, h.Embed)
}

// hashToVector converts raw hash bytes into a normalised float32 vector of
// the requested dimension.
func hashToVector(hashBytes []byte, dim int) []float32 {
	v := make([]float32, dim)
	for i := range dim {
		idx := (i * 4) % len(hashBytes)
		val := float32(hashBytes[idx])
		val += float32(i) * 0.01
		v[i] = (val / 128.0) - 1.0
	}

	// L2 normalise
	var norm float32
	for _, x := range v {
		norm += x * x
	}
	norm = float32(math.Sqrt(float64(norm)))
	if norm > 0 {
		for i := range v {
			v[i] /= norm
		}
	}

	return v
}

// batchEmbed is a helper that calls embedFn sequentially for each text.
func batchEmbed(ctx context.Context, texts []string, embedFn func(context.Context, string) ([]float32, error)) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		emb, err := embedFn(ctx, t)
		if err != nil {
			return nil, fmt.Errorf("embed text %d: %w", i, err)
		}
		out[i] = emb
	}

	return out, nil
}

// Name returns the embedder type identifier for stats reporting.
func (h *HashEmbedder) Name() string { return "hash" }
