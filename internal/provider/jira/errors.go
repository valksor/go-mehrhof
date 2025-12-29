package jira

import (
	"errors"
)

// Error types for the Jira provider
var (
	ErrNoToken          = errors.New("jira api token not found")
	ErrIssueNotFound    = errors.New("issue not found")
	ErrRateLimited      = errors.New("jira api rate limit exceeded")
	ErrNetworkError     = errors.New("network error communicating with jira")
	ErrUnauthorized     = errors.New("jira token unauthorized or expired")
	ErrInvalidReference = errors.New("invalid jira reference")
	ErrProjectRequired  = errors.New("jira project key required for this operation")
	ErrNoTransition     = errors.New("no valid transition found for status change")
)
