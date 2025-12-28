package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/valksor/go-mehrhof/internal/workflow"
)

// WorkflowAdapter wraps a plugin process to provide workflow extensions.
type WorkflowAdapter struct {
	manifest *Manifest
	proc     *Process
	phases   []PhaseInfo
	guards   []GuardInfo
	effects  []EffectInfo
}

// NewWorkflowAdapter creates a new workflow adapter for a plugin.
func NewWorkflowAdapter(manifest *Manifest, proc *Process) *WorkflowAdapter {
	return &WorkflowAdapter{
		manifest: manifest,
		proc:     proc,
	}
}

// Initialize initializes the workflow plugin and discovers its extensions.
func (a *WorkflowAdapter) Initialize(ctx context.Context, config map[string]any) error {
	result, err := a.proc.Call(ctx, "workflow.init", &InitParams{Config: config})
	if err != nil {
		return fmt.Errorf("initialize workflow plugin: %w", err)
	}

	var resp WorkflowInitResult
	if err := json.Unmarshal(result, &resp); err != nil {
		return fmt.Errorf("parse workflow init response: %w", err)
	}

	a.phases = resp.Phases
	a.guards = resp.Guards
	a.effects = resp.Effects

	return nil
}

// Phases returns the custom phases defined by this plugin.
func (a *WorkflowAdapter) Phases() []PhaseInfo {
	return a.phases
}

// Guards returns the custom guards defined by this plugin.
func (a *WorkflowAdapter) Guards() []GuardInfo {
	return a.guards
}

// Effects returns the custom effects defined by this plugin.
func (a *WorkflowAdapter) Effects() []EffectInfo {
	return a.effects
}

// EvaluateGuard evaluates a custom guard via the plugin.
func (a *WorkflowAdapter) EvaluateGuard(ctx context.Context, name string, wu *workflow.WorkUnit) (bool, string, error) {
	// Convert WorkUnit to a map for JSON serialization
	wuMap := workUnitToMap(wu)

	result, err := a.proc.Call(ctx, "workflow.evaluateGuard", &EvaluateGuardParams{
		Name:     name,
		WorkUnit: wuMap,
	})
	if err != nil {
		return false, "", fmt.Errorf("evaluate guard %s: %w", name, err)
	}

	var resp EvaluateGuardResult
	if err := json.Unmarshal(result, &resp); err != nil {
		return false, "", fmt.Errorf("parse guard response: %w", err)
	}

	return resp.Passed, resp.Reason, nil
}

// ExecuteEffect executes a custom effect via the plugin.
func (a *WorkflowAdapter) ExecuteEffect(ctx context.Context, name string, wu *workflow.WorkUnit, data map[string]any) error {
	wuMap := workUnitToMap(wu)

	result, err := a.proc.Call(ctx, "workflow.executeEffect", &ExecuteEffectParams{
		Name:     name,
		WorkUnit: wuMap,
		Data:     data,
	})
	if err != nil {
		return fmt.Errorf("execute effect %s: %w", name, err)
	}

	var resp ExecuteEffectResult
	if err := json.Unmarshal(result, &resp); err != nil {
		return fmt.Errorf("parse effect response: %w", err)
	}

	if !resp.Success {
		if resp.Error != "" {
			return fmt.Errorf("effect error: %s", resp.Error)
		}
		return fmt.Errorf("effect %s failed", name)
	}

	return nil
}

// CreateGuardFunc creates a workflow.GuardFunc that calls this plugin's guard.
func (a *WorkflowAdapter) CreateGuardFunc(name string) workflow.GuardFunc {
	return func(ctx context.Context, wu *workflow.WorkUnit) bool {
		// Use a timeout to prevent hanging
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		passed, _, err := a.EvaluateGuard(ctx, name, wu)
		if err != nil {
			// Guard errors are treated as failure
			return false
		}
		return passed
	}
}

// CreateEffectFunc creates a workflow.EffectFunc that calls this plugin's effect.
func (a *WorkflowAdapter) CreateEffectFunc(name string, data map[string]any) workflow.EffectFunc {
	return func(ctx context.Context, wu *workflow.WorkUnit) error {
		// Use a timeout to prevent hanging
		ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()

		return a.ExecuteEffect(ctx, name, wu, data)
	}
}

// Manifest returns the plugin manifest.
func (a *WorkflowAdapter) Manifest() *Manifest {
	return a.manifest
}

// ─────────────────────────────────────────────────────────────────────────────
// Phase Registration Helper
// ─────────────────────────────────────────────────────────────────────────────

// PluginPhase represents a custom phase to be registered with the workflow.
type PluginPhase struct {
	Name        string
	Description string
	State       workflow.State
	After       workflow.State // Insert after this state
	Before      workflow.State // Or insert before this state
	EntryEvent  workflow.Event // Event to enter this phase
	ExitEvent   workflow.Event // Event to exit this phase
	Guards      []workflow.GuardFunc
	Effects     []workflow.EffectFunc
}

// BuildPhases converts plugin phase definitions to PluginPhase structs.
func (a *WorkflowAdapter) BuildPhases() []PluginPhase {
	var phases []PluginPhase

	for _, p := range a.phases {
		// Create a unique state and events for this plugin phase
		stateName := workflow.State("plugin_" + a.manifest.Name + "_" + p.Name)
		entryEvent := workflow.Event("plugin_" + a.manifest.Name + "_" + p.Name + "_start")
		exitEvent := workflow.Event("plugin_" + a.manifest.Name + "_" + p.Name + "_done")

		phase := PluginPhase{
			Name:        p.Name,
			Description: p.Description,
			State:       stateName,
			EntryEvent:  entryEvent,
			ExitEvent:   exitEvent,
		}

		// Determine insertion point
		if p.After != "" {
			phase.After = workflow.State(p.After)
		}
		if p.Before != "" {
			phase.Before = workflow.State(p.Before)
		}

		// Build guards for this phase
		for _, g := range a.guards {
			// Check if this guard is associated with this phase
			// (convention: guard name starts with phase name)
			if len(g.Name) > len(p.Name) && g.Name[:len(p.Name)] == p.Name {
				phase.Guards = append(phase.Guards, a.CreateGuardFunc(g.Name))
			}
		}

		// Build effects for this phase
		for _, e := range a.effects {
			// Check if this effect is associated with this phase
			if len(e.Name) > len(p.Name) && e.Name[:len(p.Name)] == p.Name {
				phase.Effects = append(phase.Effects, a.CreateEffectFunc(e.Name, nil))
			}
		}

		phases = append(phases, phase)
	}

	return phases
}

// ─────────────────────────────────────────────────────────────────────────────
// Conversion helpers
// ─────────────────────────────────────────────────────────────────────────────

func workUnitToMap(wu *workflow.WorkUnit) map[string]any {
	if wu == nil {
		return nil
	}

	result := map[string]any{
		"id":             wu.ID,
		"externalId":     wu.ExternalID,
		"title":          wu.Title,
		"description":    wu.Description,
		"specifications": wu.Specifications,
		"checkpoints":    wu.Checkpoints,
	}

	if wu.Source != nil {
		result["source"] = map[string]any{
			"reference": wu.Source.Reference,
			"content":   wu.Source.Content,
		}
	}

	return result
}
