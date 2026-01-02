package conductor

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/valksor/go-mehrhof/internal/vcs"
)

// Make quality target exit codes.
const (
	makeTargetUpToDate = 0 // Target is up to date
	makeTargetNeedsRun = 1 // Target needs to be run
	makeTargetNotFound = 2 // Target doesn't exist or error
)

// QualityOptions configures the quality phase.
type QualityOptions struct {
	Target       string // Make target to run (default: "quality")
	SkipPrompt   bool   // Skip confirmation prompt if files changed
	AllowFailure bool   // Continue even if quality check fails
}

// DefaultQualityOptions returns default quality options.
func DefaultQualityOptions() QualityOptions {
	return QualityOptions{
		Target:       "quality",
		SkipPrompt:   false,
		AllowFailure: false,
	}
}

// QualityResult holds the result of the quality check.
type QualityResult struct {
	Ran          bool     // Whether quality check ran
	Passed       bool     // Whether it passed
	Output       string   // Command output
	FilesChanged []string // Files modified by quality checks
	UserAborted  bool     // User chose to abort
}

// HasQualityTarget checks if the Makefile has a quality target.
func (c *Conductor) HasQualityTarget(ctx context.Context) bool {
	if c.workspace == nil {
		return false
	}

	makefilePath := filepath.Join(c.workspace.Root(), "Makefile")
	if _, err := os.Stat(makefilePath); os.IsNotExist(err) {
		return false
	}

	// Check if quality target exists using make -q
	cmd := exec.CommandContext(ctx, "make", "-q", "quality")
	cmd.Dir = c.workspace.Root()
	err := cmd.Run()

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode() != makeTargetNotFound
	}

	return err == nil
}

// RunQuality runs the quality phase before finish.
func (c *Conductor) RunQuality(ctx context.Context, opts QualityOptions) (*QualityResult, error) {
	result := &QualityResult{Ran: false, Passed: true}

	// Check if quality target exists
	if !c.HasQualityTarget(ctx) {
		return result, nil
	}

	target := opts.Target
	if target == "" {
		target = "quality"
	}

	// Get git status before running quality
	var beforeFiles []string
	if c.git != nil {
		files, _ := c.git.Status(ctx)
		for _, f := range files {
			beforeFiles = append(beforeFiles, f.Path)
		}
	}

	// Run make quality
	result.Ran = true
	cmd := exec.CommandContext(ctx, "make", target)
	cmd.Dir = c.workspace.Root()

	output, err := cmd.CombinedOutput()
	result.Output = string(output)

	if err != nil {
		result.Passed = false
		if !opts.AllowFailure {
			return result, fmt.Errorf("quality check failed\n\nOutput:\n%s\n\nTo fix:\n  1. Review the output above for specific issues\n  2. Fix the issues and run 'mehr implement' again\n  3. Or use 'mehr finish --no-quality' to proceed", result.Output)
		}
	}

	// Check if files changed after running quality
	if c.git != nil {
		afterFiles, _ := c.git.Status(ctx)
		result.FilesChanged = detectChangedFiles(beforeFiles, afterFiles)
	}

	// If files changed, prompt user
	if len(result.FilesChanged) > 0 && !opts.SkipPrompt {
		fmt.Println("\nQuality checks modified the following files:")
		for _, f := range result.FilesChanged {
			fmt.Printf("  - %s\n", f)
		}
		fmt.Println()
		fmt.Print("Continue with finish? [Y/n]: ")

		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return result, fmt.Errorf("read response: %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response == "n" || response == "no" {
			result.UserAborted = true
			return result, nil
		}

		// Stage the changes
		if c.git != nil {
			if err := c.git.AddAll(ctx); err != nil {
				return result, fmt.Errorf("stage quality changes: %w", err)
			}
		}
	}

	return result, nil
}

// detectChangedFiles compares file states before and after an operation,
// returning paths of files that are new or have been modified.
func detectChangedFiles(beforePaths []string, afterFiles []vcs.FileStatus) []string {
	// Build set of files that existed before
	beforeSet := make(map[string]struct{}, len(beforePaths))
	for _, path := range beforePaths {
		beforeSet[path] = struct{}{}
	}

	// Track changed files (using map to deduplicate)
	changedSet := make(map[string]struct{})

	for _, f := range afterFiles {
		// File is changed if it's new (not in beforeSet) or modified/staged
		_, existedBefore := beforeSet[f.Path]
		if !existedBefore || f.IsModified() || f.IsStaged() {
			changedSet[f.Path] = struct{}{}
		}
	}

	// Convert to slice
	changed := make([]string, 0, len(changedSet))
	for path := range changedSet {
		changed = append(changed, path)
	}
	return changed
}
