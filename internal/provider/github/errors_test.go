package github

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"

	"github.com/google/go-github/v67/github"
	providererrors "github.com/valksor/go-mehrhof/internal/provider/errors"
)

func TestWrapAPIError_NetworkError(t *testing.T) {
	// Test network error wrapping (uses shared errors)
	netErr := &net.OpError{Op: "dial", Net: "tcp", Err: fmt.Errorf("connection refused")}
	got := wrapAPIError(netErr)

	if got == nil {
		t.Fatal("wrapAPIError() = nil, want error")
	}

	// Should use shared ErrNetworkError
	if !errors.Is(got, providererrors.ErrNetworkError) {
		t.Errorf("wrapAPIError() error = %v, want wrapped %v", got, providererrors.ErrNetworkError)
	}
}

func TestWrapAPIError_500ServerError(t *testing.T) {
	// Test that 500 errors pass through unchanged
	serverErr := &github.ErrorResponse{
		Response: &http.Response{StatusCode: 500},
		Message:  "Internal Server Error",
	}
	got := wrapAPIError(serverErr)

	if got == nil {
		t.Fatal("wrapAPIError() = nil, want error")
	}

	// Should pass through as-is, not wrapped
	var ghErr *github.ErrorResponse
	if !errors.As(got, &ghErr) {
		t.Errorf("wrapAPIError() should return original github.ErrorResponse for 500")
	}
}

func TestErrorVariables(t *testing.T) {
	// Test that GitHub-specific error variables are properly defined
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"ErrRepoNotDetected", ErrRepoNotDetected, "could not detect repository from git remote"},
		{"ErrRepoNotConfigured", ErrRepoNotConfigured, "repository not configured"},
		{"ErrIssueNotFound", ErrIssueNotFound, "issue not found"},
		{"ErrInsufficientScope", ErrInsufficientScope, "github token lacks required scope"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.want {
				t.Errorf("%s.Error() = %q, want %q", tt.name, tt.err.Error(), tt.want)
			}
		})
	}
}
