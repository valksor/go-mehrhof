package socket

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/valksor/kvelmo/pkg/memory"
)

// memoryState lazily creates and caches the global memory adapter.
// It is initialised on first use so startup is not blocked by model probing.
var (
	memMu      sync.Mutex
	memAdapter *memory.Adapter
	errMem     error
	memInited  bool
)

// PrewarmMemory starts background initialization of the memory adapter so
// model download completes before the first memory.search call.
func PrewarmMemory(ctx context.Context) {
	go func() {
		_, _ = getMemoryAdapter(ctx) // result is cached; errors are non-fatal
	}()
}

// getMemoryAdapter returns the singleton global memory adapter, creating it on
// first call.  The result (success or failure) is cached to prevent repeated
// expensive initialization attempts.
// The store is persisted at ~/.valksor/kvelmo/memory/.
func getMemoryAdapter(ctx context.Context) (*memory.Adapter, error) {
	memMu.Lock()
	defer memMu.Unlock()

	if memInited {
		return memAdapter, errMem
	}

	storeDir := filepath.Join(BaseDir(), "memory")
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		errMem = fmt.Errorf("create memory store dir: %w", err)
		memInited = true

		return nil, errMem
	}

	adapter, _, err := memory.NewAdapterAuto(ctx, storeDir)
	if err != nil {
		errMem = fmt.Errorf("init memory adapter: %w", err)
		memInited = true

		return nil, errMem
	}

	memAdapter = adapter
	memInited = true

	return memAdapter, nil
}

// --- memory.search ---

type memorySearchParams struct {
	Query         string   `json:"query"`
	Limit         int      `json:"limit"`
	MinScore      float32  `json:"min_score"`
	DocumentTypes []string `json:"document_types"`
}

type memorySearchResult struct {
	Results []memoryHit `json:"results"`
	Total   int         `json:"total"`
}

type memoryHit struct {
	ID      string                  `json:"id"`
	TaskID  string                  `json:"task_id"`
	Type    string                  `json:"type"`
	Content string                  `json:"content"`
	Score   float32                 `json:"score"`
	Outcome *memory.DocumentOutcome `json:"outcome,omitempty"`
}

func (g *GlobalSocket) handleMemorySearch(ctx context.Context, req *Request) (*Response, error) {
	adapter, err := getMemoryAdapter(ctx)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, fmt.Sprintf("memory unavailable: %s", err)), nil
	}

	var params memorySearchParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
		}
	}

	if params.Query == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "query is required"), nil
	}
	if params.Limit <= 0 {
		params.Limit = 10
	}

	var docTypes []memory.DocumentType
	for _, t := range params.DocumentTypes {
		docTypes = append(docTypes, memory.DocumentType(t))
	}

	results, err := adapter.Store().Search(ctx, params.Query, memory.SearchOptions{
		Limit:         params.Limit,
		MinScore:      params.MinScore,
		DocumentTypes: docTypes,
	})
	if err != nil {
		return NewErrorResponse(req.ID, -32603, fmt.Sprintf("search failed: %s", err)), nil
	}

	hits := make([]memoryHit, len(results))
	for i, r := range results {
		hits[i] = memoryHit{
			ID:      r.Document.ID,
			TaskID:  r.Document.TaskID,
			Type:    string(r.Document.Type),
			Content: r.Document.Content,
			Score:   r.Score,
			Outcome: r.Document.Outcome,
		}
	}

	return NewResultResponse(req.ID, memorySearchResult{
		Results: hits,
		Total:   len(hits),
	})
}

// --- memory.stats ---

// MemoryStatsResponse is the response for memory.stats calls.
type MemoryStatsResponse struct {
	TotalEntries   int   `json:"total_entries"`
	TotalSizeBytes int64 `json:"total_size_bytes"`
	IndexReady     bool  `json:"index_ready"`
}

func (g *GlobalSocket) handleMemoryStats(ctx context.Context, req *Request) (*Response, error) {
	adapter, err := getMemoryAdapter(ctx)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, fmt.Sprintf("memory unavailable: %s", err)), nil
	}

	stats := adapter.Stats()

	// Index is ready when we have a functioning embedder (TF-IDF)
	indexReady := stats.Embedder != "" && stats.Embedder != "hash"

	resp := MemoryStatsResponse{
		TotalEntries:   stats.TotalDocuments,
		TotalSizeBytes: 0,
		IndexReady:     indexReady,
	}

	return NewResultResponse(req.ID, resp)
}

// --- memory.outcomes ---

type memoryOutcomesParams struct {
	TaskID string `json:"task_id"`
}

type memoryOutcomeDoc struct {
	ID      string                  `json:"id"`
	TaskID  string                  `json:"task_id"`
	Type    string                  `json:"type"`
	Content string                  `json:"content"`
	Outcome *memory.DocumentOutcome `json:"outcome,omitempty"`
}

type memoryOutcomesResult struct {
	Documents []memoryOutcomeDoc `json:"documents"`
	Total     int                `json:"total"`
}

func (g *GlobalSocket) handleMemoryOutcomes(ctx context.Context, req *Request) (*Response, error) {
	adapter, err := getMemoryAdapter(ctx)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, fmt.Sprintf("memory unavailable: %s", err)), nil
	}

	var params memoryOutcomesParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
		}
	}

	if params.TaskID == "" {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "task_id is required"), nil
	}

	docs := adapter.Store().GetDocumentsForTask(ctx, params.TaskID)
	result := memoryOutcomesResult{
		Documents: make([]memoryOutcomeDoc, len(docs)),
		Total:     len(docs),
	}
	for i, doc := range docs {
		result.Documents[i] = memoryOutcomeDoc{
			ID:      doc.ID,
			TaskID:  doc.TaskID,
			Type:    string(doc.Type),
			Content: doc.Content,
			Outcome: doc.Outcome,
		}
	}

	return NewResultResponse(req.ID, result)
}

// --- memory.clear ---

func (g *GlobalSocket) handleMemoryClear(ctx context.Context, req *Request) (*Response, error) {
	adapter, err := getMemoryAdapter(ctx)
	if err != nil {
		return NewErrorResponse(req.ID, -32603, fmt.Sprintf("memory unavailable: %s", err)), nil
	}

	if err := adapter.Clear(ctx); err != nil {
		return NewErrorResponse(req.ID, -32603, fmt.Sprintf("clear failed: %s", err)), nil
	}

	return NewResultResponse(req.ID, map[string]bool{"ok": true})
}
