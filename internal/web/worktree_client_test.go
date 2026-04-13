package web

import (
	"testing"
)

func TestNewWorktreeCreatorClient(t *testing.T) {
	client := NewWorktreeCreatorClient("/tmp/test.sock")
	if client == nil {
		t.Fatal("NewWorktreeCreatorClient returned nil")
	}
	if client.globalSocketPath != "/tmp/test.sock" {
		t.Errorf("globalSocketPath = %q, want %q", client.globalSocketPath, "/tmp/test.sock")
	}
}

func TestGetOrCreateWorktreeSocket_NoSocket(t *testing.T) {
	client := NewWorktreeCreatorClient("/tmp/nonexistent.sock")
	_, err := client.GetOrCreateWorktreeSocket("/some/path")
	if err == nil {
		t.Error("expected error for nonexistent socket, got nil")
	}
}
