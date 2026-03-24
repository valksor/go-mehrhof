package socket

import (
	"context"
	"encoding/json"
	"testing"
)

func TestHandleReportGenerate_NoActivityLog(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	req := &Request{ID: "1", Method: "report.generate"}
	resp, err := g.handleReportGenerate(ctx, req)
	if err != nil {
		t.Fatalf("handleReportGenerate() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleReportGenerate() returned error: %s", resp.Error.Message)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	md, ok := result["markdown"].(string)
	if !ok {
		t.Fatal("markdown should be a string")
	}
	if md == "" {
		t.Error("markdown should not be empty")
	}

	if _, ok := result["report"]; !ok {
		t.Error("result should contain 'report' key")
	}
}
