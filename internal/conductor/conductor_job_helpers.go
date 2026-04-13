package conductor

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"regexp"
	"strings"
	"time"

	"github.com/valksor/kvelmo/internal/git"
	"github.com/valksor/kvelmo/internal/provider"
	"github.com/valksor/kvelmo/internal/storage"
	"github.com/valksor/kvelmo/internal/worker"
)

// saveJobSession persists a session entry for the given job so that it can be
// resumed later.  This is a best-effort operation; errors are logged and ignored.
func (c *Conductor) saveJobSession(jobID, phase, agentType string) {
	if c.store == nil || c.workUnit == nil {
		return
	}
	sessStore := storage.NewSessionStore(c.store)
	entry := storage.SessionEntry{
		SessionID: jobID,
		AgentType: agentType,
		TaskID:    c.workUnit.ID,
		Phase:     phase,
	}
	if err := sessStore.SaveSession(entry); err != nil {
		slog.Warn("persist session failed", "task_id", c.workUnit.ID, "phase", phase, "error", err)
	}
}

func (c *Conductor) generateBranchName(wu *WorkUnit) string {
	effectiveSettings := c.getEffectiveSettings()
	pattern := effectiveSettings.Git.BranchPattern
	if pattern == "" {
		pattern = "feature/{key}--{slug}"
	}

	// Determine key
	key := wu.ID
	if wu.ExternalID != "" {
		key = wu.ExternalID
	}

	// Generate slug from title
	slug := slugify(wu.Title)

	// Determine type (provider name or "local")
	taskType := "local"
	if wu.Source != nil {
		taskType = wu.Source.Provider
	}

	// Interpolate variables
	result := pattern
	if key == "" {
		// Remove {key}-- or {key}- patterns when key is empty to avoid leading dashes
		result = strings.ReplaceAll(result, "{key}--", "")
		result = strings.ReplaceAll(result, "{key}-", "")
	}
	result = strings.ReplaceAll(result, "{key}", key)
	result = strings.ReplaceAll(result, "{slug}", slug)
	result = strings.ReplaceAll(result, "{type}", taskType)

	// Clean up: collapse multiple dashes and remove trailing dashes
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	result = strings.TrimRight(result, "-")

	return result
}

func validateBranchName(name, pattern string) error {
	if pattern == "" {
		return nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid branch validation pattern %q: %w", pattern, err)
	}
	if !re.MatchString(name) {
		return fmt.Errorf("generated branch name %q does not match required pattern %s", name, pattern)
	}

	return nil
}

// formatCheckpointMessage formats a checkpoint commit message using the same
// CommitPrefix as agent commits, keeping git history style-consistent.
func (c *Conductor) formatCheckpointMessage(message string) string {
	prefix := c.interpolatedCommitPrefix()

	return prefix + " " + message
}

// recordCheckpointMeta stores rich metadata for a checkpoint SHA.
// Caller must hold c.mu.
func (c *Conductor) recordCheckpointMeta(sha, message, state string) {
	if c.workUnit == nil {
		return
	}
	if c.workUnit.CheckpointMeta == nil {
		c.workUnit.CheckpointMeta = make(map[string]CheckpointMeta)
	}
	c.workUnit.CheckpointMeta[sha] = CheckpointMeta{
		Message:   message,
		State:     state,
		CreatedAt: time.Now(),
	}
}

func (c *Conductor) interpolatedCommitPrefix() string {
	settings := c.getEffectiveSettings()
	prefix := settings.Git.CommitPrefix
	if prefix == "" {
		return "[kvelmo]"
	}
	if c.workUnit != nil && c.workUnit.ExternalID != "" {
		prefix = strings.ReplaceAll(prefix, "{key}", c.workUnit.ExternalID)
	} else {
		prefix = strings.ReplaceAll(prefix, "{key}", "")
		prefix = strings.ReplaceAll(prefix, "[]", "[kvelmo]")
	}

	return prefix
}

