//go:build !testbinary
// +build !testbinary

package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestInitCommand(t *testing.T) {
	tests := []struct {
		name            string
		wantFiles       []string
		wantInOutput    []string
		wantNotInOutput []string
		setupGit        bool
		runTwice        bool
	}{
		{
			name:     "creates workspace structure",
			setupGit: false,
			wantFiles: []string{
				".mehrhof",             // workspace directory
				".mehrhof/config.yaml", // config file
				".mehrhof/.env",        // env template (created in .mehrhof)
			},
			wantInOutput: []string{
				"Created config file",
				"Created .env template",
				"Workspace initialized",
			},
		},
		{
			name:     "idempotent - can run twice",
			setupGit: false,
			runTwice: true,
			wantFiles: []string{
				".mehrhof",
				".mehrhof/config.yaml",
			},
			wantInOutput: []string{
				"Config file already exists",
				"Workspace initialized",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := NewTestContext(t)
			defer tc.Cleanup()

			// Create a git repo if requested
			if tt.setupGit {
				tc.WithGit()
			}

			// Add init command to root
			rootCmd := &cobra.Command{
				Use:   "mehr",
				Short: "Test command",
			}
			rootCmd.SetOut(tc.StdoutBuf)
			rootCmd.SetErr(tc.StderrBuf)
			rootCmd.AddCommand(initCmd)

			// Execute init command (possibly twice)
			rootCmd.SetArgs([]string{"init"})
			err := rootCmd.Execute()
			if err != nil {
				t.Fatalf("First Execute: %v", err)
			}

			if tt.runTwice {
				// Reset output and run again
				tc.ResetOutput()
				rootCmd.SetArgs([]string{"init"})
				err = rootCmd.Execute()
				if err != nil {
					t.Fatalf("Second Execute: %v", err)
				}
			}

			// Check expected files exist
			for _, file := range tt.wantFiles {
				fullPath := filepath.Join(tc.TmpDir, file)
				if _, err := os.Stat(fullPath); os.IsNotExist(err) {
					t.Errorf("file not created: %s", file)
				}
			}

			// Check output
			output := tc.StdoutString()
			for _, want := range tt.wantInOutput {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q\nGot: %s", want, output)
				}
			}

			for _, notWant := range tt.wantNotInOutput {
				if strings.Contains(output, notWant) {
					t.Errorf("output should not contain %q\nGot: %s", notWant, output)
				}
			}
		})
	}
}

func TestInitCommand_WithGitRepo(t *testing.T) {
	tc := NewTestContext(t)
	defer tc.Cleanup()

	// Initialize git repo first
	tc.WithGit()

	rootCmd := &cobra.Command{
		Use:   "mehr",
		Short: "Test command",
	}
	rootCmd.SetOut(tc.StdoutBuf)
	rootCmd.AddCommand(initCmd)

	rootCmd.SetArgs([]string{"init"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Check .mehrhof was created
	tc.AssertFileExists(".mehrhof/config.yaml")

	// Check .env template was created in .mehrhof/
	tc.AssertFileExists(".mehrhof/.env")

	// Check .env has content
	content, err := os.ReadFile(filepath.Join(tc.TmpDir, ".mehrhof", ".env"))
	if err != nil {
		t.Fatalf("Read .env: %v", err)
	}
	envContent := string(content)
	if !strings.Contains(envContent, "# Mehrhof environment variables") {
		t.Errorf(".env template missing expected content")
	}
	if !strings.Contains(envContent, "ANTHROPIC_API_KEY") {
		t.Errorf(".env template missing ANTHROPIC_API_KEY example")
	}
}

func TestInitCommand_Twice(t *testing.T) {
	tc := NewTestContext(t)
	defer tc.Cleanup()

	rootCmd := &cobra.Command{
		Use:   "mehr",
		Short: "Test command",
	}
	rootCmd.SetOut(tc.StdoutBuf)
	rootCmd.AddCommand(initCmd) // Add init command

	// First run
	rootCmd.SetArgs([]string{"init"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("First Execute: %v", err)
	}

	firstOutput := tc.StdoutString()

	// Reset and run again
	tc.ResetOutput()
	rootCmd.SetArgs([]string{"init"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Second Execute: %v", err)
	}

	secondOutput := tc.StdoutString()

	// First run should create, second should say already exists
	if !strings.Contains(firstOutput, "Created config file") {
		t.Errorf("First run should create config file, got: %s", firstOutput)
	}
	if !strings.Contains(secondOutput, "Config file already exists") {
		t.Errorf("Second run should say config exists, got: %s", secondOutput)
	}
}

func TestCreateEnvTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")

	err := createEnvTemplate(envPath)
	if err != nil {
		t.Fatalf("createEnvTemplate: %v", err)
	}

	// Check file exists
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		t.Fatal(".env file was not created")
	}

	// Check file permissions
	info, err := os.Stat(envPath)
	if err != nil {
		t.Fatalf("Stat .env: %v", err)
	}
	// 0o600 = rw------- (user read/write only)
	if info.Mode().Perm() != 0o600 {
		t.Errorf(".env permissions = %o, want %o", info.Mode().Perm(), 0o600)
	}

	// Check content
	content, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("Read .env: %v", err)
	}

	contentStr := string(content)
	expectedStrings := []string{
		"# Mehrhof environment variables",
		"ANTHROPIC_API_KEY",
		"GLM_API_KEY",
		"GITHUB_TOKEN",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(contentStr, expected) {
			t.Errorf(".env missing expected string: %s", expected)
		}
	}
}
