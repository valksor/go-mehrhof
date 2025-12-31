//go:build !testbinary
// +build !testbinary

package commands

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/valksor/go-mehrhof/internal/storage"
	"github.com/valksor/go-mehrhof/internal/testutil"
)

// TestContext provides test context for command tests.
type TestContext struct {
	T         *testing.T
	StdoutBuf *bytes.Buffer
	StderrBuf *bytes.Buffer
	RootCmd   *cobra.Command
	Workspace *storage.Workspace
	Cleanup   func()
	TmpDir    string
}

// NewTestContext creates a test context for command testing.
// It sets up a temporary directory, workspace, and captures output.
func NewTestContext(t *testing.T) *TestContext {
	t.Helper()

	tmpDir := t.TempDir()

	// Create stdout/stderr buffers
	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}

	// Set up workspace
	ws, err := storage.OpenWorkspace(tmpDir, nil)
	if err != nil {
		t.Fatalf("Open workspace: %v", err)
	}
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("Ensure initialized: %v", err)
	}

	// Create a test root command
	rootCmd := createTestRootCommand(stdoutBuf, stderrBuf)

	// Set working directory
	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	cleanup := func() {
		_ = os.Chdir(oldWd)
	}

	return &TestContext{
		T:         t,
		TmpDir:    tmpDir,
		StdoutBuf: stdoutBuf,
		StderrBuf: stderrBuf,
		RootCmd:   rootCmd,
		Workspace: ws,
		Cleanup:   cleanup,
	}
}

// createTestRootCommand creates a minimal root command for testing.
func createTestRootCommand(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mehr",
		Short: "Test command",
		Long:  "Test command for testing",
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetContext(context.Background())
	return cmd
}

// ExecuteCommand executes a command with the given arguments.
func ExecuteCommand(cmd *cobra.Command, args ...string) error {
	ctx := context.Background()
	cmd.SetContext(ctx)
	cmd.SetArgs(args)
	return cmd.Execute()
}

// ExecuteCommandWithContext executes a command with a custom context.
func ExecuteCommandWithContext(ctx context.Context, cmd *cobra.Command, args ...string) error {
	cmd.SetContext(ctx)
	cmd.SetArgs(args)
	return cmd.Execute()
}

// SetupTestWorkspace creates a test workspace in the given directory.
func SetupTestWorkspace(t *testing.T) string {
	t.Helper()
	return testutil.CreateTempGitRepo(t)
}

// SetupTestGitRepo creates a test git repository.
func SetupTestGitRepo(t *testing.T) string {
	t.Helper()
	return testutil.CreateTempGitRepo(t)
}

// AssertOutputContains fails the test if the output doesn't contain the substring.
func AssertOutputContains(t *testing.T, buf *bytes.Buffer, substr string) {
	t.Helper()

	output := buf.String()
	if !strings.Contains(output, substr) {
		t.Errorf("output does not contain %q\nGot:\n%s", substr, output)
	}
}

// AssertOutputNotContains fails the test if the output contains the substring.
func AssertOutputNotContains(t *testing.T, buf *bytes.Buffer, substr string) {
	t.Helper()

	output := buf.String()
	if strings.Contains(output, substr) {
		t.Errorf("output should not contain %q\nGot:\n%s", substr, output)
	}
}

// AssertOutputEquals fails the test if the output doesn't match exactly.
func AssertOutputEquals(t *testing.T, buf *bytes.Buffer, expected string) {
	t.Helper()

	output := buf.String()
	if output != expected {
		t.Errorf("output mismatch\nGot:\n%s\nWant:\n%s", output, expected)
	}
}

// AssertStdoutContains is a helper for TestContext.
func (tc *TestContext) AssertStdoutContains(substr string) {
	AssertOutputContains(tc.T, tc.StdoutBuf, substr)
}

// AssertStderrContains is a helper for TestContext.
func (tc *TestContext) AssertStderrContains(substr string) {
	AssertOutputContains(tc.T, tc.StderrBuf, substr)
}

// AssertStdoutNotContains is a helper for TestContext.
func (tc *TestContext) AssertStdoutNotContains(substr string) {
	AssertOutputNotContains(tc.T, tc.StdoutBuf, substr)
}

// Execute executes the root command with arguments.
func (tc *TestContext) Execute(args ...string) error {
	return ExecuteCommand(tc.RootCmd, args...)
}

