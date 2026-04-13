package socket

import (
	"context"
)

// handleCacheStats returns response cache performance statistics.
func (w *WorktreeSocket) handleCacheStats(_ context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no conductor"), nil
	}

	stats := w.conductor.ResponseCacheStats()
	if stats == nil {
		return NewResultResponse(req.ID, map[string]any{
			"enabled": false,
		})
	}

	return NewResultResponse(req.ID, map[string]any{
		"enabled":      true,
		"entries":      stats.Entries,
		"hits":         stats.Hits,
		"misses":       stats.Misses,
		"hit_rate":     stats.HitRate,
		"tokens_saved": stats.TokensSaved,
	})
}

// handleCacheClear clears all response cache entries.
func (w *WorktreeSocket) handleCacheClear(_ context.Context, req *Request) (*Response, error) {
	if w.conductor == nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, "no conductor"), nil
	}

	w.conductor.ClearResponseCache()

	return NewResultResponse(req.ID, map[string]string{
		"status": "ok",
	})
}
