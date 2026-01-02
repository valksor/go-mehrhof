package workflow

import (
	"context"
	"fmt"
	"log/slog"
)

// EffectFunc is a side effect executed during a transition
// Effects are executed by the conductor, not the state machine directly.
type EffectFunc func(ctx context.Context, wu *WorkUnit) error

// EffectType identifies effect categories for conductor to handle.
type EffectType string

const (
	EffectInitWorkUnit        EffectType = "init_work_unit"
	EffectParseSource         EffectType = "parse_source"
	EffectSaveBlueprints      EffectType = "save_blueprints"
	EffectStageChanges        EffectType = "stage_changes"
	EffectRecordCheckpoint    EffectType = "record_checkpoint"
	EffectRestoreCheckpoint   EffectType = "restore_checkpoint"
	EffectSaveState           EffectType = "save_state"
	EffectLoadState           EffectType = "load_state"
	EffectLogError            EffectType = "log_error"
	EffectLogValidationErrors EffectType = "log_validation_errors"
	EffectRollbackChanges     EffectType = "rollback_changes"
	EffectMergeAndCleanup     EffectType = "merge_and_cleanup"
	EffectCleanup             EffectType = "cleanup"
)

// EffectRequest represents a request for the conductor to execute an effect.
type EffectRequest struct {
	Data map[string]any
	Type EffectType
}

// EffectRegistry allows registering effect handlers.
type EffectRegistry struct {
	handlers map[EffectType]EffectFunc
}

// NewEffectRegistry creates a new effect registry.
func NewEffectRegistry() *EffectRegistry {
	return &EffectRegistry{
		handlers: make(map[EffectType]EffectFunc),
	}
}

// Register adds an effect handler.
func (r *EffectRegistry) Register(effectType EffectType, handler EffectFunc) {
	r.handlers[effectType] = handler
}

// Execute runs an effect handler.
func (r *EffectRegistry) Execute(ctx context.Context, effectType EffectType, wu *WorkUnit) error {
	handler, ok := r.handlers[effectType]
	if !ok {
		return nil // No handler registered, skip
	}
	return handler(ctx, wu)
}

// Has checks if an effect handler is registered.
func (r *EffectRegistry) Has(effectType EffectType) bool {
	_, ok := r.handlers[effectType]
	return ok
}

// ─────────────────────────────────────────────────────────────────────────────
// Critical Effects for Plugin Support
// ─────────────────────────────────────────────────────────────────────────────

// CriticalEffect wraps an EffectFunc with criticality metadata.
// Critical effects must succeed for the workflow transition to complete.
// Non-critical effects log errors but allow the workflow to continue.
type CriticalEffect struct {
	Name     string     // Effect name for logging and debugging
	Fn       EffectFunc // The actual effect function
	Critical bool       // If true, failure blocks the transition
}

// ExecuteEffects runs all effects in order.
// Returns an error only if a critical effect fails.
// Non-critical effect failures are logged but don't block the workflow.
func ExecuteEffects(ctx context.Context, wu *WorkUnit, effects []CriticalEffect) error {
	for _, eff := range effects {
		if eff.Fn == nil {
			continue
		}

		err := eff.Fn(ctx, wu)
		if err != nil {
			if eff.Critical {
				return fmt.Errorf("critical effect %s failed: %w", eff.Name, err)
			}
			// Log non-critical effect failure but continue
			slog.Debug("non-critical effect failed", "effect", eff.Name, "error", err)
		}
	}
	return nil
}

// WrapEffect wraps a simple EffectFunc as a non-critical CriticalEffect.
func WrapEffect(name string, fn EffectFunc) CriticalEffect {
	return CriticalEffect{
		Name:     name,
		Fn:       fn,
		Critical: false,
	}
}

// WrapCriticalEffect wraps an EffectFunc as a critical CriticalEffect.
func WrapCriticalEffect(name string, fn EffectFunc) CriticalEffect {
	return CriticalEffect{
		Name:     name,
		Fn:       fn,
		Critical: true,
	}
}
