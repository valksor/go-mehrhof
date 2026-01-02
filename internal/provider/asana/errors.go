package asana

import (
	"errors"
	"fmt"

	providererrors "github.com/valksor/go-mehrhof/internal/provider/errors"
)

// Sentinel errors for the Asana provider.
var (
	ErrNoToken           = errors.New("asana token not found")
	ErrWorkspaceRequired = errors.New("workspace GID is required")
	ErrProjectRequired   = errors.New("project GID is required for list operations")
	ErrInvalidReference  = errors.New("invalid asana reference")
	ErrTaskNotFound      = errors.New("task not found")
	ErrRateLimited       = errors.New("rate limited")
)

// wrapAPIError wraps API errors with appropriate provider error types.
func wrapAPIError(err error) error {
	if err == nil {
		return nil
	}

	errStr := err.Error()

	// Check for common HTTP status codes
	if contains(errStr, "401") || contains(errStr, "Unauthorized") {
		return fmt.Errorf("%w: %w", providererrors.ErrUnauthorized, err)
	}
	if contains(errStr, "403") || contains(errStr, "Forbidden") {
		return fmt.Errorf("%w: %w", providererrors.ErrUnauthorized, err)
	}
	if contains(errStr, "404") || contains(errStr, "Not Found") {
		return fmt.Errorf("%w: %w", ErrTaskNotFound, err)
	}
	if contains(errStr, "429") || contains(errStr, "rate limit") {
		return fmt.Errorf("%w: %w", ErrRateLimited, err)
	}

	return fmt.Errorf("asana API error: %w", err)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}
