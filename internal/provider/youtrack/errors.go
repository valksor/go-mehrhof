package youtrack

import (
	"errors"
	"fmt"
	"net"
	"net/http"

	providererrors "github.com/valksor/go-mehrhof/internal/provider/errors"
)

// httpError represents an HTTP error with status code.
// This is provider-specific to match YouTrack's error format.
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

// HTTPStatusCode returns the HTTP status code.
func (e *httpError) HTTPStatusCode() int {
	return e.code
}

// newHTTPError creates a new HTTP error.
func newHTTPError(code int, message string) *httpError {
	return &httpError{code: code, message: message}
}

// wrapAPIError wraps an error with appropriate typed errors using shared error package.
func wrapAPIError(err error) error {
	if err == nil {
		return nil
	}

	// Check if it's already a provider error
	if errors.Is(err, providererrors.ErrNoToken) ||
		errors.Is(err, providererrors.ErrNotFound) ||
		errors.Is(err, providererrors.ErrRateLimited) ||
		errors.Is(err, providererrors.ErrNetworkError) ||
		errors.Is(err, providererrors.ErrUnauthorized) ||
		errors.Is(err, providererrors.ErrInvalidReference) {
		return err
	}

	// Check for HTTP errors
	var httpErr *httpError
	if errors.As(err, &httpErr) {
		switch httpErr.code {
		case http.StatusUnauthorized:
			return fmt.Errorf("%w: %v", providererrors.ErrUnauthorized, err)
		case http.StatusForbidden:
			return fmt.Errorf("%w: %v", providererrors.ErrRateLimited, err)
		case http.StatusNotFound:
			return fmt.Errorf("%w: %v", providererrors.ErrNotFound, err)
		case http.StatusTooManyRequests:
			return fmt.Errorf("%w: %v", providererrors.ErrRateLimited, err)
		case http.StatusServiceUnavailable:
			return fmt.Errorf("%w: %v", providererrors.ErrRateLimited, err)
		default:
			return err
		}
	}

	// Check for network errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		return fmt.Errorf("%w: %v", providererrors.ErrNetworkError, err)
	}

	return err
}
