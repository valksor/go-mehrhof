package memory

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Adapter provides high-level memory operations for agent integration.
// It wraps VectorStore and Indexer with convenience methods that are
// called by socket handlers and the Indexer.
type Adapter struct {
	store   *VectorStore
	indexer *Indexer
}

// NewAdapter creates a new Adapter backed by the provided store and indexer.
// Use NewAdapterWithEmbedder to control which embedder the store uses.
func NewAdapter(store *VectorStore, indexer *Indexer) *Adapter {
	return &Adapter{
		store:   store,
		indexer: indexer,
	}
}

// NewAdapterAuto creates a VectorStore with the best available embedder and
// wraps it in an Adapter.
//
// Fallback chain:
//  1. CybertronEmbedder (all-MiniLM-L6-v2) — neural embeddings, pure Go.
//  2. TFIDFEmbedder — always available, semantically useful.
//
// storeDir is the directory used by VectorStore to persist documents.
func NewAdapterAuto(ctx context.Context, storeDir string) (*Adapter, *Indexer, error) {
	embedder := selectEmbedder()

	store, err := NewVectorStore(storeDir, embedder)
	if err != nil {
		return nil, nil, fmt.Errorf("create vector store: %w", err)
	}

	indexer := NewIndexer(store, storeDir)

	return NewAdapter(store, indexer), indexer, nil
}

// selectEmbedder returns the best available embedder:
//  1. CybertronEmbedder if available (neural, pure Go).
//  2. TFIDFEmbedder as fallback.
func selectEmbedder() Embedder {
	modelsDir := defaultModelsDir()
	if modelsDir != "" {
		e, err := NewCybertronEmbedder(modelsDir)
		if err == nil {
			slog.Info("memory: using Cybertron neural embedder", "model", "all-MiniLM-L6-v2")

			return e
		}
		slog.Debug("memory: Cybertron unavailable, falling back to TF-IDF", "err", err)
	}

	slog.Info("memory: using TF-IDF embedder")

	return NewTFIDFEmbedder()
}

// defaultModelsDir returns the directory where models are cached.
func defaultModelsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(home, ".valksor", "kvelmo", "models")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}

	return dir
}

// AugmentPrompt returns a markdown-formatted context block drawn from
// past tasks that are semantically similar to the provided task title
// and description.  Returns an empty string (not an error) when no
// relevant history is found.
func (a *Adapter) AugmentPrompt(ctx context.Context, taskTitle, taskDescription string) (string, error) {
	query := fmt.Sprintf("%s %s", taskTitle, taskDescription)

	results, err := a.store.Search(ctx, query, SearchOptions{
		Limit:         3,
		MinScore:      0.70,
		DocumentTypes: []DocumentType{TypeSpecification, TypeSolution},
	})
	if err != nil {
		return "", fmt.Errorf("search memory: %w", err)
	}

	if len(results) == 0 {
		return "", nil
	}

	var sb strings.Builder
	sb.WriteString("## Relevant Context from Similar Past Tasks\n\n")
	sb.WriteString("The following past tasks may be relevant to the current request:\n\n")

	for _, result := range results {
		doc := result.Document
		fmt.Fprintf(&sb, "### Task %s (%.0f%% similar)\n", doc.TaskID, result.Score*100)
		fmt.Fprintf(&sb, "**Type**: %s\n", doc.Type)

		preview := doc.Content
		if len(preview) > 300 {
			preview = preview[:300] + "..."
		}
		fmt.Fprintf(&sb, "**Content**:\n%s\n\n", preview)
	}

	sb.WriteString("Use this context to inform your approach.\n")

	return sb.String(), nil
}

// SearchSimilarTasks returns formatted text snippets for past tasks
// matching query.  Useful for injecting raw context into prompts.
func (a *Adapter) SearchSimilarTasks(ctx context.Context, query string, limit int) ([]string, error) {
	results, err := a.store.Search(ctx, query, SearchOptions{
		Limit:         limit,
		MinScore:      0.70,
		DocumentTypes: []DocumentType{TypeSpecification, TypeSolution},
	})
	if err != nil {
		return nil, err
	}

	var out []string
	for _, r := range results {
		out = append(out, formatResult(r))
	}

	return out, nil
}

// LearnFromCorrection stores a user correction as a Solution document for
// future retrieval.
func (a *Adapter) LearnFromCorrection(ctx context.Context, taskID, problem, solution string) error {
	doc := &Document{
		ID:      fmt.Sprintf("solution:%s:%s", taskID, uniqueSuffix()),
		TaskID:  taskID,
		Type:    TypeSolution,
		Content: fmt.Sprintf("Problem: %s\n\nSolution: %s", problem, solution),
		Metadata: map[string]any{
			"problem": problem,
		},
		Tags:      []string{"solution", "fix", "learned"},
		CreatedAt: time.Now(),
	}

	return a.store.Store(ctx, doc)
}

// Stats returns aggregate statistics about stored memory.
func (a *Adapter) Stats() Stats {
	return a.store.Stats()
}

// Clear removes all documents from the store.
func (a *Adapter) Clear(ctx context.Context) error {
	return a.store.Clear(ctx)
}

// --- helpers ---

func formatResult(r *SearchResult) string {
	doc := r.Document
	var sb strings.Builder
	fmt.Fprintf(&sb, "## %s (%.2f)\n", doc.TaskID, r.Score)
	fmt.Fprintf(&sb, "Type: %s\n", doc.Type)

	preview := doc.Content
	if len(preview) > 200 {
		preview = preview[:200] + "..."
	}
	fmt.Fprintf(&sb, "\n%s\n\n", preview)

	return sb.String()
}

func uniqueSuffix() string {
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}

// Store returns the underlying VectorStore for direct access.
func (a *Adapter) Store() *VectorStore {
	return a.store
}
