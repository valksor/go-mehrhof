package workflow

import "context"

// EffectFunc is a side effect executed during a transition
// Effects are executed by the conductor, not the state machine directly
type EffectFunc func(ctx context.Context, wu *WorkUnit) error

// EffectType identifies effect categories for conductor to handle
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

// EffectRequest represents a request for the conductor to execute an effect
type EffectRequest struct {
	Type EffectType
	Data map[string]any
}

// EffectRegistry allows registering effect handlers
type EffectRegistry struct {
	handlers map[EffectType]EffectFunc
}

// NewEffectRegistry creates a new effect registry
func NewEffectRegistry() *EffectRegistry {
	return &EffectRegistry{
		handlers: make(map[EffectType]EffectFunc),
	}
}

// Register adds an effect handler
func (r *EffectRegistry) Register(effectType EffectType, handler EffectFunc) {
	r.handlers[effectType] = handler
}

// Execute runs an effect handler
func (r *EffectRegistry) Execute(ctx context.Context, effectType EffectType, wu *WorkUnit) error {
	handler, ok := r.handlers[effectType]
	if !ok {
		return nil // No handler registered, skip
	}
	return handler(ctx, wu)
}

// Has checks if an effect handler is registered
func (r *EffectRegistry) Has(effectType EffectType) bool {
	_, ok := r.handlers[effectType]
	return ok
}
