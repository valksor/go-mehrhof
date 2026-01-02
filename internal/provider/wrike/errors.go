package wrike

import (
	"errors"
	"net/http"

	providererrors "github.com/valksor/go-mehrhof/internal/provider/errors"
)

// Wrike-specific error types.
var (
	ErrTaskNotFound     = errors.New("task not found")
	ErrInvalidReference = errors.New("invalid wrike reference")
)

// wrapAPIError converts HTTP errors to typed errors.
// Uses shared error types from provider/errors package for common cases.
func wrapAPIError(err error) error {
	if err == nil {
		return nil
	}

	// Use shared HTTP error wrapping
	// For 404, we want to return our specific ErrTaskNotFound
	baseErrors := map[int]error{
		http.StatusNotFound: ErrTaskNotFound,
	}

	return providererrors.WrapHTTPError(err, "wrike", baseErrors)
}
