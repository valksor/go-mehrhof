package socket

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractMentions(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected []string
	}{
		{
			name:     "no mentions",
			message:  "This is a regular message",
			expected: nil,
		},
		{
			name:     "single mention",
			message:  "Check @main.go for the implementation",
			expected: []string{"main.go"},
		},
		{
			name:     "multiple mentions",
			message:  "Look at @main.go and @pkg/utils.go",
			expected: []string{"main.go", "pkg/utils.go"},
		},
		{
			name:     "mention with path",
			message:  "See @internal/server/handler.go",
			expected: []string{"internal/server/handler.go"},
		},
		{
			name:     "duplicate mentions",
			message:  "Check @main.go and then @main.go again",
			expected: []string{"main.go"},
		},
		{
			name:     "mention at start",
			message:  "@README.md has the docs",
			expected: []string{"README.md"},
		},
		{
			name:     "mention at end",
			message:  "Documentation is in @README.md",
			expected: []string{"README.md"},
		},
		{
			name:     "no space before @",
			message:  "Check email@example.com for contact",
			expected: []string{"example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMentions(tt.message)
			if len(result) != len(tt.expected) {
				t.Errorf("extractMentions(%q) = %v, want %v", tt.message, result, tt.expected)

				return
			}
			for i, mention := range result {
				if mention != tt.expected[i] {
					t.Errorf("extractMentions(%q)[%d] = %q, want %q", tt.message, i, mention, tt.expected[i])
				}
			}
		})
	}
}

func TestResolveMentions(t *testing.T) {
	// Create a temp directory with test files
	tmpDir := t.TempDir()

	// Create test files
	testContent := "package main\n\nfunc main() {}\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(testContent), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name            string
		message         string
		workDir         string
		expectResolved  int
		expectExpansion bool
	}{
		{
			name:            "no mentions",
			message:         "This is a regular message",
			workDir:         tmpDir,
			expectResolved:  0,
			expectExpansion: false,
		},
		{
			name:            "existing file",
			message:         "Check @main.go",
			workDir:         tmpDir,
			expectResolved:  1,
			expectExpansion: true,
		},
		{
			name:            "non-existent file",
			message:         "Check @nonexistent.go",
			workDir:         tmpDir,
			expectResolved:  0,
			expectExpansion: true, // Still expands with "not found" message
		},
		{
			name:            "no work dir",
			message:         "Check @main.go",
			workDir:         "",
			expectResolved:  0,
			expectExpansion: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expanded, resolved := resolveMentions(tt.message, tt.workDir)

			if len(resolved) != tt.expectResolved {
				t.Errorf("resolveMentions(%q) resolved %d files, want %d", tt.message, len(resolved), tt.expectResolved)
			}

			hasExpansion := len(expanded) > len(tt.message)
			if hasExpansion != tt.expectExpansion {
				t.Errorf("resolveMentions(%q) expansion=%v, want expansion=%v", tt.message, hasExpansion, tt.expectExpansion)
			}
		})
	}
}

func TestChatMessage(t *testing.T) {
	msg := ChatMessage{
		ID:       "msg-1",
		Role:     "user",
		Content:  "Hello",
		Mentions: []string{"main.go"},
	}

	if msg.ID != "msg-1" {
		t.Errorf("ChatMessage.ID = %q, want %q", msg.ID, "msg-1")
	}
	if msg.Role != "user" {
		t.Errorf("ChatMessage.Role = %q, want %q", msg.Role, "user")
	}
	if len(msg.Mentions) != 1 || msg.Mentions[0] != "main.go" {
		t.Errorf("ChatMessage.Mentions = %v, want [main.go]", msg.Mentions)
	}
}
