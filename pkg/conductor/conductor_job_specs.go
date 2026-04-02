package conductor

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/valksor/kvelmo/pkg/git"
	"github.com/valksor/kvelmo/pkg/storage"
)

// detectSpecificationFiles scans for specification files and adds any new ones
// to the work unit's Specifications list. Uses the storage layer path which
// respects the saveInProject config setting.
// Must be called with c.mu held.
func (c *Conductor) detectSpecificationFiles() {
	if c.workUnit == nil || c.store == nil {
		return
	}

	// Build set of known specs for quick lookup (normalized for deduplication)
	known := make(map[string]bool)
	for _, sp := range c.workUnit.Specifications {
		known[filepath.Clean(sp)] = true
	}

	specDir := c.store.SpecificationsDir(c.workUnit.ID)
	entries, err := os.ReadDir(specDir)
	if err != nil {
		// Directory may not exist yet
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "specification-") || !strings.HasSuffix(name, ".md") {
			continue
		}
		fullPath := filepath.Join(specDir, name)
		if !known[filepath.Clean(fullPath)] {
			c.workUnit.Specifications = append(c.workUnit.Specifications, fullPath)
			slog.Info("detected new specification file", "path", fullPath)
		}
	}
}

// copySpecsToRepo copies specification files to an in-repo path if configured.
// Must be called with c.mu held.
func (c *Conductor) copySpecsToRepo() {
	if c.workUnit == nil {
		return
	}

	settings := c.getEffectiveSettings()
	outputPath := settings.Storage.SpecOutputPath
	if outputPath == "" {
		return
	}

	// Interpolate variables
	key := ""
	if c.workUnit.ExternalID != "" {
		key = c.workUnit.ExternalID
	}
	slug := slugify(c.workUnit.Title)

	workDir := c.getWorkDir()

	for _, specPath := range c.workUnit.Specifications {
		data, err := os.ReadFile(specPath)
		if err != nil {
			c.logVerbosef("Warning: could not read spec for repo copy: %v", err)

			continue
		}

		// Interpolate output path per spec
		resolved := outputPath
		resolved = strings.ReplaceAll(resolved, "{key}", key)
		resolved = strings.ReplaceAll(resolved, "{slug}", slug)

		fullPath := filepath.Join(workDir, resolved)

		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			c.logVerbosef("Warning: could not create spec output dir: %v", err)

			continue
		}

		if err := os.WriteFile(fullPath, data, 0o644); err != nil {
			c.logVerbosef("Warning: could not write spec to repo: %v", err)
		} else {
			slog.Info("spec copied to repo", "path", fullPath)
		}
	}
}

// copyPlanToRepo copies the latest plan to an in-repo path if configured.
// Must be called with c.mu held.
func (c *Conductor) copyPlanToRepo() {
	if c.workUnit == nil || c.store == nil {
		return
	}

	s := c.getEffectiveSettings()
	outputPath := s.Storage.PlanOutputPath
	if outputPath == "" {
		return
	}

	planStore := storage.NewPlanStore(c.store)
	plan, err := planStore.GetLatestPlan(c.workUnit.ID)
	if err != nil || plan == nil {
		return
	}

	// Load plan history markdown
	history, err := planStore.LoadPlanHistory(c.workUnit.ID, plan.ID)
	if err != nil {
		c.logVerbosef("Warning: could not read plan history for repo copy: %v", err)

		return
	}

	// Interpolate variables
	key := ""
	if c.workUnit.ExternalID != "" {
		key = c.workUnit.ExternalID
	}
	slug := slugify(c.workUnit.Title)

	resolved := outputPath
	resolved = strings.ReplaceAll(resolved, "{key}", key)
	resolved = strings.ReplaceAll(resolved, "{slug}", slug)

	workDir := c.getWorkDir()
	fullPath := filepath.Join(workDir, resolved)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		c.logVerbosef("Warning: could not create plan output dir: %v", err)

		return
	}

	if err := os.WriteFile(fullPath, []byte(history), 0o644); err != nil {
		c.logVerbosef("Warning: could not write plan to repo: %v", err)
	} else {
		slog.Info("plan copied to repo", "path", fullPath)
	}
}

// commitRepoSpecs commits spec and plan files that were copied to the repo.
// Must be called after copySpecsToRepo/copyPlanToRepo and with c.mu held.
func (c *Conductor) commitRepoSpecs(ctx context.Context) {
	settings := c.getEffectiveSettings()
	if settings.Storage.CommitSpecs == nil || !*settings.Storage.CommitSpecs {
		return
	}
	if settings.Storage.SpecOutputPath == "" && settings.Storage.PlanOutputPath == "" {
		return
	}

	workDir := c.getWorkDir()
	repo, err := git.Open(workDir)
	if err != nil {
		return
	}

	// Stage only spec/plan output files
	key := ""
	slug := ""
	if c.workUnit != nil {
		if c.workUnit.ExternalID != "" {
			key = c.workUnit.ExternalID
		}
		slug = slugify(c.workUnit.Title)
	}

	var filesToStage []string
	for _, outputPath := range []string{settings.Storage.SpecOutputPath, settings.Storage.PlanOutputPath} {
		if outputPath == "" {
			continue
		}
		resolved := outputPath
		resolved = strings.ReplaceAll(resolved, "{key}", key)
		resolved = strings.ReplaceAll(resolved, "{slug}", slug)
		fullPath := filepath.Join(workDir, resolved)
		if _, err := os.Stat(fullPath); err == nil {
			filesToStage = append(filesToStage, fullPath)
		} else {
			slog.Warn("spec commit: output file not found", "path", fullPath)
		}
	}

	if len(filesToStage) == 0 {
		return
	}

	if err := repo.StageFiles(ctx, filesToStage...); err != nil {
		slog.Warn("spec stage failed", "error", err)

		return
	}

	hasChanges, _ := repo.HasUncommittedChanges(ctx)
	if !hasChanges {
		return
	}

	label := "specification"
	if key != "" {
		label += " for " + key
	}
	commitMsg := c.formatCheckpointMessage("Add " + label)

	sha, err := repo.Commit(ctx, commitMsg)
	if err != nil {
		slog.Warn("spec commit failed", "error", err)

		return
	}

	c.workUnit.Checkpoints = append(c.workUnit.Checkpoints, sha)
	slog.Info("spec committed to repo", "sha", sha)
}
