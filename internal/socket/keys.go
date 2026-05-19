package socket

// Shared JSON map keys used by RPC handlers. Centralizing these keeps the
// goconst linter happy (each literal appears once here) and lets call sites
// rely on identifier-level consistency.
const (
	keyStatus   = "status"
	keyState    = "state"
	keyMessage  = "message"
	keyPath     = "path"
	keyEntries  = "entries"
	keyCount    = "count"
	keyEnabled  = "enabled"
	keyFindings = "findings"
	keyTotal    = "total"
	keyJobID    = "job_id"

	statusOffline = "offline"

	// Batch action names accepted by the global tasks.batch handler.
	// pause/resume exist as state-machine events (conductor.EventPause,
	// conductor.StatePaused) but have no batchable RPC handler, so they
	// are deliberately absent from this list.
	actionPlan      = "plan"
	actionImplement = "implement"
	actionReview    = "review"
	actionSubmit    = "submit"
	actionAbort     = "abort"
	actionReset     = "reset"
	actionStop      = "stop"
)
