// Package retry provides a generic retryable operation helper with exponential
// backoff and jitter. It is inspired by database transaction retry patterns
// where transient failures (lock contention, temporary unavailability) can be
// resolved by waiting and retrying.
//
// The core function RetryableOp executes a function up to maxRetries times,
// sleeping between attempts with exponential backoff capped at 5 minutes.
// Jitter is applied to avoid thundering-herd problems.
//
// Callers can supply custom retry-eligibility checks via WithRetryCheck, or
// rely on the built-in IsRetryable which recognises common transient error
// strings (lock files, temporary failures, etc.).
package retry
