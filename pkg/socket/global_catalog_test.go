package socket

import (
	"context"
	"encoding/json"
	"testing"
)

func TestHandleCatalogList_NoCatalog(t *testing.T) {
	old := catalogInstance
	catalogInstance = nil
	t.Cleanup(func() { catalogInstance = old })

	ctx := context.Background()
	g := newTestGlobalSocket(t)

	req := &Request{ID: "1", Method: "catalog.list"}
	resp, err := g.handleCatalogList(ctx, req)
	if err != nil {
		t.Fatalf("handleCatalogList() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleCatalogList() returned error: %s", resp.Error.Message)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	templates, ok := result["templates"].([]any)
	if !ok {
		t.Fatal("templates should be an array")
	}
	if len(templates) != 0 {
		t.Errorf("templates length = %d, want 0", len(templates))
	}
}

func TestHandleCatalogGet_NoCatalog(t *testing.T) {
	old := catalogInstance
	catalogInstance = nil
	t.Cleanup(func() { catalogInstance = old })

	ctx := context.Background()
	g := newTestGlobalSocket(t)

	req := &Request{ID: "2", Method: "catalog.get", Params: json.RawMessage(`{"name":"test"}`)}
	resp, err := g.handleCatalogGet(ctx, req)
	if err != nil {
		t.Fatalf("handleCatalogGet() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("handleCatalogGet() should return error when catalog is nil")
	}
	if resp.Error.Message != "catalog not configured" {
		t.Errorf("error message = %q, want %q", resp.Error.Message, "catalog not configured")
	}
}

func TestHandleCatalogImport_NoCatalog(t *testing.T) {
	old := catalogInstance
	catalogInstance = nil
	t.Cleanup(func() { catalogInstance = old })

	ctx := context.Background()
	g := newTestGlobalSocket(t)

	req := &Request{ID: "3", Method: "catalog.import", Params: json.RawMessage(`{"path":"/tmp/test.yaml"}`)}
	resp, err := g.handleCatalogImport(ctx, req)
	if err != nil {
		t.Fatalf("handleCatalogImport() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("handleCatalogImport() should return error when catalog is nil")
	}
	if resp.Error.Message != "catalog not configured" {
		t.Errorf("error message = %q, want %q", resp.Error.Message, "catalog not configured")
	}
}
