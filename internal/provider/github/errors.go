package github

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/google/go-github/v67/github"
)

// Error types for the GitHub provider
var (
	ErrNoToken           = errors.New("github token not found")
	ErrRepoNotDetected   = errors.New("could not detect repository from git remote")
	ErrRepoNotConfigured = errors.New("repository not configured")
	ErrIssueNotFound     = errors.New("issue not found")
	ErrRateLimited       = errors.New("github api rate limit exceeded")
	ErrNetworkError      = errors.New("network error communicating with github")
	ErrUnauthorized      = errors.New("github token unauthorized or expired")
	ErrInsufficientScope = errors.New("github token lacks required scope")
	ErrInvalidReference  = errors.New("invalid github reference")
)

// wrapAPIError converts GitHub API errors to typed errors
func wrapAPIError(err error) error {
	if err == nil {
		return nil
	}

	var ghErr *github.ErrorResponse
	if errors.As(err, &ghErr) {
		switch ghErr.Response.StatusCode {
		case 401:
			return fmt.Errorf("%w: %v", ErrUnauthorized, err)
		case 403:
			if strings.Contains(ghErr.Message, "rate limit") {
				resetHeader := ghErr.Response.Header.Get("X-RateLimit-Reset")
				return fmt.Errorf("%w: retry after %s", ErrRateLimited, resetHeader)
			}
			return fmt.Errorf("%w: %v", ErrInsufficientScope, err)
		case 404:
			return fmt.Errorf("%w: %v", ErrIssueNotFound, err)
		}
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return fmt.Errorf("%w: %v", ErrNetworkError, err)
	}

	return err
}
