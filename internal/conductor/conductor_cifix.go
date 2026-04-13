package conductor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/valksor/kvelmo/internal/ciwatch"
	"github.com/valksor/kvelmo/internal/worker"
)

const defaultMaxCIFixAttempts = 3

// startCIFixLoop begins watching CI status after PR submission.
// When CI fails, it extracts failure logs and spawns an agent to fix.
// The loop is bounded by MaxFixAttempts (default 3).
// Must be called without c.mu held. Runs in a goroutine using lifecycleCtx.
func (c *Conductor) startCIFixLoop(ctx context.Context) {
	c.mu.RLock()
	settings := c.getEffectiveSettings()
	ciCfg := settings.Workflow.CI
	prID := ""
	branch := ""
	var sourceProvider string
	if c.workUnit != nil {
		prID = c.workUnit.PRID
		branch = c.workUnit.Branch
		if c.workUnit.Source != nil {
			sourceProvider = c.workUnit.Source.Provider
		}
	}
	c.mu.RUnlock()

	if prID == "" {
		slog.Debug("cifix: no PR ID, skipping CI fix loop")

		return
	}

	// Resolve the StatusFetcher from the provider registry
	c.mu.RLock()
	providers := c.providers
	c.mu.RUnlock()

	if providers == nil || sourceProvider == "" {
		slog.Debug("cifix: no provider registry or source provider, skipping")

		return
	}

	p, err := providers.Get(sourceProvider)
	if err != nil {
		slog.Debug("cifix: could not get provider", "provider", sourceProvider, "error", err)

		return
	}

	fetcher, ok := p.(ciwatch.StatusFetcher)
	if !ok {
		slog.Debug("cifix: provider does not implement StatusFetcher", "provider", sourceProvider)

		return
	}

	maxAttempts := ciCfg.MaxFixAttempts
	if maxAttempts <= 0 {
		maxAttempts = defaultMaxCIFixAttempts
	}

	pollInterval := time.Duration(ciCfg.PollIntervalSec) * time.Second
	if pollInterval <= 0 {
		pollInterval = 30 * time.Second
	}

	// Attempt to get a LogsFetcher for richer failure context
	logsFetcher, _ := p.(ciwatch.LogsFetcher)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		slog.Info("cifix: watching CI status", "pr_id", prID, "attempt", attempt, "max", maxAttempts)

		if data, err := json.Marshal(map[string]int{"attempt": attempt, "max_attempts": maxAttempts}); err == nil {
			c.emit(ConductorEvent{
				Type:    "ci_fix_watching",
				Message: fmt.Sprintf("Watching CI status (attempt %d/%d)", attempt, maxAttempts),
				Data:    data,
			})
		}

		// Create a watcher and wait for terminal state
		watcher := ciwatch.New(fetcher, prID, pollInterval)

		statusCh := make(chan ciwatch.Status, 1)
		watcher.OnUpdate(func(s ciwatch.Status) {
			if s.State == "success" || s.State == "failure" {
				select {
				case statusCh <- s:
				default:
				}
			}
		})

		// Run watcher in background
		watchCtx, watchCancel := context.WithCancel(ctx)
		go watcher.Start(watchCtx)

		// Wait for terminal status or context cancellation
		var finalStatus ciwatch.Status
		select {
		case <-ctx.Done():
			watchCancel()

			return
		case finalStatus = <-statusCh:
			watchCancel()
		}

		if finalStatus.State == "success" {
			slog.Info("cifix: CI passed", "pr_id", prID, "attempt", attempt)

			c.emit(ConductorEvent{
				Type:    "ci_fix_success",
				Message: fmt.Sprintf("CI passed on attempt %d/%d", attempt, maxAttempts),
			})

			return
		}

		// CI failed - extract logs and attempt fix
		slog.Info("cifix: CI failed, attempting fix", "pr_id", prID, "attempt", attempt)

		ciLogs := c.extractCILogs(ctx, logsFetcher, prID, &finalStatus)

		if err := c.attemptCIFix(ctx, ciLogs, attempt, branch); err != nil {
			slog.Warn("cifix: fix attempt failed", "attempt", attempt, "error", err)

			c.emit(ConductorEvent{
				Type:    "ci_fix_attempt_failed",
				Error:   err.Error(),
				Message: fmt.Sprintf("CI fix attempt %d/%d failed: %s", attempt, maxAttempts, err.Error()),
			})

			// If the fix job itself failed, no point retrying immediately
			if attempt >= maxAttempts {
				break
			}

			continue
		}

		// Fix was applied, push and loop back to watch CI again
		slog.Info("cifix: fix applied, pushing changes", "pr_id", prID, "attempt", attempt)
	}

	// Exhausted all attempts
	slog.Warn("cifix: exhausted all fix attempts", "pr_id", prID, "max", maxAttempts)

	data, err := json.Marshal(map[string]string{
		"pr_id":        prID,
		"max_attempts": strconv.Itoa(maxAttempts),
	})
	if err != nil {
		slog.Warn("cifix: failed to marshal exhaustion data", "error", err)
		data = []byte("{}")
	}

	c.emit(ConductorEvent{
		Type:    "ci_fix_exhausted",
		Message: fmt.Sprintf("CI fix loop exhausted after %d attempts", maxAttempts),
		Data:    data,
	})
}

