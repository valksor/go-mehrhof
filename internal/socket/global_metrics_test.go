package socket

import (
	"context"
	"encoding/json"
	"testing"
)

func TestHandleMetricsHistory_NoStore(t *testing.T) {
	old := timeSeriesStore
	timeSeriesStore = nil
	t.Cleanup(func() { timeSeriesStore = old })

	ctx := context.Background()
	g := newTestGlobalSocket(t)

	req := &Request{ID: "1", Method: "metrics.history"}
	resp, err := g.handleMetricsHistory(ctx, req)
	if err != nil {
		t.Fatalf("handleMetricsHistory() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleMetricsHistory() returned error: %s", resp.Error.Message)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	enabled, ok := result["enabled"].(bool)
	if !ok {
		t.Fatal("enabled should be a boolean")
	}
	if enabled {
		t.Error("enabled = true, want false when store is nil")
	}

	entries, ok := result["entries"].([]any)
	if !ok {
		t.Fatal("entries should be an array")
	}
	if len(entries) != 0 {
		t.Errorf("entries length = %d, want 0", len(entries))
	}
}
