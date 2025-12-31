package clickup

import (
	"errors"
	"fmt"

	providererrors "github.com/valksor/go-mehrhof/internal/provider/errors"
)

// Sentinel errors for the ClickUp provider
var (
	ErrNoToken          = errors.New("clickup token not found")
	ErrListRequired     = errors.New("list ID is required for list operations")
	ErrInvalidReference = errors.New("invalid clickup reference")
	ErrTaskNotFound     = errors.New("task not found")
	ErrRateLimited      = errors.New("rate limited")
)

// wrapAPIError wraps API errors with appropriate provider error types
func wrapAPIError(err error) error {
	if err == nil {
		return nil
	}

	errStr := err.Error()

	// Check for common HTTP status codes
	if contains(errStr, "401") || contains(errStr, "Unauthorized") {
		return fmt.Errorf("%w: %v", providererrors.ErrUnauthorized, err)
	}
	if contains(errStr, "403") || contains(errStr, "Forbidden") {
		return fmt.Errorf("%w: %v", providererrors.ErrUnauthorized, err)
	}
	if contains(errStr, "404") || contains(errStr, "Not Found") {
		return fmt.Errorf("%w: %v", ErrTaskNotFound, err)
	}
	if contains(errStr, "429") || contains(errStr, "rate limit") {
		return fmt.Errorf("%w: %v", ErrRateLimited, err)
	}

	return fmt.Errorf("clickup API error: %w", err)
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