// attemptCIFix spawns an agent to fix a CI failure, commits the changes, and pushes.
func (c *Conductor) attemptCIFix(ctx context.Context, ciLogs string, attempt int, branch string) error {
	c.mu.RLock()
	pool := c.pool
	workUnit := c.workUnit
	c.mu.RUnlock()

	if pool == nil {
		return errors.New("no worker pool available")
	}

	if workUnit == nil {
		return errors.New("no task loaded")
	}

	prompt := buildCIFixPrompt(workUnit.Title, workUnit.Description, ciLogs, attempt)

	c.mu.Lock()
	opts := c.buildJobOptionsForPhase("implement")
	workDir := c.getWorkDir()
	c.mu.Unlock()

	var job *worker.Job
	var err error
	if cached, ok := c.lookupResponseCache(prompt); ok {
		job, err = pool.SubmitCached(worker.JobTypeImplement, workDir, prompt, cached, opts)
	} else {
		job, err = pool.SubmitWithOptions(worker.JobTypeImplement, workDir, prompt, opts)
	}
	if err != nil {
		return fmt.Errorf("submit CI fix job: %w", err)
	}

	c.emit(ConductorEvent{
		Type:    "ci_fix_started",
		JobID:   job.ID,
		Message: fmt.Sprintf("CI fix agent started (attempt %d)", attempt),
	})

	// Wait for job completion synchronously by streaming events
	stream := pool.Stream(job.ID)
	if stream == nil {
		return fmt.Errorf("could not stream CI fix job %s", job.ID)
	}

	var jobErr error

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("CI fix job timed out: %w", ctx.Err())
		case event, ok := <-stream:
			if !ok {
				goto streamDone
			}
			// Forward streaming output
			c.emit(ConductorEvent{
				Type:    "job_output",
				JobID:   job.ID,
				Message: event.Content,
			})

			if event.Type == "job_completed" {
				goto streamDone
			}

			if event.Type == "job_failed" {
				jobErr = fmt.Errorf("CI fix job failed: %s", event.Content)

				goto streamDone
			}
		}
	}
streamDone:

	if jobErr != nil {
		return jobErr
	}

	// Commit and push the fix
	c.mu.RLock()
	repo := c.git
	c.mu.RUnlock()

	if repo == nil {
		return errors.New("no git repository available")
	}

	if err := repo.StageAll(ctx); err != nil {
		return fmt.Errorf("stage CI fix changes: %w", err)
	}

	hasChanges, err := repo.HasUncommittedChanges(ctx)
	if err != nil {
		return fmt.Errorf("check uncommitted changes: %w", err)
	}

	if !hasChanges {
		return errors.New("CI fix agent produced no changes")
	}

	c.mu.RLock()
	commitMsg := c.formatCheckpointMessage(fmt.Sprintf("CI fix attempt %d", attempt))
	c.mu.RUnlock()

	sha, err := repo.Commit(ctx, commitMsg)
	if err != nil {
		return fmt.Errorf("commit CI fix: %w", err)
	}

	slog.Info("cifix: committed fix", "sha", sha, "attempt", attempt)

	// Track checkpoint
	c.mu.Lock()
	if c.workUnit != nil {
		c.workUnit.Checkpoints = append(c.workUnit.Checkpoints, sha)
	}
	c.persistState()
	c.mu.Unlock()

	// Guard against pushing after conductor shutdown.
	if c.closed.Load() {
		return errors.New("conductor closed, skipping push")
	}

	// Push to origin
	if err := repo.Push(ctx, "origin", branch); err != nil {
		return fmt.Errorf("push CI fix: %w", err)
	}

	slog.Info("cifix: pushed fix", "branch", branch, "attempt", attempt)

	return nil
}

// extractCILogs retrieves CI failure logs, falling back to a check summary.
func (c *Conductor) extractCILogs(ctx context.Context, logsFetcher ciwatch.LogsFetcher, prID string, status *ciwatch.Status) string {
	// Try the LogsFetcher if the provider supports it
	if logsFetcher != nil {
		logs, err := logsFetcher.FetchCILogs(ctx, prID)
		if err == nil && logs != "" {
			return logs
		}

		slog.Debug("cifix: FetchCILogs failed or empty, falling back to summary", "error", err)
	}

	// Fall back to a summary of failed checks
	return ciwatch.FailedChecksSummary(status)
}

// buildCIFixPrompt constructs the prompt for the CI fix agent.
func buildCIFixPrompt(title, description, ciLogs string, attempt int) string {
	return fmt.Sprintf(`Fix CI failures for the following task:

Title: %s
Description: %s

## CI Failure (attempt %d)

The CI pipeline failed after the PR was submitted. Below are the CI failure logs.
Analyze the failures and fix the code to make CI pass.

### CI Logs

%s

## Instructions

1. Read the CI failure logs carefully
2. Identify the root cause of each failure
3. Fix the code to resolve the failures
4. Do NOT introduce new features or change functionality beyond what is needed to fix CI
5. Focus on: compilation errors, lint failures, test failures, type errors
6. Commit your changes with a clear message describing what was fixed

If the failure is in tests, fix the tests or the code they are testing.
If the failure is a lint error, fix the lint issue in the source code.
If the failure is a build error, fix the build issue.
`, title, description, attempt, ciLogs)
}