func (c *Conductor) interpolatePRTitle(title string) string {
	settings := c.getEffectiveSettings()
	pattern := settings.Git.PRTitlePattern
	if pattern == "" {
		return "[kvelmo] " + title
	}

	key := ""
	taskType := "local"
	if c.workUnit != nil {
		if c.workUnit.ExternalID != "" {
			key = c.workUnit.ExternalID
		}
		if c.workUnit.Source != nil {
			taskType = c.workUnit.Source.Provider
		}
	}

	slug := ""
	if c.workUnit != nil {
		slug = slugify(c.workUnit.Title)
	}

	result := pattern
	if key == "" {
		result = strings.ReplaceAll(result, "[{key}] ", "")
		result = strings.ReplaceAll(result, "{key} ", "")
	}
	result = strings.ReplaceAll(result, "{key}", key)
	result = strings.ReplaceAll(result, "{title}", title)
	result = strings.ReplaceAll(result, "{type}", taskType)
	result = strings.ReplaceAll(result, "{slug}", slug)

	return result
}

func (c *Conductor) buildGitConventionInstructions() string {
	settings := c.getEffectiveSettings()
	var instructions []string

	prefix := c.interpolatedCommitPrefix()
	if prefix != "[kvelmo]" && prefix != "" {
		instructions = append(instructions, "- Prefix all commit messages with: "+prefix)
	}

	if settings.Git.CommitPattern != "" {
		instructions = append(instructions, "- All commit messages must match this pattern: "+settings.Git.CommitPattern)
	}

	if len(instructions) == 0 {
		return ""
	}

	return "\n\n## Git Conventions\n\n" + strings.Join(instructions, "\n") + "\n"
}

// commitValidationParams holds the data needed to validate agent commits,
// extracted from conductor state under the lock so the git subprocess can
// run without holding c.mu.
type commitValidationParams struct {
	pattern        string
	workDir        string
	lastCheckpoint string
	checkpointSHAs map[string]struct{}
}

// prepareCommitValidation extracts commit validation parameters from conductor state.
// Must be called with c.mu held. Returns nil if validation is not configured.
func (c *Conductor) prepareCommitValidation(lastCheckpoint string) *commitValidationParams {
	if lastCheckpoint == "" {
		return nil
	}

	settings := c.getEffectiveSettings()
	pattern := settings.Git.CommitPattern
	if pattern == "" {
		return nil
	}

	// Collect checkpoint SHAs so validateAgentCommits can skip kvelmo's own commits.
	cpSHAs := make(map[string]struct{}, len(c.workUnit.Checkpoints))
	for _, sha := range c.workUnit.Checkpoints {
		cpSHAs[sha] = struct{}{}
	}

	return &commitValidationParams{
		pattern:        pattern,
		workDir:        c.getWorkDir(),
		lastCheckpoint: lastCheckpoint,
		checkpointSHAs: cpSHAs,
	}
}

// validateAgentCommits checks commits made by the agent against CommitPattern.
// Emits a degraded warning if any agent commits don't match the configured pattern.
// Safe to call without c.mu held — runs git subprocess without holding the lock.
func (c *Conductor) validateAgentCommits(ctx context.Context, params *commitValidationParams) {
	if params == nil {
		return
	}

	repo, err := git.Open(params.workDir)
	if err != nil {
		slog.Debug("commit validation: could not open repo", "error", err)

		return
	}

	commits, err := repo.CommitsSince(ctx, params.lastCheckpoint)
	if err != nil {
		slog.Debug("commit validation: could not list commits", "error", err)

		return
	}

	var invalid []string
	for _, entry := range commits {
		// Skip kvelmo's own checkpoint commits (identified by SHA, not prefix).
		if _, isCheckpoint := params.checkpointSHAs[entry.SHA]; isCheckpoint {
			continue
		}
		if err := git.ValidateCommitMessage(entry.Message, params.pattern); err != nil {
			shortSHA := entry.SHA
			if len(shortSHA) > 8 {
				shortSHA = shortSHA[:8]
			}
			invalid = append(invalid, fmt.Sprintf("%s: %s", shortSHA, entry.Message))
		}
	}

	if len(invalid) == 0 {
		return
	}

	msg := fmt.Sprintf("%d commit(s) do not match pattern %s: %s",
		len(invalid), params.pattern, strings.Join(invalid, "; "))
	slog.Warn("commit validation", "violations", len(invalid), "pattern", params.pattern)

	c.emit(ConductorEvent{
		Type:           "commit_validation_warning",
		Message:        msg,
		FailureClass:   FailureClassDegraded,
		FailureMessage: msg,
	})
}

