package errors

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"
)

// mockHTTPError is a mock error that implements StatusCode()
type mockHTTPError struct {
	msg    string
	status int
}

func (e *mockHTTPError) Error() string {
	return e.msg
}

func (e *mockHTTPError) StatusCode() int {
	return e.status
}

// mockHTTPStatuser is a mock error that implements HTTPStatusCode()
type mockHTTPStatuser struct {
	msg    string
	status int
}

func (e *mockHTTPStatuser) Error() string {
	return e.msg
}

func (e *mockHTTPStatuser) HTTPStatusCode() int {
	return e.status
}

func TestIsNoToken(t *testing.T) {
	err := NoTokenError("github")
	if !IsNoToken(err) {
		t.Error("NoTokenError should be identifiable by IsNoToken")
	}
	if IsUnauthorized(err) {
		t.Error("NoTokenError should not be Unauthorized")
	}
}

func TestIsUnauthorized(t *testing.T) {
	err := UnauthorizedError("github", errors.New("bad token"))
	if !IsUnauthorized(err) {
		t.Error("UnauthorizedError should be identifiable by IsUnauthorized")
	}
}

func TestIsRateLimited(t *testing.T) {
	err := RateLimitedError("github", "retry after 60s")
	if !IsRateLimited(err) {
		t.Error("RateLimitedError should be identifiable by IsRateLimited")
	}
}

func TestIsNotFound(t *testing.T) {
	err := NotFoundError("github", "issue #123")
	if !IsNotFound(err) {
		t.Error("NotFoundError should be identifiable by IsNotFound")
	}
}

func TestIsInvalidReference(t *testing.T) {
	err := InvalidReferenceError("github", "bad:ref")
	if !IsInvalidReference(err) {
		t.Error("InvalidReferenceError should be identifiable by IsInvalidReference")
	}
}

func TestIsNetworkError(t *testing.T) {
	err := NewProviderError("github", fmt.Errorf("%w: connection refused", ErrNetworkError))
	if !IsNetworkError(err) {
		t.Error("Network error should be identifiable by IsNetworkError")
	}
}

func TestWrapHTTPError(t *testing.T) {
	tests := []struct {
		err        error
		baseErrors map[int]error
		name       string
		provider   string
		wantCode   ErrorCode
	}{
		{
			name:     "401 unauthorized",
			err:      &mockHTTPError{status: http.StatusUnauthorized, msg: "unauthorized"},
			provider: "github",
			wantCode: ErrorCodeUnauthorized,
		},
		{
			name:     "403 rate limit",
			err:      &mockHTTPError{status: http.StatusForbidden, msg: "rate limit"},
			provider: "github",
			wantCode: ErrorCodeRateLimited,
		},
		{
			name:     "404 not found",
			err:      &mockHTTPError{status: http.StatusNotFound, msg: "not found"},
			provider: "github",
			wantCode: ErrorCodeNotFound,
		},
		{
			name:     "429 too many requests",
			err:      &mockHTTPError{status: http.StatusTooManyRequests, msg: "too many"},
			provider: "github",
			wantCode: ErrorCodeRateLimited,
		},
		{
			name:     "network error",
			err:      &net.DNSError{Err: "lookup failed", IsTimeout: false},
			provider: "github",
			wantCode: ErrorCodeNetworkError,
		},
		{
			name:     "nil error",
			err:      nil,
			provider: "github",
			wantCode: ErrorCodeUnknown,
		},
		{
			name:     "custom base error",
			err:      &mockHTTPError{status: http.StatusForbidden, msg: "insufficient scope"},
			provider: "github",
			baseErrors: map[int]error{
				http.StatusForbidden: NewBaseError(ErrorCodeInsufficientScope, "insufficient scope"),
			},
			wantCode: ErrorCodeInsufficientScope,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := WrapHTTPError(tt.err, tt.provider, tt.baseErrors)
			if tt.err == nil {
				if wrapped != nil {
					t.Errorf("WrapHTTPError(nil) should return nil, got %v", wrapped)
				}
				return
			}

			// Unwrap to get the base error
			var providerErr *ProviderError
			if !errors.As(wrapped, &providerErr) {
				// Some errors might not be wrapped
				if tt.wantCode == ErrorCodeUnknown {
					return
				}
				t.Fatalf("error should be a ProviderError, got %T", wrapped)
			}

			var baseErr *BaseError
			if !errors.As(providerErr.Err, &baseErr) {
				t.Fatalf("provider error should wrap a BaseError, got %T", providerErr.Err)
			}

			if baseErr.Code != tt.wantCode {
				t.Errorf("error code = %d, want %d", baseErr.Code, tt.wantCode)
			}
		})
	}
}

func TestHTTPStatusCodeInterface(t *testing.T) {
	// Test StatusCode() interface
	err1 := &mockHTTPError{status: http.StatusUnauthorized}
	wrapped1 := WrapHTTPError(err1, "test", nil)
	if !IsUnauthorized(wrapped1) {
		t.Error("StatusCode() interface should work")
	}

	// Test HTTPStatusCode() interface
	err2 := &mockHTTPStatuser{status: http.StatusUnauthorized}
	wrapped2 := WrapHTTPError(err2, "test", nil)
	if !IsUnauthorized(wrapped2) {
		t.Error("HTTPStatusCode() interface should work")
	}
}

func TestProviderError_Unwrap(t *testing.T) {
	base := errors.New("base error")
	err := &ProviderError{Provider: "github", Err: base}

	if !errors.Is(err, base) {
		t.Error("ProviderError.Unwrap() should return the base error")
	}
}

func TestProviderError_Error(t *testing.T) {
	err := &ProviderError{Provider: "github", Err: errors.New("something failed")}
	expected := "github: something failed"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}
