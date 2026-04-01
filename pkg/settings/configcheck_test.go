package settings

import (
	"testing"
)

func TestCheckConfigDrift_NoDrift(t *testing.T) {
	ref := map[string]any{
		"host": "localhost",
		"port": 8080,
	}
	actual := map[string]any{
		"host": "localhost",
		"port": 8080,
	}

	drifts := CheckConfigDrift(ref, actual)
	if len(drifts) != 0 {
		t.Fatalf("expected no drift, got %d: %+v", len(drifts), drifts)
	}
}

func TestCheckConfigDrift_MissingKey(t *testing.T) {
	ref := map[string]any{
		"host": "localhost",
		"port": 8080,
	}
	actual := map[string]any{
		"host": "localhost",
	}

	drifts := CheckConfigDrift(ref, actual)
	if len(drifts) != 1 {
		t.Fatalf("expected 1 drift, got %d: %+v", len(drifts), drifts)
	}

	if drifts[0].Path != "port" {
		t.Errorf("expected path 'port', got %q", drifts[0].Path)
	}

	if drifts[0].Actual != nil {
		t.Errorf("expected nil actual, got %v", drifts[0].Actual)
	}
}

func TestCheckConfigDrift_DifferentValue(t *testing.T) {
	ref := map[string]any{
		"host": "localhost",
		"port": 8080,
	}
	actual := map[string]any{
		"host": "localhost",
		"port": 9090,
	}

	drifts := CheckConfigDrift(ref, actual)
	if len(drifts) != 1 {
		t.Fatalf("expected 1 drift, got %d: %+v", len(drifts), drifts)
	}

	if drifts[0].Path != "port" {
		t.Errorf("expected path 'port', got %q", drifts[0].Path)
	}

	if drifts[0].Expected != 8080 {
		t.Errorf("expected Expected=8080, got %v", drifts[0].Expected)
	}

	if drifts[0].Actual != 9090 {
		t.Errorf("expected Actual=9090, got %v", drifts[0].Actual)
	}
}

func TestCheckConfigDrift_NestedDrift(t *testing.T) {
	ref := map[string]any{
		"database": map[string]any{
			"host": "db.local",
			"port": 5432,
		},
		"cache": map[string]any{
			"ttl": 300,
		},
	}
	actual := map[string]any{
		"database": map[string]any{
			"host": "db.remote",
			"port": 5432,
		},
		"cache": map[string]any{
			"ttl": 600,
		},
	}

	drifts := CheckConfigDrift(ref, actual)
	if len(drifts) != 2 {
		t.Fatalf("expected 2 drifts, got %d: %+v", len(drifts), drifts)
	}

	paths := make(map[string]bool)
	for _, d := range drifts {
		paths[d.Path] = true
	}

	if !paths["cache.ttl"] {
		t.Error("expected drift at 'cache.ttl'")
	}

	if !paths["database.host"] {
		t.Error("expected drift at 'database.host'")
	}
}
