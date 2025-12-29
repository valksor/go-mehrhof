package notion

import (
	"errors"
	"fmt"
	"net"
	"net/http"
)

// Error types for the Notion provider
var (
	ErrNoToken          = errors.New("notion api key not found")
	ErrPageNotFound     = errors.New("page not found")
	ErrUnauthorized     = errors.New("notion token unauthorized or expired")
	ErrInvalidReference = errors.New("invalid notion reference")
	ErrDatabaseRequired = errors.New("database id required for this operation")
	ErrRateLimited      = errors.New("notion api rate limit exceeded")
	ErrNetworkError     = errors.New("network error communicating with notion")
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
			return fmt.Errorf("%w: %v", ErrPageNotFound, err)
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
