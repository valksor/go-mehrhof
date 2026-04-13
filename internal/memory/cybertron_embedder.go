package memory

import (
	"context"

	"github.com/tmc/langchaingo/embeddings/cybertron"
)

// CybertronEmbedder uses the all-MiniLM-L6-v2 sentence transformer for
// semantic embeddings. Pure Go implementation via nlpodyssey/cybertron.
type CybertronEmbedder struct {
	embedder *cybertron.Cybertron
}

// NewCybertronEmbedder creates an embedder using the Cybertron library.
// modelsDir is where HuggingFace models are downloaded and cached.
// Uses sentence-transformers/all-MiniLM-L6-v2 by default.
func NewCybertronEmbedder(modelsDir string) (*CybertronEmbedder, error) {
	opts := []cybertron.Option{
		cybertron.WithModelsDir(modelsDir),
	}

	e, err := cybertron.NewCybertron(opts...)
	if err != nil {
		return nil, err
	}

	return &CybertronEmbedder{embedder: e}, nil
}

// Embed returns a 384-dimensional embedding for the given text.
func (e *CybertronEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	result, err := e.embedder.CreateEmbedding(ctx, []string{text})
	if err != nil {
		return nil, err
	}

	return result[0], nil
}

// EmbedBatch returns embeddings for multiple texts.
func (e *CybertronEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	return e.embedder.CreateEmbedding(ctx, texts)
}

// Dimension returns the embedding dimension (384 for all-MiniLM-L6-v2).
func (e *CybertronEmbedder) Dimension() int {
	return 384
}

// Name returns the embedder identifier for stats reporting.
func (e *CybertronEmbedder) Name() string {
	return "cybertron"
}
