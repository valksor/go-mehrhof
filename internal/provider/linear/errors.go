package linear

import (
	"errors"
	"fmt"
	"net"
	"net/http"

	providererrors "github.com/valksor/go-mehrhof/internal/provider/errors"
)

// Linear-specific error types that don't have shared equivalents.
var (
	// ErrTeamRequired is returned when a Linear team key is needed but not provided.
	ErrTeamRequired = errors.New("linear team key required for this operation")
)

// wrapAPIError converts HTTP errors to typed errors using shared error package.
func wrapAPIError(err error) error {
	if err == nil {
		return nil
	}

	// Check for HTTP errors
	var httpErr interface{ HTTPStatusCode() int }
	if errors.As(err, &httpErr) {
		switch httpErr.HTTPStatusCode() {
		case http.StatusUnauthorized:
			return fmt.Errorf("%w: %v", providererrors.ErrUnauthorized, err)
		case http.StatusForbidden:
			return fmt.Errorf("%w: %v", providererrors.ErrRateLimited, err)
		case http.StatusNotFound:
			return fmt.Errorf("%w: %v", providererrors.ErrNotFound, err)
		case http.StatusTooManyRequests:
			return fmt.Errorf("%w: %v", providererrors.ErrRateLimited, err)
		}
	}

	// Check for network errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		return fmt.Errorf("%w: %v", providererrors.ErrNetworkError, err)
	}

	return err
}
