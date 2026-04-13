package socket

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ============================================================
// handleSecurityScan tests
// ============================================================

func TestGlobalHandleSecurityScan_InvalidJSON(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	resp, err := g.handleSecurityScan(ctx, &Request{ID: "1", Params: json.RawMessage(`invalid`)})
	if err != nil {
		t.Fatalf("handleSecurityScan() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for invalid JSON params")
	}
	if resp.Error.Code != ErrCodeInvalidParams {
		t.Errorf("error code = %d, want %d", resp.Error.Code, ErrCodeInvalidParams)
	}
}

func TestGlobalHandleSecurityScan_EmptyDir(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	params, _ := json.Marshal(securityScanParams{Dir: ""}) //nolint:errchkjson // test data
	resp, err := g.handleSecurityScan(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleSecurityScan() error = %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response for empty dir")
	}
	if resp.Error.Code != ErrCodeInvalidParams {
		t.Errorf("error code = %d, want %d", resp.Error.Code, ErrCodeInvalidParams)
	}
}

func TestGlobalHandleSecurityScan_CleanDirectory(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	dir := t.TempDir()
	// Create a harmless source file
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	params, _ := json.Marshal(securityScanParams{Dir: dir}) //nolint:errchkjson // test data
	resp, err := g.handleSecurityScan(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleSecurityScan() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleSecurityScan() returned error: %s", resp.Error.Message)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := result["findings"]; !ok {
		t.Error("result should have 'findings' key")
	}
	if _, ok := result["count"]; !ok {
		t.Error("result should have 'count' key")
	}
	if _, ok := result["scanners"]; !ok {
		t.Error("result should have 'scanners' key")
	}

	var count int
	if err := json.Unmarshal(result["count"], &count); err != nil {
		t.Fatalf("unmarshal count: %v", err)
	}
	// A clean directory should have no secret findings (dependency scanner
	// may report info-level "tool missing" but those still appear in count).
	// We only verify the structure is correct.
}

func TestGlobalHandleSecurityScan_DirectoryWithSecret(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	dir := t.TempDir()
	// Create a file with a fake AWS access key to trigger the secret scanner
	content := `package main

const awsKey = "AKIAIOSFODNN7EXAMPLE"

func main() {}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(content), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	params, _ := json.Marshal(securityScanParams{Dir: dir}) //nolint:errchkjson // test data
	resp, err := g.handleSecurityScan(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleSecurityScan() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleSecurityScan() returned error: %s", resp.Error.Message)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	var count int
	if err := json.Unmarshal(result["count"], &count); err != nil {
		t.Fatalf("unmarshal count: %v", err)
	}
	if count < 1 {
		t.Errorf("expected at least 1 finding for fake AWS key, got %d", count)
	}

	var scanners []string
	if err := json.Unmarshal(result["scanners"], &scanners); err != nil {
		t.Fatalf("unmarshal scanners: %v", err)
	}
	if len(scanners) == 0 {
		t.Error("expected at least one scanner name")
	}
}

func TestGlobalHandleSecurityScan_EmptyDirectory(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	dir := t.TempDir()

	params, _ := json.Marshal(securityScanParams{Dir: dir}) //nolint:errchkjson // test data
	resp, err := g.handleSecurityScan(ctx, &Request{ID: "1", Params: params})
	if err != nil {
		t.Fatalf("handleSecurityScan() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleSecurityScan() returned error: %s", resp.Error.Message)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	var count int
	if err := json.Unmarshal(result["count"], &count); err != nil {
		t.Fatalf("unmarshal count: %v", err)
	}
	// Empty directory should have no secret findings (dependency scanner may add info)
}
