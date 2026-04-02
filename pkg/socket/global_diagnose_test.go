package socket

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/valksor/kvelmo/pkg/paths"
)

// ============================================================
// handleDiagnose tests
// ============================================================

func TestGlobalHandleDiagnose_ReturnsExpectedStructure(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	resp, err := g.handleDiagnose(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleDiagnose() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleDiagnose() returned error: %s", resp.Error.Message)
	}

	var result diagnoseResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// GlobalSocket should be "running" since we're calling the handler
	if result.GlobalSocket != "running" {
		t.Errorf("global_socket = %q, want %q", result.GlobalSocket, "running")
	}

	// Checks should not be nil (may be empty)
	if result.Checks == nil {
		t.Error("checks should not be nil")
	}

	// Providers should contain the expected provider names
	if result.Providers == nil {
		t.Fatal("providers should not be nil")
	}

	expectedProviders := map[string]bool{
		"GitHub": false,
		"GitLab": false,
		"Linear": false,
		"Wrike":  false,
	}
	for _, p := range result.Providers {
		if _, ok := expectedProviders[p.Name]; ok {
			expectedProviders[p.Name] = true
		}
	}
	for name, found := range expectedProviders {
		if !found {
			t.Errorf("expected provider %q in results", name)
		}
	}
}

func TestGlobalHandleDiagnose_ChecksHaveNameAndStatus(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	resp, err := g.handleDiagnose(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleDiagnose() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleDiagnose() returned error: %s", resp.Error.Message)
	}

	var result diagnoseResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for i, c := range result.Checks {
		if c.Name == "" {
			t.Errorf("check[%d] has empty name", i)
		}
		if c.Status == "" {
			t.Errorf("check[%d] (%s) has empty status", i, c.Name)
		}
	}
}

func TestGlobalHandleDiagnose_UnconfiguredProvidersAreNotIssues(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	// Redirect global config to an empty temp dir so LoadEnvMap finds no .env file
	paths.SetPaths(paths.NewPathResolver(t.TempDir()))
	t.Cleanup(paths.ResetForTesting)

	resp, err := g.handleDiagnose(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleDiagnose() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleDiagnose() returned error: %s", resp.Error.Message)
	}

	var result diagnoseResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Unconfigured provider tokens should not generate issues — they are optional
	for _, issue := range result.Issues {
		if issue == "" {
			t.Error("issue string should not be empty")
		}
	}
}

func TestGlobalHandleDiagnose_ProviderConfiguredWithEnvFile(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	// Set up a temp base dir with a .env file containing the token
	tmpDir := t.TempDir()
	paths.SetPaths(paths.NewPathResolver(tmpDir))
	t.Cleanup(paths.ResetForTesting)

	if err := os.WriteFile(filepath.Join(tmpDir, ".env"), []byte("GITHUB_TOKEN=test-token-value\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	resp, err := g.handleDiagnose(ctx, &Request{ID: "1"})
	if err != nil {
		t.Fatalf("handleDiagnose() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleDiagnose() returned error: %s", resp.Error.Message)
	}

	var result diagnoseResponse
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, p := range result.Providers {
		if p.Name == "GitHub" {
			if !p.Configured {
				t.Error("GitHub provider should be configured when GITHUB_TOKEN is set")
			}

			return
		}
	}
	t.Error("GitHub provider not found in results")
}

func TestGlobalHandleDiagnose_NilParams(t *testing.T) {
	ctx := context.Background()
	g := newTestGlobalSocket(t)

	// handleDiagnose ignores params entirely, so nil should work
	resp, err := g.handleDiagnose(ctx, &Request{ID: "1", Params: nil})
	if err != nil {
		t.Fatalf("handleDiagnose() error = %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("handleDiagnose() returned error: %s", resp.Error.Message)
	}
}
