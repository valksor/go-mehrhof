package commands

import "github.com/valksor/kvelmo/internal/conductor"

// Subcommand names and parameter keys reused across many CLI commands.
// statusOK / statusError / statusWarning are already defined in
// config_validate.go and reused from there.
const (
	subList   = "list"
	subStatus = "status"
	subStats  = "stats"
	subFiles  = "files"

	// Phase identifiers — aliases of conductor.PhaseXxx so the canonical
	// definition lives in one place. Adding a new phase requires updating
	// internal/conductor/conductor_mcp.go first.
	phasePlan      = conductor.PhasePlan
	phaseImplement = conductor.PhaseImplement
	phaseSimplify  = conductor.PhaseSimplify
	phaseOptimize  = conductor.PhaseOptimize
	phaseReview    = conductor.PhaseReview
	phaseSubmit    = conductor.PhaseSubmit

	paramSource     = "source"
	paramAction     = "action"
	paramWorktreeID = "worktree_id"
	paramDryRun     = "dry_run"
	paramSelector   = "selector"
	paramName       = "name"

	stateNone      = "none"
	stateFailed    = "failed"
	stateLoaded    = "loaded"
	stateSubmitted = "submitted"
)
