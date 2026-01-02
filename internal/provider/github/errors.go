package github

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/go-github/v67/github"
	providererrors "github.com/valksor/go-mehrhof/internal/provider/errors"
)

// GitHub-specific error types.
var (
	ErrRepoNotDetected   = errors.New("could not detect repository from git remote")
	ErrRepoNotConfigured = errors.New("repository not configured")
	ErrIssueNotFound     = errors.New("issue not found")
	ErrInsufficientScope = errors.New("github token lacks required scope")
	ErrInvalidReference  = errors.New("invalid github reference")
)

// wrapAPIError converts GitHub API errors to typed errors.
// Uses shared error types from provider/errors package for common cases.
func wrapAPIError(err error) error {
	if err == nil {
		return nil
	}

	var ghErr *github.ErrorResponse
	if errors.As(err, &ghErr) {
		switch ghErr.Response.StatusCode {
		case 401:
			return providererrors.UnauthorizedError("github", err)
		case 403:
			if strings.Contains(ghErr.Message, "rate limit") {
				resetHeader := ghErr.Response.Header.Get("X-RateLimit-Reset")
				return providererrors.RateLimitedError("github", "retry after "+resetHeader)
			}
			return fmt.Errorf("%w: %w", ErrInsufficientScope, err)
		case 404:
			return fmt.Errorf("%w: %w", ErrIssueNotFound, err)
		}
	}

	// Use shared HTTP error wrapping for network and other common errors
	return providererrors.WrapHTTPError(err, "github", nil)
}
