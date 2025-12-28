package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/valksor/go-mehrhof/internal/conductor"
)

// maxReviewFileNumber is the maximum number to try when generating review filenames
const maxReviewFileNumber = 100

var (
	reviewTool           string
	reviewOutput         string
	reviewAgentReviewing string
)

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Run code review on current changes",
	Long: `Run an automated code review on the current task's changes.

By default, this runs CodeRabbit CLI to review code changes. The review
output is saved to the task's work directory.

Review Status:
  COMPLETE - Review passed with no issues
  ISSUES   - Review found issues that need attention
  ERROR    - Review tool failed to run

Examples:
  mehr review                     # Run CodeRabbit review
  mehr review --tool coderabbit   # Explicitly specify tool
  mehr review --output review.txt # Save to specific file`,
	RunE: runReview,
}

func init() {
	rootCmd.AddCommand(reviewCmd)

	reviewCmd.Flags().StringVar(&reviewTool, "tool", "coderabbit", "Review tool to use (coderabbit)")
	reviewCmd.Flags().StringVarP(&reviewOutput, "output", "o", "", "Output file name (default: review-N.txt)")
	reviewCmd.Flags().StringVar(&reviewAgentReviewing, "agent-reviewing", "", "Agent for review step (when using agent-based review)")
}

func runReview(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Initialize conductor with standard providers and agents
	cond, err := initializeConductor(ctx, conductor.WithVerbose(verbose))
	if err != nil {
		return err
	}

	// Check for active task
	activeTask := cond.GetActiveTask()
	if activeTask == nil {
		return fmt.Errorf("no active task - start a task first with 'mehr start'")
	}

	// Get workspace
	ws := cond.GetWorkspace()
	if ws == nil {
		return fmt.Errorf("workspace not available")
	}

	// Check if review tool is available
	toolPath, err := exec.LookPath(reviewTool)
	if err != nil {
		return fmt.Errorf("review tool '%s' not found in PATH\nInstall it with: npm install -g coderabbitai", reviewTool)
	}

	fmt.Printf("Running %s review...\n", reviewTool)

	// Run the review tool
	reviewCmd := exec.CommandContext(ctx, toolPath, "review")
	reviewCmd.Dir = ws.Root()

	output, err := reviewCmd.CombinedOutput()
	outputStr := string(output)

	// Determine review status
	var status string
	if err != nil {
		status = "ERROR"
		fmt.Printf("Review failed: %v\n", err)
	} else if containsIssues(outputStr) {
		status = "ISSUES"
		fmt.Println("Review completed with issues")
	} else {
		status = "COMPLETE"
		fmt.Println("Review completed successfully")
	}

	// Determine output filename
	outputFile := reviewOutput
	if outputFile == "" {
		outputFile = getNextReviewFilename(ws.WorkPath(activeTask.ID))
	}
	outputPath := filepath.Join(ws.WorkPath(activeTask.ID), outputFile)

	// Build review content
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# Code Review - %s\n\n", time.Now().Format("2006-01-02 15:04:05")))
	content.WriteString(fmt.Sprintf("Tool: %s\n", reviewTool))
	content.WriteString(fmt.Sprintf("Status: %s\n", status))
	content.WriteString(fmt.Sprintf("Task: %s\n", activeTask.ID))
	if activeTask.Branch != "" {
		content.WriteString(fmt.Sprintf("Branch: %s\n", activeTask.Branch))
	}
	content.WriteString("\n---\n\n")
	content.WriteString(outputStr)

	// Save review output
	if err := os.WriteFile(outputPath, []byte(content.String()), 0o644); err != nil {
		return fmt.Errorf("save review output: %w", err)
	}

	fmt.Printf("\nReview saved to: %s\n", outputPath)
	fmt.Printf("Status: %s\n", status)

	if status == "ISSUES" {
		fmt.Println("\nPlease review the issues and address them before finishing the task.")
	}

	return nil
}

// containsIssues checks if the review output indicates issues
func containsIssues(output string) bool {
	lowerOutput := strings.ToLower(output)
	issueIndicators := []string{
		"error",
		"warning",
		"issue",
		"problem",
		"fix",
		"should",
		"must",
		"consider",
		"recommend",
	}

	for _, indicator := range issueIndicators {
		if strings.Contains(lowerOutput, indicator) {
			return true
		}
	}

	return false
}

// getNextReviewFilename returns the next available review-N.txt filename
func getNextReviewFilename(workDir string) string {
	for i := 1; i <= maxReviewFileNumber; i++ {
		filename := fmt.Sprintf("review-%d.txt", i)
		path := filepath.Join(workDir, filename)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return filename
		}
	}
	return fmt.Sprintf("review-%d.txt", time.Now().Unix())
}