// slugify converts a string to a URL-safe slug.
func slugify(s string) string {
	// Lowercase
	s = strings.ToLower(s)
	// Replace spaces and underscores with hyphens
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	// Remove non-alphanumeric characters except hyphens
	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	// Collapse multiple hyphens
	slug := result.String()
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	// Trim leading/trailing hyphens
	slug = strings.Trim(slug, "-")
	// Limit length (use runes for UTF-8 safety)
	runes := []rune(slug)
	if len(runes) > 50 {
		slug = string(runes[:50])
		// Don't end with hyphen
		slug = strings.TrimRight(slug, "-")
	}

	return slug
}

// shouldPostTicketComment checks if ticket comments are enabled for the current provider.
func (c *Conductor) shouldPostTicketComment() bool {
	if c.workUnit == nil || c.workUnit.Source == nil {
		return false
	}

	effectiveSettings := c.getEffectiveSettings()

	switch c.workUnit.Source.Provider {
	case provider.NameGitHub:
		return effectiveSettings.Providers.GitHub.AllowTicketComment
	case provider.NameGitLab:
		return effectiveSettings.Providers.GitLab.AllowTicketComment
	case provider.NameWrike:
		return effectiveSettings.Providers.Wrike.AllowTicketComment
	case provider.NameLinear:
		return effectiveSettings.Providers.Linear.AllowTicketComment
	case provider.NameJira:
		return effectiveSettings.Providers.Jira.AllowTicketComment
	default:
		return false
	}
}

// buildJobOptions creates JobOptions with execution context for multi-project support.
// This ensures jobs carry full context (WorkDir, metadata) so any worker can execute them.
// If canary sandboxing is enabled, injects fake HOME environment variables.
// buildJobOptionsForPhase creates JobOptions with the per-phase agent override set.
func (c *Conductor) buildJobOptionsForPhase(phase string) *worker.JobOptions {
	opts := c.buildJobOptions()
	if agentName := c.resolveAgent(phase); agentName != "" {
		opts.Agent = agentName
	}

	return opts
}

func (c *Conductor) buildJobOptions() *worker.JobOptions {
	opts := &worker.JobOptions{
		WorkDir:  c.getWorkDir(), // Use isolated worktree if available
		Metadata: make(map[string]any),
	}

	// Add task metadata
	if c.workUnit != nil {
		opts.Metadata["task_id"] = c.workUnit.ID
		opts.Metadata["task_title"] = c.workUnit.Title
		if c.workUnit.ExternalID != "" {
			opts.Metadata["external_id"] = c.workUnit.ExternalID
		}
		if c.workUnit.Source != nil {
			opts.Metadata["provider"] = c.workUnit.Source.Provider
			opts.Metadata["reference"] = c.workUnit.Source.Reference
		}
	}

	// Inject canary harness environment if active
	if c.canaryHarness != nil {
		if opts.Environment == nil {
			opts.Environment = make(map[string]string)
		}
		maps.Copy(opts.Environment, c.canaryHarness.Env())
	}

	return opts
}

// toInt64 extracts an int64 from an any value (handles int64 and float64 from JSON).
func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case float64:
		return int64(n), true
	case int:
		return int64(n), true
	default:
		return 0, false
	}
}

// estimateCost returns a rough USD cost estimate based on provider pricing.
// Prices are approximate and should be updated as provider pricing changes.
func estimateCost(agentName string, inputTokens, outputTokens int64) float64 {
	var inputPricePer1M, outputPricePer1M float64

	switch agentName {
	case "anthropic":
		// Claude Sonnet 4 pricing (approximate)
		inputPricePer1M = 3.0
		outputPricePer1M = 15.0
	case "openai":
		// GPT-4o pricing (approximate)
		inputPricePer1M = 2.5
		outputPricePer1M = 10.0
	case "ollama":
		// Local models: no cost
		return 0
	default:
		// Conservative default estimate
		inputPricePer1M = 3.0
		outputPricePer1M = 15.0
	}

	return float64(inputTokens)/1_000_000*inputPricePer1M +
		float64(outputTokens)/1_000_000*outputPricePer1M
}
