package wrike

import (
	"errors"
	"fmt"
	"net"
	"net/http"
)

// Error types for the Wrike provider
var (
	ErrNoToken          = errors.New("wrike token not found")
	ErrTaskNotFound     = errors.New("task not found")
	ErrRateLimited      = errors.New("wrike api rate limit exceeded")
	ErrNetworkError     = errors.New("network error communicating with wrike")
	ErrUnauthorized     = errors.New("wrike token unauthorized or expired")
	ErrInvalidReference = errors.New("invalid wrike reference")
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
			return fmt.Errorf("%w: %v", ErrTaskNotFound, err)
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
