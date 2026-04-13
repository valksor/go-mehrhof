package notify

import (
	"slices"
	"testing"
)

func TestAllTypes_ReturnsBuiltInTypes(t *testing.T) {
	types := AllTypes()
	if len(types) != 6 {
		t.Fatalf("expected 6 built-in types, got %d", len(types))
	}

	names := make([]string, len(types))
	for i, nt := range types {
		names[i] = nt.Name
	}

	expected := []string{
		"task.phase_complete",
		"task.review_needed",
		"task.failed",
		"task.submitted",
		"agent.error",
		"quality.gate_failed",
	}

	for _, name := range expected {
		if !slices.Contains(names, name) {
			t.Errorf("expected type %q in AllTypes result", name)
		}
	}
}

func TestAllTypes_ReturnsCopy(t *testing.T) {
	types1 := AllTypes()
	types1[0].Name = "mutated"

	types2 := AllTypes()
	if types2[0].Name == "mutated" {
		t.Error("AllTypes returned a reference to internal slice, expected a copy")
	}
}

func TestLookupType_Found(t *testing.T) {
	nt, ok := LookupType("task.failed")
	if !ok {
		t.Fatal("expected to find task.failed")
	}
	if nt.Name != "task.failed" {
		t.Errorf("expected name=task.failed, got %s", nt.Name)
	}
	if !nt.Default {
		t.Error("expected task.failed to be enabled by default")
	}
}

func TestLookupType_NotFound(t *testing.T) {
	_, ok := LookupType("nonexistent.type")
	if ok {
		t.Error("expected LookupType to return false for unknown type")
	}
}

func TestLookupType_GateFailedNotDefaultEnabled(t *testing.T) {
	nt, ok := LookupType("quality.gate_failed")
	if !ok {
		t.Fatal("expected to find quality.gate_failed")
	}
	if nt.Default {
		t.Error("expected quality.gate_failed to not be enabled by default")
	}
}
