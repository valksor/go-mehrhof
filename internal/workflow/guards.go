package workflow

import "context"

// GuardFunc is a predicate that must return true for a transition to occur
type GuardFunc func(ctx context.Context, wu *WorkUnit) bool

// GuardHasSource checks if a source reference is provided
func GuardHasSource(ctx context.Context, wu *WorkUnit) bool {
	return wu != nil && wu.Source != nil && wu.Source.Reference != ""
}

// GuardHasSpecifications checks if specifications exist for implementation
func GuardHasSpecifications(ctx context.Context, wu *WorkUnit) bool {
	return wu != nil && len(wu.Specifications) > 0
}

// GuardNoSpecifications checks if no specifications exist yet
func GuardNoSpecifications(ctx context.Context, wu *WorkUnit) bool {
	return wu != nil && len(wu.Specifications) == 0
}

// GuardCanUndo checks if there are checkpoints to undo
func GuardCanUndo(ctx context.Context, wu *WorkUnit) bool {
	return wu != nil && len(wu.Checkpoints) > 0
}

// GuardCanRedo checks if there are undone changes to restore
// Note: The conductor validates git-level redo capability before dispatching.
// This guard performs a basic check; actual redo validation happens in conductor.
func GuardCanRedo(ctx context.Context, wu *WorkUnit) bool {
	// Allow if work unit exists - conductor handles detailed validation
	return wu != nil
}

// GuardCanFinish checks if the work is ready to be completed
// Requires at least one specification to have been created
func GuardCanFinish(ctx context.Context, wu *WorkUnit) bool {
	return wu != nil && len(wu.Specifications) > 0
}

// GuardCanReview checks if there are specifications to review
func GuardCanReview(ctx context.Context, wu *WorkUnit) bool {
	return wu != nil && len(wu.Specifications) > 0
}

// EvaluateGuards checks if all guards pass for a transition
func EvaluateGuards(ctx context.Context, wu *WorkUnit, guards []GuardFunc) bool {
	for _, guard := range guards {
		if !guard(ctx, wu) {
			return false
		}
	}
	return true
}
