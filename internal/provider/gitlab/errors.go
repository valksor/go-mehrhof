package gitlab

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

// Error types for the GitLab provider.
var (
	ErrNoToken              = errors.New("gitlab token not found")
	ErrProjectNotDetected   = errors.New("could not detect project from git remote")
	ErrProjectNotConfigured = errors.New("project not configured")
	ErrIssueNotFound        = errors.New("issue not found")
	ErrRateLimited          = errors.New("gitlab api rate limit exceeded")
	ErrNetworkError         = errors.New("network error communicating with gitlab")
	ErrUnauthorized         = errors.New("gitlab token unauthorized or expired")
	ErrInsufficientScope    = errors.New("gitlab token lacks required scope")
	ErrInvalidReference     = errors.New("invalid gitlab reference")
)

// wrapAPIError converts GitLab API errors to typed errors.
func wrapAPIError(err error) error {
	if err == nil {
		return nil
	}

	// Check for GitLab error response
	errMsg := err.Error()

	// Check for specific error patterns
	if strings.Contains(errMsg, "401") || strings.Contains(errMsg, "403 Unauthorized") {
		return fmt.Errorf("%w: %w", ErrUnauthorized, err)
	}
	if strings.Contains(errMsg, "403") && strings.Contains(errMsg, "rate limit") {
		return fmt.Errorf("%w: %w", ErrRateLimited, err)
	}
	if strings.Contains(errMsg, "404") {
		return fmt.Errorf("%w: %w", ErrIssueNotFound, err)
	}
	if strings.Contains(errMsg, "403") {
		return fmt.Errorf("%w: %w", ErrInsufficientScope, err)
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return fmt.Errorf("%w: %w", ErrNetworkError, err)
	}

	return err
}
