package worker

// Worker event-type strings emitted on the worker pool's event stream.
// Exported so the conductor and socket layers can use a single source of
// truth instead of redefining the wire-format literals per package.
const (
	EventJobCompleted = "job_completed"
	EventJobFailed    = "job_failed"
)
