package conductor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/valksor/go-mehrhof/internal/storage"
	"github.com/valksor/go-mehrhof/internal/vcs"
)

func TestDefaultQualityOptions(t *testing.T) {
	opts := DefaultQualityOptions()

	if opts.Target != "quality" {
		t.Errorf("Target = %q, want %q", opts.Target, "quality")
	}
	if opts.SkipPrompt != false {
		t.Errorf("SkipPrompt = %v, want false", opts.SkipPrompt)
	}
	if opts.AllowFailure != false {
		t.Errorf("AllowFailure = %v, want false", opts.AllowFailure)
	}
}

func TestQualityOptionsStruct(t *testing.T) {
	opts := QualityOptions{
		Target:       "lint",
		SkipPrompt:   true,
		AllowFailure: true,
	}

	if opts.Target != "lint" {
		t.Errorf("Target = %q, want %q", opts.Target, "lint")
	}
	if opts.SkipPrompt != true {
		t.Errorf("SkipPrompt = %v, want true", opts.SkipPrompt)
	}
	if opts.AllowFailure != true {
		t.Errorf("AllowFailure = %v, want true", opts.AllowFailure)
	}
}

func TestQualityResultStruct(t *testing.T) {
	result := QualityResult{
		Ran:          true,
		Passed:       false,
		Output:       "error: lint failed",
		FilesChanged: []string{"file1.go", "file2.go"},
		UserAborted:  true,
	}

	if result.Ran != true {
		t.Errorf("Ran = %v, want true", result.Ran)
	}
	if result.Passed != false {
		t.Errorf("Passed = %v, want false", result.Passed)
	}
	if result.Output != "error: lint failed" {
		t.Errorf("Output = %q, want %q", result.Output, "error: lint failed")
	}
	if len(result.FilesChanged) != 2 {
		t.Errorf("FilesChanged len = %d, want 2", len(result.FilesChanged))
	}
	if result.UserAborted != true {
		t.Errorf("UserAborted = %v, want true", result.UserAborted)
	}
}

func TestHasQualityTarget_NoWorkspace(t *testing.T) {
	c := &Conductor{}
	ctx := context.Background()

	if c.HasQualityTarget(ctx) {
		t.Error("HasQualityTarget should return false when workspace is nil")
	}
}

func TestHasQualityTarget_NoMakefile(t *testing.T) {
	tmpDir := t.TempDir()

	ws, err := storage.OpenWorkspace(tmpDir, nil)
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}

	c := &Conductor{workspace: ws}
	ctx := context.Background()

	if c.HasQualityTarget(ctx) {
		t.Error("HasQualityTarget should return false when no Makefile exists")
	}
}

func TestHasQualityTarget_WithQualityTarget(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// Create a Makefile with quality target
	makefileContent := `.PHONY: quality
quality:
	@echo "running quality checks"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "Makefile"), []byte(makefileContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	ws, err := storage.OpenWorkspace(tmpDir, nil)
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}

	c := &Conductor{workspace: ws}
	ctx := context.Background()

	if !c.HasQualityTarget(ctx) {
		t.Error("HasQualityTarget should return true when quality target exists")
	}
}

func TestHasQualityTarget_WithoutQualityTarget(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// Create a Makefile without quality target
	makefileContent := `.PHONY: build
build:
	@echo "building"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "Makefile"), []byte(makefileContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	ws, err := storage.OpenWorkspace(tmpDir, nil)
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}

	c := &Conductor{workspace: ws}
	ctx := context.Background()

	if c.HasQualityTarget(ctx) {
		t.Error("HasQualityTarget should return false when quality target doesn't exist")
	}
}

func TestRunQuality_NoTarget(t *testing.T) {
	tmpDir := t.TempDir()

	ws, err := storage.OpenWorkspace(tmpDir, nil)
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}

	c := &Conductor{workspace: ws}
	ctx := context.Background()

	result, err := c.RunQuality(ctx, DefaultQualityOptions())
	if err != nil {
		t.Fatalf("RunQuality: %v", err)
	}

	if result.Ran {
		t.Error("Ran should be false when quality target doesn't exist")
	}
	if !result.Passed {
		t.Error("Passed should be true when quality target doesn't exist")
	}
}

