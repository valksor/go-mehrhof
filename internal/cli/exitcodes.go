package cli

import "errors"

// Exit code constants for CLI commands.
const (
	ExitSuccess    = 0
	ExitGeneral    = 1
	ExitUsage      = 2
	ExitConnection = 3
	ExitTimeout    = 4
	ExitState      = 5
	ExitNotFound   = 6
)

// ExitCodeFromError returns the appropriate exit code for the given error.
func ExitCodeFromError(err error) int {
	if err == nil {
		return ExitSuccess
	}

	var connErr *ConnectionError
	if errors.As(err, &connErr) {
		return ExitConnection
	}

	var timeoutErr *TimeoutError
	if errors.As(err, &timeoutErr) {
		return ExitTimeout
	}

	var stateErr *StateError
	if errors.As(err, &stateErr) {
		return ExitState
	}

	var notFoundErr *NotFoundError
	if errors.As(err, &notFoundErr) {
		return ExitNotFound
	}

	var usageErr *UsageError
	if errors.As(err, &usageErr) {
		return ExitUsage
	}

	return ExitGeneral
}
