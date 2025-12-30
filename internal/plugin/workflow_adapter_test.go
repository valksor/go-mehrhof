package plugin

import (
	"context"
	"testing"

	"github.com/valksor/go-mehrhof/internal/workflow"
)

func TestWorkUnitToMap(t *testing.T) {
	tests := []struct {
		wu       *workflow.WorkUnit
		name     string
		wantKeys []string
		wantNil  bool
	}{
		{
			name:    "nil work unit",
			wu:      nil,
			wantNil: true,
		},
		{
			name: "basic work unit",
			wu: &workflow.WorkUnit{
				ID:          "test-123",
				ExternalID:  "ext-456",
				Title:       "Test Task",
				Description: "Test Description",
			},
			wantKeys: []string{"id", "externalId", "title", "description"},
		},
		{
			name: "work unit with source",
			wu: &workflow.WorkUnit{
				ID:         "test-123",
				ExternalID: "ext-456",
				Title:      "Test Task",
				Source: &workflow.Source{
					Reference: "file:task.md",
					Content:   "task content",
				},
			},
			wantKeys: []string{"id", "externalId", "title", "source"},
		},
		{
			name: "work unit with specifications",
			wu: &workflow.WorkUnit{
				ID:             "test-123",
				Specifications: []string{"spec-1", "spec-2"},
			},
			wantKeys: []string{"id", "specifications"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := workUnitToMap(tt.wu)

			if tt.wantNil {
				if got != nil {
					t.Errorf("workUnitToMap() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatal("workUnitToMap() returned nil, want non-nil")
			}

			for _, key := range tt.wantKeys {
				if _, ok := got[key]; !ok {
					t.Errorf("workUnitToMap() missing key %q", key)
				}
			}
		})
	}
}

func TestGetEffectInfo(t *testing.T) {
	manifest := &Manifest{
		Name: "test-plugin",
	}
	proc := &Process{}

	adapter := NewWorkflowAdapter(manifest, proc)

	// Set up effects directly
	adapter.effects = []EffectInfo{
		{Name: "effect1", Description: "First effect"},
		{Name: "effect2", Description: "Second effect", Critical: true},
	}

	tests := []struct {
		name      string
		effect    string
		wantDesc  string
		wantFound bool
	}{
		{
			name:      "existing effect",
			effect:    "effect1",
			wantFound: true,
			wantDesc:  "First effect",
		},
		{
			name:      "existing critical effect",
			effect:    "effect2",
			wantFound: true,
			wantDesc:  "Second effect",
		},
		{
			name:      "non-existing effect",
			effect:    "effect3",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := adapter.GetEffectInfo(tt.effect)
			if found != tt.wantFound {
				t.Errorf("GetEffectInfo() found = %v, want %v", found, tt.wantFound)
			}
			if found && got.Description != tt.wantDesc {
				t.Errorf("GetEffectInfo() description = %q, want %q", got.Description, tt.wantDesc)
			}
		})
	}
}

func TestCreateGuardFunc(t *testing.T) {
	manifest := &Manifest{Name: "test-plugin"}
	proc := &Process{}
	adapter := NewWorkflowAdapter(manifest, proc)

	// Create a mock guard function
	guardFunc := adapter.CreateGuardFunc("test-guard")

	if guardFunc == nil {
		t.Fatal("CreateGuardFunc() returned nil")
	}

	// Verify the function signature is correct
	if guardFunc == nil {
		t.Fatal("guardFunc is nil")
	}

	// The function should have a timeout built in
	// We can't test execution without a real process, but we can verify it's a valid function
	ctx := context.Background()
	wu := &workflow.WorkUnit{}

	// This will panic/return false with a nil process, which is expected behavior
	// The important thing is the function was created
	defer func() {
		if r := recover(); r != nil {
			_ = r // Expected to panic with nil process
		}
	}()
	_ = guardFunc(ctx, wu)
}

func TestCreateEffectFunc(t *testing.T) {
	manifest := &Manifest{Name: "test-plugin"}
	proc := &Process{}
	adapter := NewWorkflowAdapter(manifest, proc)

	effectFunc := adapter.CreateEffectFunc("test-effect", nil)

	if effectFunc == nil {
		t.Fatal("CreateEffectFunc() returned nil")
	}

	// Verify the function signature is correct
	ctx := context.Background()
	wu := &workflow.WorkUnit{}

	// This will error with a nil process, which is expected behavior
	defer func() {
		if r := recover(); r != nil {
			_ = r // Expected to panic with nil process
		}
	}()
	_ = effectFunc(ctx, wu)
}

func TestCreateCriticalEffect(t *testing.T) {
	manifest := &Manifest{Name: "test-plugin"}
	proc := &Process{}
	adapter := NewWorkflowAdapter(manifest, proc)

	info := EffectInfo{
		Name:     "critical-effect",
		Critical: true,
	}

	critical := adapter.CreateCriticalEffect(info, nil)

	if critical.Name != "critical-effect" {
		t.Errorf("Name = %q, want %q", critical.Name, "critical-effect")
	}

	if !critical.Critical {
		t.Error("Critical = false, want true")
	}

	if critical.Fn == nil {
		t.Error("Fn is nil, want non-nil")
	}
}

func TestPhases_Guards_Effects(t *testing.T) {
	manifest := &Manifest{Name: "test-plugin"}
	proc := &Process{}
	adapter := NewWorkflowAdapter(manifest, proc)

	// Initially empty
	if len(adapter.Phases()) != 0 {
		t.Errorf("Phases() = %v, want empty", adapter.Phases())
	}
	if len(adapter.Guards()) != 0 {
		t.Errorf("Guards() = %v, want empty", adapter.Guards())
	}
	if len(adapter.Effects()) != 0 {
		t.Errorf("Effects() = %v, want empty", adapter.Effects())
	}

	// Set up phases, guards, effects directly
	adapter.phases = []PhaseInfo{
		{Name: "phase1", Description: "First phase"},
	}
	adapter.guards = []GuardInfo{
		{Name: "guard1", Description: "First guard"},
	}
	adapter.effects = []EffectInfo{
		{Name: "effect1", Description: "First effect"},
	}

	// Now they should return the set values
	if len(adapter.Phases()) != 1 {
		t.Errorf("Phases() length = %d, want 1", len(adapter.Phases()))
	}
	if len(adapter.Guards()) != 1 {
		t.Errorf("Guards() length = %d, want 1", len(adapter.Guards()))
	}
	if len(adapter.Effects()) != 1 {
		t.Errorf("Effects() length = %d, want 1", len(adapter.Effects()))
	}
}

func TestManifest(t *testing.T) {
	manifest := &Manifest{
		Name:        "test-plugin",
		Description: "Test plugin",
	}
	proc := &Process{}
	adapter := NewWorkflowAdapter(manifest, proc)

	if adapter.Manifest() != manifest {
		t.Error("Manifest() returned different manifest")
	}

	if adapter.Manifest().Name != "test-plugin" {
		t.Errorf("Manifest().Name = %q, want %q", adapter.Manifest().Name, "test-plugin")
	}
}

func TestNewWorkflowAdapter(t *testing.T) {
	manifest := &Manifest{Name: "test"}
	proc := &Process{}

	adapter := NewWorkflowAdapter(manifest, proc)

	if adapter == nil {
		t.Fatal("NewWorkflowAdapter() returned nil")
	}

	if adapter.manifest != manifest {
		t.Error("manifest not set correctly")
	}

	if adapter.proc != proc {
		t.Error("proc not set correctly")
	}
}

func TestBuildPhaseDefinitions(t *testing.T) {
	manifest := &Manifest{Name: "test"}
	proc := &Process{}
	adapter := NewWorkflowAdapter(manifest, proc)

	// Set up phases
	adapter.phases = []PhaseInfo{
		{Name: "phase1", Description: "Phase One", After: "planning"},
	}

	// Set up effects
	adapter.effects = []EffectInfo{
		{Name: "phase1_effect1", Description: "Effect for phase1", Critical: true},
	}

	defs := adapter.BuildPhaseDefinitions()

	if len(defs) != 1 {
		t.Fatalf("BuildPhaseDefinitions() length = %d, want 1", len(defs))
	}

	d1 := defs[0]
	if d1.Description != "Phase One" {
		t.Errorf("Phase 1 Description = %q, want %q", d1.Description, "Phase One")
	}
	if d1.After != "planning" {
		t.Errorf("Phase 1 After = %q, want %q", d1.After, "planning")
	}
	if len(d1.Effects) != 1 {
		t.Errorf("Phase 1 Effects length = %d, want 1", len(d1.Effects))
	}

	// Check critical effect
	effect := d1.Effects[0]
	if !effect.Critical {
		t.Error("Effect Critical = false, want true")
	}
}
