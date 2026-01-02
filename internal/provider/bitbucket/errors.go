package bitbucket

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

// Error types for the Bitbucket provider.
var (
	ErrNoToken           = errors.New("bitbucket token not found")
	ErrNoUsername        = errors.New("bitbucket username not configured")
	ErrRepoNotConfigured = errors.New("repository not configured")
	ErrIssueNotFound     = errors.New("issue not found")
	ErrRateLimited       = errors.New("bitbucket api rate limit exceeded")
	ErrNetworkError      = errors.New("network error communicating with bitbucket")
	ErrUnauthorized      = errors.New("bitbucket credentials unauthorized or expired")
	ErrInvalidReference  = errors.New("invalid bitbucket reference")
	ErrIssuesNotEnabled  = errors.New("issues not enabled for this repository")
)

// wrapAPIError converts Bitbucket API errors to typed errors.
func wrapAPIError(err error) error {
	if err == nil {
		return nil
	}

	errMsg := err.Error()

	// Check for specific error patterns
	if strings.Contains(errMsg, "401") {
		return fmt.Errorf("%w: %v", ErrUnauthorized, err)
	}
	if strings.Contains(errMsg, "403") {
		return fmt.Errorf("%w: %v", ErrUnauthorized, err)
	}
	if strings.Contains(errMsg, "429") {
		return fmt.Errorf("%w: %v", ErrRateLimited, err)
	}
	if strings.Contains(errMsg, "404") {
		return fmt.Errorf("%w: %v", ErrIssueNotFound, err)
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return fmt.Errorf("%w: %v", ErrNetworkError, err)
	}

	return err
}
