package jira

import (
	"errors"
)

// Jira-specific error types that don't have shared equivalents.
var (
	// ErrProjectRequired is returned when a Jira project key is needed but not provided.
	ErrProjectRequired = errors.New("jira project key required for this operation")

	// ErrNoTransition is returned when no valid Jira transition exists for a status change.
	ErrNoTransition = errors.New("no valid transition found for status change")
)
