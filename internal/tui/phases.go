package tui

import "github.com/valksor/kvelmo/internal/conductor"

// Phase identifier aliases — the canonical definitions live in
// conductor.PhaseXxx. State strings are duplicated here because there is no
// untyped-string source for them (socket.StateXxx is typed as TaskState).
const (
	phasePlan      = conductor.PhasePlan
	phaseImplement = conductor.PhaseImplement
	phaseSimplify  = conductor.PhaseSimplify
	phaseOptimize  = conductor.PhaseOptimize
	phaseReview    = conductor.PhaseReview
	phaseSubmit    = conductor.PhaseSubmit

	stateNone         = "none"
	stateLoaded       = "loaded"
	statePlanning     = "planning"
	statePlanned      = "planned"
	stateImplementing = "implementing"
	stateImplemented  = "implemented"
	stateSimplifying  = "simplifying"
	stateOptimizing   = "optimizing"
	stateFailed       = "failed"
	stateSubmitted    = "submitted"
)
