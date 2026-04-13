package memory

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Indexer auto-indexes task artefacts into the VectorStore when a task
// completes.  It extracts specification files, the git diff, and session
// log files from the project directory.
type Indexer struct {
	store      *VectorStore
	projectDir string              // root of the project (contains .kvelmo/)
	scorer     *SignificanceScorer // optional; nil disables the significance gate
}

// Store returns the underlying VectorStore.
func (idx *Indexer) Store() *VectorStore {
	return idx.store
}

// NewIndexer creates an Indexer for the given project directory.
// The significance gate is enabled automatically using the store's embedder.
func NewIndexer(store *VectorStore, projectDir string) *Indexer {
	return &Indexer{
		store:      store,
		projectDir: projectDir,
		scorer:     NewSignificanceScorer(store, store.embedder),
	}
}

// IndexTask indexes artefacts for a completed task.
//
// taskID is the internal task ID; title and description are used as metadata.
// branch is the git branch containing the implementation; baseBranch is the
// branch it diverged from (used to compute the diff).
func (idx *Indexer) IndexTask(ctx context.Context, taskID, title, description, branch, baseBranch string) error {
	// Use description as the task spec for significance scoring.
	taskSpec := description

	if err := idx.indexSpecifications(ctx, taskID, title); err != nil {
		slog.Warn("index specifications failed", "task_id", taskID, "error", err)
	}

	if err := idx.indexCodeChange(ctx, taskID, title, branch, baseBranch, taskSpec); err != nil {
		slog.Warn("index code change failed", "task_id", taskID, "error", err)
	}

	if err := idx.indexSession(ctx, taskID, title, description, taskSpec); err != nil {
		slog.Warn("index session failed", "task_id", taskID, "error", err)
	}

	return nil
}

// SearchSimilar searches the store for documents similar to query.
func (idx *Indexer) SearchSimilar(ctx context.Context, query string, limit int) ([]*SearchResult, error) {
	return idx.store.Search(ctx, query, SearchOptions{
		Limit:         limit,
		MinScore:      0.7,
		DocumentTypes: []DocumentType{TypeSpecification, TypeCodeChange, TypeSolution},
	})
}

// Stats returns current store statistics.
func (idx *Indexer) Stats() Stats {
	return idx.store.Stats()
}

// storeIfSignificant checks the significance gate before storing a document.
// Specifications and solutions always pass (high-value by definition).
// Session logs and code changes are gated.
func (idx *Indexer) storeIfSignificant(ctx context.Context, doc *Document, taskSpec string) error {
	// Always store specifications and solutions — they're high-value by definition.
	if doc.Type == TypeSpecification || doc.Type == TypeSolution || doc.Type == TypeDecision {
		return idx.store.Store(ctx, doc)
	}

	// Apply significance gate for session logs and code changes.
	if idx.scorer != nil {
		significant, score := idx.scorer.IsSignificant(ctx, doc.Content, taskSpec)
		if !significant {
			slog.Debug("skipping low-significance content",
				"id", doc.ID, "type", doc.Type, "score", fmt.Sprintf("%.2f", score))

			return nil
		}
	}

	return idx.store.Store(ctx, doc)
}

// --- private helpers ---

func (idx *Indexer) indexSpecifications(ctx context.Context, taskID, title string) error {
	specDir := filepath.Join(idx.projectDir, ".kvelmo", "specifications")
	entries, err := os.ReadDir(specDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("read spec dir: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(specDir, e.Name()))
		if err != nil {
			slog.Warn("read specification file", "file", e.Name(), "error", err)

			continue
		}
		doc := &Document{
			ID:      fmt.Sprintf("specification:%s:%s", taskID, e.Name()),
			TaskID:  taskID,
			Type:    TypeSpecification,
			Content: string(data),
			Metadata: map[string]any{
				"title": title,
				"file":  e.Name(),
			},
			Tags:      []string{"specification", "completed"},
			CreatedAt: time.Now(),
		}
		if err := idx.store.Store(ctx, doc); err != nil {
			slog.Warn("store specification document", "task_id", taskID, "error", err)
		}
	}

	return nil
}

func (idx *Indexer) indexCodeChange(ctx context.Context, taskID, title, branch, baseBranch, taskSpec string) error {
	if branch == "" {
		return nil
	}
	diff := gitDiff(ctx, idx.projectDir, baseBranch, branch)
	if strings.TrimSpace(diff) == "" {
		return nil
	}

	doc := &Document{
		ID:      "code_change:" + taskID,
		TaskID:  taskID,
		Type:    TypeCodeChange,
		Content: diff,
		Metadata: map[string]any{
			"title":       title,
			"branch":      branch,
			"base_branch": baseBranch,
		},
		Tags:      []string{"code", "diff", "completed"},
		CreatedAt: time.Now(),
	}

	return idx.storeIfSignificant(ctx, doc, taskSpec)
}

func (idx *Indexer) indexSession(ctx context.Context, taskID, title, description, taskSpec string) error {
	logDir := filepath.Join(idx.projectDir, ".kvelmo", "sessions")
	entries, err := os.ReadDir(logDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("read session dir: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), taskID) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(logDir, e.Name()))
		if err != nil {
			continue
		}
		doc := &Document{
			ID:      fmt.Sprintf("session:%s:%s", taskID, e.Name()),
			TaskID:  taskID,
			Type:    TypeSession,
			Content: string(data),
			Metadata: map[string]any{
				"title":       title,
				"description": description,
				"file":        e.Name(),
			},
			Tags:      []string{"session", "completed"},
			CreatedAt: time.Now(),
		}
		if err := idx.storeIfSignificant(ctx, doc, taskSpec); err != nil {
			slog.Warn("store session document", "task_id", taskID, "error", err)
		}
	}

	return nil
}

// gitDiff runs git diff between two refs in dir.
func gitDiff(ctx context.Context, dir, from, to string) string {
	if from == "" {
		from = "HEAD"
	}
	cmd := exec.CommandContext(ctx, "git", "diff", from+"..."+to)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		// Non-zero exit is common when refs don't exist; treat as empty diff.
		return ""
	}

	return string(out)
}
