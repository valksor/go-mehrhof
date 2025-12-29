// Package errors provides common error types and utilities for all providers.
package errors

import (
	"errors"
	"fmt"
	"net"
	"net/http"
)

// Common error types that can be used across providers.
// These are base errors that providers can wrap with provider-specific context.
var (
	// ErrNoToken is returned when no API token is configured.
	ErrNoToken = NewBaseError(ErrorCodeNoToken, "api token not found")

	// ErrUnauthorized is returned when the API token is invalid or expired.
	ErrUnauthorized = NewBaseError(ErrorCodeUnauthorized, "token unauthorized or expired")

	// ErrRateLimited is returned when the API rate limit is exceeded.
	ErrRateLimited = NewBaseError(ErrorCodeRateLimited, "api rate limit exceeded")

	// ErrNetworkError is returned for network-related errors.
	ErrNetworkError = NewBaseError(ErrorCodeNetworkError, "network error")

	// ErrNotFound is returned when a resource is not found.
	ErrNotFound = NewBaseError(ErrorCodeNotFound, "resource not found")

	// ErrInvalidReference is returned when a reference is invalid.
	ErrInvalidReference = NewBaseError(ErrorCodeInvalidReference, "invalid reference")
)

// Error codes for categorizing errors.
type ErrorCode int

const (
	ErrorCodeUnknown ErrorCode = iota
	ErrorCodeNoToken
	ErrorCodeUnauthorized
	ErrorCodeRateLimited
	ErrorCodeNetworkError
	ErrorCodeNotFound
	ErrorCodeInvalidReference
	ErrorCodeInsufficientScope // For tokens with insufficient permissions
)

// BaseError is a typed error that can be identified by code.
type BaseError struct {
	Msg  string
	Code ErrorCode
}

func (e *BaseError) Error() string {
	return e.Msg
}

// NewBaseError creates a new BaseError with the given code and message.
func NewBaseError(code ErrorCode, msg string) error {
	return &BaseError{Code: code, Msg: msg}
}

// ProviderError wraps an error with provider name for better error messages.
type ProviderError struct {
	Err      error
	Provider string
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("%s: %v", e.Provider, e.Err)
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}

// NewProviderError wraps an error with provider context.
func NewProviderError(provider string, err error) error {
	if err == nil {
		return nil
	}
	return &ProviderError{Provider: provider, Err: err}
}

// IsNoToken returns true if err is or wraps ErrNoToken.
func IsNoToken(err error) bool {
	return errors.Is(err, ErrNoToken)
}

// IsUnauthorized returns true if err is or wraps ErrUnauthorized.
func IsUnauthorized(err error) bool {
	return errors.Is(err, ErrUnauthorized)
}

// IsRateLimited returns true if err is or wraps ErrRateLimited.
func IsRateLimited(err error) bool {
	return errors.Is(err, ErrRateLimited)
}

// IsNetworkError returns true if err is or wraps ErrNetworkError.
func IsNetworkError(err error) bool {
	return errors.Is(err, ErrNetworkError)
}

// IsNotFound returns true if err is or wraps ErrNotFound.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsInvalidReference returns true if err is or wraps ErrInvalidReference.
func IsInvalidReference(err error) bool {
	return errors.Is(err, ErrInvalidReference)
}

// WrapHTTPError converts HTTP status codes to typed errors.
// The providerName is used to create provider-specific wrapped errors.
// The baseErrors map should contain status codes to base errors for provider-specific mappings.
func WrapHTTPError(err error, providerName string, baseErrors map[int]error) error {
	if err == nil {
		return nil
	}

	// Check for network errors first
	var netErr net.Error
	if errors.As(err, &netErr) {
		return NewProviderError(providerName, fmt.Errorf("%w: %v", ErrNetworkError, err))
	}

	// Check for HTTP errors
	// Try common interfaces for HTTP status codes
	type statusCoder interface {
		StatusCode() int
	}
	type httpStatuser interface {
		HTTPStatusCode() int
	}

	var statusCode int
	if sc, ok := err.(statusCoder); ok {
		statusCode = sc.StatusCode()
	} else if hs, ok := err.(httpStatuser); ok {
		statusCode = hs.HTTPStatusCode()
	} else {
		// No status code available, return as-is
		return err
	}

	// Check provider-specific mappings first
	if baseErr, ok := baseErrors[statusCode]; ok {
		return NewProviderError(providerName, fmt.Errorf("%w: %v", baseErr, err))
	}

	// Default mappings
	switch statusCode {
	case http.StatusUnauthorized:
		return NewProviderError(providerName, fmt.Errorf("%w: %v", ErrUnauthorized, err))
	case http.StatusForbidden:
		return NewProviderError(providerName, fmt.Errorf("%w: %v", ErrRateLimited, err))
	case http.StatusNotFound:
		return NewProviderError(providerName, fmt.Errorf("%w: %v", ErrNotFound, err))
	case http.StatusTooManyRequests:
		return NewProviderError(providerName, fmt.Errorf("%w: %v", ErrRateLimited, err))
	default:
		return err
	}
}

// NoTokenError creates a provider-specific "no token" error.
func NoTokenError(provider string) error {
	return NewProviderError(provider, ErrNoToken)
}

// UnauthorizedError creates a provider-specific unauthorized error.
func UnauthorizedError(provider string, err error) error {
	return NewProviderError(provider, fmt.Errorf("%w: %v", ErrUnauthorized, err))
}

// RateLimitedError creates a provider-specific rate limit error.
func RateLimitedError(provider string, detail string) error {
	return NewProviderError(provider, fmt.Errorf("%w: %s", ErrRateLimited, detail))
}

// NotFoundError creates a provider-specific not found error.
func NotFoundError(provider string, resource string) error {
	return NewProviderError(provider, fmt.Errorf("%w: %s", ErrNotFound, resource))
}

// InvalidReferenceError creates a provider-specific invalid reference error.
func InvalidReferenceError(provider string, ref string) error {
	return NewProviderError(provider, fmt.Errorf("%w: %s", ErrInvalidReference, ref))
}
