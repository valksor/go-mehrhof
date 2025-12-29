package linear

import (
	"errors"
	"fmt"
	"net"
	"net/http"
)

// Error types for the Linear provider
var (
	ErrNoToken          = errors.New("linear api key not found")
	ErrIssueNotFound    = errors.New("issue not found")
	ErrRateLimited      = errors.New("linear api rate limit exceeded")
	ErrNetworkError     = errors.New("network error communicating with linear")
	ErrUnauthorized     = errors.New("linear token unauthorized or expired")
	ErrInvalidReference = errors.New("invalid linear reference")
	ErrTeamRequired     = errors.New("linear team key required for this operation")
)

// wrapAPIError converts HTTP errors to typed errors
func wrapAPIError(err error) error {
	if err == nil {
		return nil
	}

	// Check for HTTP errors
	var httpErr interface{ HTTPStatusCode() int }
	if errors.As(err, &httpErr) {
		switch httpErr.HTTPStatusCode() {
		case http.StatusUnauthorized:
			return fmt.Errorf("%w: %v", ErrUnauthorized, err)
		case http.StatusForbidden:
			return fmt.Errorf("%w: %v", ErrRateLimited, err)
		case http.StatusNotFound:
			return fmt.Errorf("%w: %v", ErrIssueNotFound, err)
		case http.StatusTooManyRequests:
			return fmt.Errorf("%w: %v", ErrRateLimited, err)
		}
	}

	// Check for network errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		return fmt.Errorf("%w: %v", ErrNetworkError, err)
	}

	return err
}
