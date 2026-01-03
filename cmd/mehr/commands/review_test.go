//go:build !testbinary
// +build !testbinary

package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewCommand_Properties(t *testing.T) {
	if reviewCmd.Use != "review" {
		t.Errorf("Use = %q, want %q", reviewCmd.Use, "review")
	}

	if reviewCmd.Short == "" {
		t.Error("Short description is empty")
	}

	if reviewCmd.Long == "" {
		t.Error("Long description is empty")
	}

	if reviewCmd.RunE == nil {
		t.Error("RunE not set")
	}
}

func TestReviewCommand_Flags(t *testing.T) {
	tests := []struct {
		name         string
		flagName     string
		shorthand    string
		defaultValue string
	}{
		{
			name:         "tool flag",
			flagName:     "tool",
			shorthand:    "",
			defaultValue: "coderabbit",
		},
		{
			name:         "output flag",
			flagName:     "output",
			shorthand:    "o",
			defaultValue: "",
		},
		{
			name:         "agent-review flag",
			flagName:     "agent-review",
			shorthand:    "",
			defaultValue: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := reviewCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("flag %q not found", tt.flagName)

				return
			}

			if flag.DefValue != tt.defaultValue {
				t.Errorf("flag %q default value = %q, want %q", tt.flagName, flag.DefValue, tt.defaultValue)
			}

			if tt.shorthand != "" {
				shorthand := reviewCmd.Flags().ShorthandLookup(tt.shorthand)
				if shorthand == nil {
					t.Errorf("shorthand %q not found for flag %q", tt.shorthand, tt.flagName)
				}
			}
		})
	}
}

func TestReviewCommand_ShortDescription(t *testing.T) {
	expected := "Run code review on current changes"
	if reviewCmd.Short != expected {
		t.Errorf("Short = %q, want %q", reviewCmd.Short, expected)
	}
}

func TestReviewCommand_LongDescriptionContains(t *testing.T) {
	contains := []string{
		"code review",
		"CodeRabbit",
		"Review Status",
	}

	for _, substr := range contains {
		if !containsString(reviewCmd.Long, substr) {
			t.Errorf("Long description does not contain %q", substr)
		}
	}
}

func TestReviewCommand_DocumentsReviewStatuses(t *testing.T) {
	statuses := []string{
		"COMPLETE",
		"ISSUES",
		"ERROR",
	}

	for _, status := range statuses {
		if !containsString(reviewCmd.Long, status) {
			t.Errorf("Long description does not document status %q", status)
		}
	}
}

func TestReviewCommand_Examples(t *testing.T) {
	examples := []string{
		"mehr review",
		"--tool",
		"--output",
	}

	for _, example := range examples {
		if !containsString(reviewCmd.Long, example) {
			t.Errorf("Long description does not contain example %q", example)
		}
	}
}

func TestReviewCommand_RegisteredInRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "review" {
			found = true

			break
		}
	}
	if !found {
		t.Error("review command not registered in root command")
	}
}

func TestReviewCommand_OutputFlagHasShorthand(t *testing.T) {
	flag := reviewCmd.Flags().Lookup("output")
	if flag == nil {
		t.Fatal("output flag not found")
	}
	if flag.Shorthand != "o" {
		t.Errorf("output flag shorthand = %q, want 'o'", flag.Shorthand)
	}
}

func TestReviewCommand_DefaultTool(t *testing.T) {
	flag := reviewCmd.Flags().Lookup("tool")
	if flag == nil {
		t.Fatal("tool flag not found")
	}
	if flag.DefValue != "coderabbit" {
		t.Errorf("tool flag default = %q, want 'coderabbit'", flag.DefValue)
	}
}

func TestContainsIssues(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected bool
	}{
		{
			name:     "contains error",
			output:   "Found an error in the code",
			expected: true,
		},
		{
			name:     "contains warning",
			output:   "Warning: potential issue",
			expected: true,
		},
		{
			name:     "contains issue",
			output:   "This is an issue",
			expected: true,
		},
		{
			name:     "contains recommend",
			output:   "I recommend using a different approach",
			expected: true,
		},
		{
			name:     "clean output",
			output:   "All checks passed successfully",
			expected: false,
		},
		{
			name:     "empty output",
			output:   "",
			expected: false,
		},
		{
			name:     "case insensitive",
			output:   "ERROR in line 5",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsIssues(tt.output)
			if result != tt.expected {
				t.Errorf("containsIssues(%q) = %v, want %v", tt.output, result, tt.expected)
			}
		})
	}
}

// TestGetNextReviewFilename tests the getNextReviewFilename function.
func TestGetNextReviewFilename(t *testing.T) {
	tests := []struct {
		name           string
		existingFiles  []string
		expectedSuffix string
	}{
		{
			name:           "no existing reviews",
			existingFiles:  []string{},
			expectedSuffix: "review-1.txt",
		},
		{
			name: "one existing review",
			existingFiles: []string{
				"review-1.txt",
			},
			expectedSuffix: "review-2.txt",
		},
		{
			name: "multiple existing reviews",
			existingFiles: []string{
				"review-1.txt",
				"review-2.txt",
				"review-3.txt",
			},
			expectedSuffix: "review-4.txt",
		},
		{
			name: "gaps in numbering",
			existingFiles: []string{
				"review-1.txt",
				"review-5.txt",
			},
			expectedSuffix: "review-2.txt",
		},
		{
			name: "non-review files present",
			existingFiles: []string{
				"other.txt",
				"readme.md",
			},
			expectedSuffix: "review-1.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			reviewsDir := filepath.Join(tmpDir, "reviews")
			if err := os.MkdirAll(reviewsDir, 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}

			// Create existing files
			for _, fname := range tt.existingFiles {
				if strings.HasPrefix(fname, "review-") {
					path := filepath.Join(reviewsDir, fname)
					if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
						t.Fatalf("write file: %v", err)
					}
				}
			}

			// Test with full path (workDir should be reviews dir)
			result := getNextReviewFilename(reviewsDir)

			// Check that the result ends with the expected suffix
			expectedFile := tt.expectedSuffix
			if !strings.Contains(result, expectedFile) {
				t.Errorf("getNextReviewFilename() = %q, want to contain %q", result, expectedFile)
			}
		})
	}

	// Test that it handles non-existent directory gracefully
	t.Run("non-existent directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		nonExistentDir := filepath.Join(tmpDir, "nonexistent")

		result := getNextReviewFilename(nonExistentDir)

		// Should still return a valid filename
		if !strings.Contains(result, "review-1.txt") {
			t.Errorf("getNextReviewFilename() with non-existent dir = %q, want review-1.txt", result)
		}
	})
}
