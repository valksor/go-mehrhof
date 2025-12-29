package youtrack

import (
	"errors"
	"fmt"
	"net"
	"net/http"
)

var (
	// ErrNoToken is returned when no YouTrack token can be found
	ErrNoToken = errors.New("youtrack token not found")
	// ErrIssueNotFound is returned when an issue cannot be found
	ErrIssueNotFound = errors.New("issue not found")
	// ErrRateLimited is returned when API rate limit is exceeded
	ErrRateLimited = errors.New("youtrack api rate limit exceeded")
	// ErrNetworkError is returned for network communication errors
	ErrNetworkError = errors.New("network error communicating with youtrack")
	// ErrUnauthorized is returned when the token is invalid or expired
	ErrUnauthorized = errors.New("youtrack token unauthorized or expired")
	// ErrInvalidReference is returned when a reference format is invalid
	ErrInvalidReference = errors.New("invalid youtrack reference")
)

// httpError represents an HTTP error with status code
type httpError struct {
	message string
	code    int
}

func (e *httpError) Error() string {
	if e.message != "" {
		return fmt.Sprintf("HTTP %d: %s", e.code, e.message)
	}
	return fmt.Sprintf("HTTP %d", e.code)
}

// HTTPStatusCode returns the HTTP status code
func (e *httpError) HTTPStatusCode() int {
	return e.code
}

// wrapAPIError wraps an error with appropriate YouTrack-specific error types
func wrapAPIError(err error) error {
	if err == nil {
		return nil
	}

	// Check if it's already one of our errors
	if errors.Is(err, ErrNoToken) ||
		errors.Is(err, ErrIssueNotFound) ||
		errors.Is(err, ErrRateLimited) ||
		errors.Is(err, ErrNetworkError) ||
		errors.Is(err, ErrUnauthorized) ||
		errors.Is(err, ErrInvalidReference) {
		return err
	}

	// Check for HTTP errors
	var httpErr *httpError
	if errors.As(err, &httpErr) {
		switch httpErr.code {
		case http.StatusUnauthorized:
			return fmt.Errorf("%w: %v", ErrUnauthorized, err)
		case http.StatusForbidden:
			return fmt.Errorf("%w: %v", ErrRateLimited, err)
		case http.StatusNotFound:
			return fmt.Errorf("%w: %v", ErrIssueNotFound, err)
		case http.StatusTooManyRequests:
			return fmt.Errorf("%w: %v", ErrRateLimited, err)
		case http.StatusServiceUnavailable:
			return fmt.Errorf("%w: %v", ErrRateLimited, err)
		default:
			return err
		}
	}

	// Check for network errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		return fmt.Errorf("%w: %v", ErrNetworkError, err)
	}

	return err
}

// newHTTPError creates a new HTTP error
func newHTTPError(code int, message string) *httpError {
	return &httpError{code: code, message: message}
}