// ExecuteWithContext executes the root command with a custom context.
func (tc *TestContext) ExecuteWithContext(ctx context.Context, args ...string) error {
	return ExecuteCommandWithContext(ctx, tc.RootCmd, args...)
}

// CreateTestTaskFile creates a test task file in the test directory.
func (tc *TestContext) CreateTestTaskFile(filename, title, description string) string {
	return testutil.CreateTaskFile(tc.T, tc.TmpDir, filename, title, description)
}

// CreateTestTaskDir creates a test task directory.
func (tc *TestContext) CreateTestTaskDir(taskName, readmeContent string, subtasks []string) string {
	return testutil.CreateTaskDir(tc.T, tc.TmpDir, taskName, readmeContent, subtasks)
}

// GetWorkspaceConfig returns the workspace configuration.
func (tc *TestContext) GetWorkspaceConfig() (*storage.WorkspaceConfig, error) {
	return tc.Workspace.LoadConfig()
}

// SaveWorkspaceConfig saves the workspace configuration.
func (tc *TestContext) SaveWorkspaceConfig(cfg *storage.WorkspaceConfig) error {
	return tc.Workspace.SaveConfig(cfg)
}

// CreateActiveTask creates an active task in the workspace.
func (tc *TestContext) CreateActiveTask(taskID, ref string) *storage.ActiveTask {
	activeTask := storage.NewActiveTask(taskID, ref, tc.Workspace.WorkPath(taskID))
	if err := tc.Workspace.SaveActiveTask(activeTask); err != nil {
		tc.T.Fatalf("Save active task: %v", err)
	}
	return activeTask
}

// CreateTaskWork creates a task work directory and files.
func (tc *TestContext) CreateTaskWork(taskID, title string) *storage.TaskWork {
	work, err := tc.Workspace.CreateWork(taskID, storage.SourceInfo{
		Type:    "file",
		Ref:     "task.md",
		Content: testutil.SampleTaskContent(title),
	})
	if err != nil {
		tc.T.Fatalf("Create work: %v", err)
	}
	work.Metadata.Title = title
	if err := tc.Workspace.SaveWork(work); err != nil {
		tc.T.Fatalf("Save work: %v", err)
	}
	return work
}

// WithGit initializes a git repository in the test directory.
func (tc *TestContext) WithGit() {
	testutil.CreateTempGitRepoInDir(tc.T, tc.TmpDir)
}

// StdoutString returns the stdout buffer as a string.
func (tc *TestContext) StdoutString() string {
	return tc.StdoutBuf.String()
}

// StderrString returns the stderr buffer as a string.
func (tc *TestContext) StderrString() string {
	return tc.StderrBuf.String()
}

// ResetOutput resets the stdout and stderr buffers.
func (tc *TestContext) ResetOutput() {
	tc.StdoutBuf.Reset()
	tc.StderrBuf.Reset()
}

// AddSubCommand adds a subcommand to the root command.
func (tc *TestContext) AddSubCommand(cmd *cobra.Command) {
	tc.RootCmd.AddCommand(cmd)
}

// WithRegisteredSubCommands adds common subcommands to the root command.
func (tc *TestContext) WithRegisteredSubCommands(cmds ...*cobra.Command) {
	for _, cmd := range cmds {
		tc.RootCmd.AddCommand(cmd)
	}
}

// CreateFile creates a file in the test directory.
func (tc *TestContext) CreateFile(relativePath, content string) {
	fullPath := filepath.Join(tc.TmpDir, relativePath)
	testutil.WriteFile(tc.T, fullPath, content)
}

// AssertFileExists fails if the file doesn't exist.
func (tc *TestContext) AssertFileExists(relativePath string) {
	testutil.AssertFileExists(tc.T, filepath.Join(tc.TmpDir, relativePath))
}

// AssertFileNotExists fails if the file exists.
func (tc *TestContext) AssertFileNotExists(relativePath string) {
	testutil.AssertFileNotExists(tc.T, filepath.Join(tc.TmpDir, relativePath))
}

// AssertFileContains fails if the file doesn't contain the content.
func (tc *TestContext) AssertFileContains(relativePath, content string) {
	testutil.AssertFileContains(tc.T, filepath.Join(tc.TmpDir, relativePath), content)
}