func TestRunQuality_TargetExists(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// Create a Makefile with quality target that succeeds
	makefileContent := `.PHONY: quality
quality:
	@echo "quality check passed"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "Makefile"), []byte(makefileContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	ws, err := storage.OpenWorkspace(tmpDir, nil)
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}

	c := &Conductor{workspace: ws}
	ctx := context.Background()

	opts := DefaultQualityOptions()
	opts.SkipPrompt = true // Skip user prompt in tests

	result, err := c.RunQuality(ctx, opts)
	if err != nil {
		t.Fatalf("RunQuality: %v", err)
	}

	if !result.Ran {
		t.Error("Ran should be true when quality target exists")
	}
	if !result.Passed {
		t.Error("Passed should be true when quality check succeeds")
	}
	if result.Output == "" {
		t.Error("Output should not be empty")
	}
}

func TestRunQuality_TargetFails(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// Create a Makefile with quality target that fails
	makefileContent := `.PHONY: quality
quality:
	@echo "quality check failed" && exit 1
`
	if err := os.WriteFile(filepath.Join(tmpDir, "Makefile"), []byte(makefileContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	ws, err := storage.OpenWorkspace(tmpDir, nil)
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}

	c := &Conductor{workspace: ws}
	ctx := context.Background()

	opts := DefaultQualityOptions()
	opts.SkipPrompt = true

	result, err := c.RunQuality(ctx, opts)
	if err == nil {
		t.Error("RunQuality should return error when quality check fails")
	}
	if !result.Ran {
		t.Error("Ran should be true")
	}
	if result.Passed {
		t.Error("Passed should be false when quality check fails")
	}
}

func TestRunQuality_AllowFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// Create a Makefile with quality target that fails
	makefileContent := `.PHONY: quality
quality:
	@echo "quality check failed" && exit 1
`
	if err := os.WriteFile(filepath.Join(tmpDir, "Makefile"), []byte(makefileContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	ws, err := storage.OpenWorkspace(tmpDir, nil)
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}

	c := &Conductor{workspace: ws}
	ctx := context.Background()

	opts := QualityOptions{
		Target:       "quality",
		SkipPrompt:   true,
		AllowFailure: true,
	}

	result, err := c.RunQuality(ctx, opts)
	if err != nil {
		t.Errorf("RunQuality should not return error when AllowFailure is true: %v", err)
	}
	if !result.Ran {
		t.Error("Ran should be true")
	}
	if result.Passed {
		t.Error("Passed should be false")
	}
}

func TestRunQuality_CustomTarget(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// Create a Makefile with custom lint target
	makefileContent := `.PHONY: lint
lint:
	@echo "lint passed"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "Makefile"), []byte(makefileContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	ws, err := storage.OpenWorkspace(tmpDir, nil)
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}

	c := &Conductor{workspace: ws}
	ctx := context.Background()

	opts := QualityOptions{
		Target:     "lint",
		SkipPrompt: true,
	}

	// Note: This will fail because HasQualityTarget checks for "quality" specifically
	// But RunQuality will try to run the specified target
	result, err := c.RunQuality(ctx, opts)
	// Since there's no "quality" target, it should not run
	if err != nil {
		t.Fatalf("RunQuality: %v", err)
	}
	if result.Ran {
		t.Error("Ran should be false because HasQualityTarget checks for 'quality' target")
	}
}

func TestDetectChangedFiles(t *testing.T) {
	tests := []struct {
		name        string
		beforePaths []string
		afterFiles  []vcs.FileStatus
		wantChanged []string
	}{
		{
			name:        "no changes",
			beforePaths: []string{"a.go", "b.go"},
			afterFiles:  []vcs.FileStatus{},
			wantChanged: []string{},
		},
		{
			name:        "new file added",
			beforePaths: []string{"a.go"},
			afterFiles: []vcs.FileStatus{
				{Path: "b.go", Index: 'A', WorkDir: ' '},
			},
			wantChanged: []string{"b.go"},
		},
		{
			name:        "existing file modified",
			beforePaths: []string{"a.go", "b.go"},
			afterFiles: []vcs.FileStatus{
				{Path: "a.go", Index: ' ', WorkDir: 'M'},
			},
			wantChanged: []string{"a.go"},
		},
		{
			name:        "existing file staged",
			beforePaths: []string{"a.go"},
			afterFiles: []vcs.FileStatus{
				{Path: "a.go", Index: 'M', WorkDir: ' '},
			},
			wantChanged: []string{"a.go"},
		},
		{
			name:        "mixed changes",
			beforePaths: []string{"a.go", "b.go"},
			afterFiles: []vcs.FileStatus{
				{Path: "a.go", Index: 'M', WorkDir: ' '},
				{Path: "c.go", Index: 'A', WorkDir: ' '},
				{Path: "d.go", Index: '?', WorkDir: '?'},
			},
			wantChanged: []string{"a.go", "c.go", "d.go"},
		},
		{
			name:        "file unchanged in status",
			beforePaths: []string{"a.go"},
			afterFiles: []vcs.FileStatus{
				{Path: "a.go", Index: ' ', WorkDir: ' '},
			},
			wantChanged: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectChangedFiles(tt.beforePaths, tt.afterFiles)

			// Convert to map for comparison since order doesn't matter
			gotMap := make(map[string]bool)
			for _, f := range got {
				gotMap[f] = true
			}
			wantMap := make(map[string]bool)
			for _, f := range tt.wantChanged {
				wantMap[f] = true
			}

			if len(gotMap) != len(wantMap) {
				t.Errorf("detectChangedFiles() got %d files, want %d", len(gotMap), len(wantMap))
				t.Errorf("got: %v", got)
				t.Errorf("want: %v", tt.wantChanged)
				return
			}

			for f := range wantMap {
				if !gotMap[f] {
					t.Errorf("detectChangedFiles() missing file %q", f)
				}
			}
		})
	}
}

func TestMakeTargetExitCodes(t *testing.T) {
	if makeTargetUpToDate != 0 {
		t.Errorf("makeTargetUpToDate = %d, want 0", makeTargetUpToDate)
	}
	if makeTargetNeedsRun != 1 {
		t.Errorf("makeTargetNeedsRun = %d, want 1", makeTargetNeedsRun)
	}
	if makeTargetNotFound != 2 {
		t.Errorf("makeTargetNotFound = %d, want 2", makeTargetNotFound)
	}
}
