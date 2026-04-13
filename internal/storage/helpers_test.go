package storage

import "testing"

// newTestStore creates a Store backed by a temporary directory.
// The store is configured with saveInProject=true for predictable paths.
func newTestStore(t *testing.T) *Store {
	t.Helper()

	return NewStore(t.TempDir(), true)
}
