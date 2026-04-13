package cli

import "fmt"

// ConnectionError indicates a socket connection failure.
type ConnectionError struct {
	Err error
}

func (e *ConnectionError) Error() string {
	return fmt.Sprintf("connection error: %v", e.Err)
}

func (e *ConnectionError) Unwrap() error {
	return e.Err
}

// TimeoutError indicates an operation timed out.
type TimeoutError struct {
	Err error
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("timeout: %v", e.Err)
}

func (e *TimeoutError) Unwrap() error {
	return e.Err
}

// StateError indicates an invalid task state transition.
type StateError struct {
	Err error
}

func (e *StateError) Error() string {
	return fmt.Sprintf("state error: %v", e.Err)
}

func (e *StateError) Unwrap() error {
	return e.Err
}

// NotFoundError indicates a requested resource was not found.
type NotFoundError struct {
	Err error
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("not found: %v", e.Err)
}

func (e *NotFoundError) Unwrap() error {
	return e.Err
}

// UsageError indicates incorrect command usage.
type UsageError struct {
	Err error
}

func (e *UsageError) Error() string {
	return fmt.Sprintf("usage error: %v", e.Err)
}

func (e *UsageError) Unwrap() error {
	return e.Err
}
