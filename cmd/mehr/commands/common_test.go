//go:build !testbinary
// +build !testbinary

package commands

import (
	"testing"
)

func TestConfirmAction(t *testing.T) {
	tests := []struct {
		name        string
		skipConfirm bool
	}{
		{
			name:        "skip confirm returns true",
			skipConfirm: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When skipConfirm is true, it should return true without prompting
			result, err := confirmAction("test action", tt.skipConfirm)
			if err != nil {
				t.Fatalf("confirmAction: %v", err)
			}
			if !result {
				t.Error("expected true when skipConfirm is true")
			}
		})
	}
}

func TestGetDeduplicatingStdout(t *testing.T) {
	// Should return a non-nil writer
	w := getDeduplicatingStdout()
	if w == nil {
		t.Error("getDeduplicatingStdout returned nil")
	}

	// Calling again should return the same instance (singleton)
	w2 := getDeduplicatingStdout()
	if w != w2 {
		t.Error("getDeduplicatingStdout should return the same instance")
	}
}
