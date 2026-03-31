// conductor_quality.go — quality gate checks run during the Review phase.
package conductor

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/valksor/kvelmo/pkg/failclass"
	"github.com/valksor/kvelmo/pkg/findings"
	"github.com/valksor/kvelmo/pkg/quality"
	"github.com/valksor/kvelmo/pkg/security"
	"github.com/valksor/kvelmo/pkg/settings"
)

// runQualityGate checks code quality before submission.
// Detects project type and runs language-specific checks, then runs
// the external review tool if installed and configured.
// If auto-fix is enabled and the current phase matches, quality gate
// failures are fed back to the agent for bounded retry.
func (c *Conductor) runQualityGate(ctx context.Context) error {
	err := c.runQualityGateChecks(ctx)
	if err == nil {
		return nil
	}

	// Check if auto-fix should handle this failure
	if c.shouldAutoFix() {
		slog.Info("quality gate: auto-fix enabled, attempting automatic fix", "error", err)

		if fixErr := c.runQualityAutoFix(ctx, err); fixErr == nil {
			return nil
		} else {
			return fixErr
		}
	}

	return err
}

// runQualityGateChecks runs the actual quality gate checks without auto-fix logic.
// Uses quality.RunGates to orchestrate language-specific checks and slop detection
// through a unified checker/gate framework, then runs security and external review.
func (c *Conductor) runQualityGateChecks(ctx context.Context) error {
	workDir := c.getWorkDir()

	// Run unified quality gates: auto-detects project type and includes slop.
	if err := c.runUnifiedQualityGates(ctx, workDir); err != nil {
		return err
	}

	// Run structured quality gates (QualityRunner) with hold-the-line filtering.
	if err := c.runStructuredQualityGates(ctx, workDir); err != nil {
		return err
	}

	// Run security scan if configured as a quality gate.
	if err := c.qualityGateSecurity(ctx, workDir); err != nil {
		return err
	}

	// External review tool runs for all project types if installed and configured.
	return c.qualityGateExternalReview(ctx, workDir)
}

// runUnifiedQualityGates uses quality.RunGates to orchestrate all checkers
// (language linters + slop detection) through the gate framework.
// Applies hold-the-line filtering so only new findings block.
func (c *Conductor) runUnifiedQualityGates(ctx context.Context, workDir string) error {
	checkers := quality.DetectCheckers(workDir)

	gates := []quality.Gate{
		quality.NoErrorsGate{},
		quality.NoSecurityIssuesGate{},
	}

	// Scale the outer timeout with the number of checkers so polyglot projects
	// (Go + Node + Python + Slop) get enough budget. Each checker also has its
	// own perCheckerTimeout (60s) enforced inside RunGates.
	outerTimeout := time.Duration(len(checkers)) * quality.PerCheckerTimeout
	gateCtx, cancel := context.WithTimeout(ctx, outerTimeout)
	defer cancel()

	result, err := quality.RunGates(gateCtx, workDir, checkers, gates)
	if err != nil {
		return fmt.Errorf("quality gates: %w", err)
	}

	slog.Info("quality gate: unified check complete",
		"total", result.Total,
		"blockers", result.Blocked,
		"passed", result.Passed,
	)

	if result.Passed {
		return nil
	}

	// Apply hold-the-line: only block on findings introduced by the agent.
	filtered := c.classifyFindings(ctx, result.Blockers)
	if len(filtered) == 0 {
		slog.Info("quality gate: all findings are pre-existing, passing")

		return nil
	}

	// Apply failure classification if enabled.
	filtered = c.applyFailureClassification(ctx, filtered)
	if filtered == nil {
		return nil
	}

	var msgs []string
	for _, f := range filtered {
		switch {
		case f.File != "" && f.Line > 0:
			msgs = append(msgs, fmt.Sprintf("  %s:%d: [%s] %s", f.File, f.Line, f.Rule, f.Message))
		case f.File != "":
			msgs = append(msgs, fmt.Sprintf("  %s: [%s] %s", f.File, f.Rule, f.Message))
		default:
			msgs = append(msgs, fmt.Sprintf("  [%s] %s", f.Rule, f.Message))
		}
	}

	return fmt.Errorf("quality gate failed with %d finding(s):\n%s", len(filtered), strings.Join(msgs, "\n"))
}

// runStructuredQualityGates runs the QualityRunner (if configured) and applies
// hold-the-line filtering to only gate on findings introduced by the agent.
func (c *Conductor) runStructuredQualityGates(ctx context.Context, workDir string) error {
	c.mu.RLock()
	runner := c.qualityRunner
	c.mu.RUnlock()

	if runner == nil {
		return nil
	}

	passed, findingMessages, err := runner.RunGates(ctx, workDir)
	if err != nil {
		return fmt.Errorf("quality runner: %w", err)
	}

	if passed || len(findingMessages) == 0 {
		return nil
	}

	// Convert string findings to structured findings for hold-the-line classification.
	structured := messagesToFindings(findingMessages)
	filtered := c.classifyFindings(ctx, structured)

	if len(filtered) == 0 {
		slog.Info("quality gate: all findings are pre-existing, passing")

		return nil
	}

	// Apply failure classification if enabled.
	filtered = c.applyFailureClassification(ctx, filtered)
	if filtered == nil {
		return nil
	}

	var msgs []string
	for _, f := range filtered {
		msgs = append(msgs, f.Message)
	}

	return fmt.Errorf("quality gate failed with %d finding(s):\n%s", len(filtered), strings.Join(msgs, "\n"))
}

// applyFailureClassification runs failure pattern classification on findings
// when enabled in settings. If only flaky findings remain, it returns nil
// (passing the gate). Otherwise it returns the genuine/intermittent findings.
func (c *Conductor) applyFailureClassification(_ context.Context, filtered []findings.Finding) []findings.Finding {
	effectiveSettings := c.getEffectiveSettings()
	if effectiveSettings == nil || !effectiveSettings.Workflow.FailureClassification.Enabled {
		return filtered
	}

	window := effectiveSettings.Workflow.FailureClassification.HistoryWindow
	if window == 0 {
		window = 10
	}

	// Lazily initialize the persistent history so IsFlaky can detect recurring patterns
	// across quality gate runs within the same session.
	c.mu.Lock()
	if c.failclassHistory == nil {
		c.failclassHistory = failclass.NewHistory(window)
	}
	history := c.failclassHistory
	c.mu.Unlock()

	classifier := failclass.New(history)
	classified := classifier.Classify(filtered)

	stats := classifier.Stats(classified)
	slog.Info("quality gate: failure classification",
		"total", stats.Total,
		"flaky", stats.Flaky,
		"genuine", stats.Genuine,
		"intermittent", stats.Intermittent,
	)

	// If all findings are flaky, pass the gate.
	if stats.Genuine == 0 && stats.Intermittent == 0 {
		slog.Info("quality gate: all findings classified as flaky, passing")

		return nil
	}

	// Return only genuine and intermittent findings.
	var genuine []findings.Finding
	for _, f := range classified {
		if f.Classification != string(failclass.ClassFlaky) {
			genuine = append(genuine, f)
		}
	}

	return genuine
}

// messagesToFindings converts plain string finding messages to Finding structs.
// These lack file/line information, so they will be classified as OriginUnknown
// by hold-the-line (and thus still gate).
func messagesToFindings(messages []string) []findings.Finding {
	result := make([]findings.Finding, len(messages))
	for i, msg := range messages {
		result[i] = findings.Finding{
			Message:  msg,
			Severity: findings.SeverityHigh,
			Category: findings.CategoryQuality,
			Source:   "quality_runner",
		}
	}

	return result
}

// runQualityGateAsync runs the quality gate in a background goroutine
// and caches the result in WorkUnit. Called during Review() so the result
// is ready by the time Submit() is called, avoiding a blocking wait.
func (c *Conductor) runQualityGateAsync() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("quality gate goroutine panicked", "panic", r)
				// Update state to reflect failure
				c.mu.Lock()
				if c.workUnit != nil {
					passed := false
					c.workUnit.QualityGatePassed = &passed
					c.workUnit.QualityGateError = fmt.Sprintf("panic: %v", r)
					c.workUnit.UpdatedAt = time.Now()
					c.persistState()
				}
				c.mu.Unlock()
			}
		}()

		err := c.runQualityGate(c.lifecycleCtx)

		c.mu.Lock()
		defer c.mu.Unlock()

		if c.workUnit == nil {
			return
		}

		passed := err == nil
		c.workUnit.QualityGatePassed = &passed
		if err != nil {
			c.workUnit.QualityGateError = err.Error()
		} else {
			c.workUnit.QualityGateError = ""
		}
		c.workUnit.UpdatedAt = time.Now()
		c.persistState()

		slog.Debug("quality gate completed async", "passed", passed, "error", c.workUnit.QualityGateError)
	}()
}

// qualityGateExternalReview runs an external CLI review tool if installed and configured.
// Skips silently if the CLI is not found. Mode comes from workflow settings:
//   - never  → skip silently
//   - always → run without prompting
//   - ask    → block on a user prompt (default)
func (c *Conductor) qualityGateExternalReview(ctx context.Context, workDir string) error {
	effectiveSettings := c.getEffectiveSettings()
	command := effectiveSettings.Workflow.ExternalReview.Command
	if command == "" {
		command = "coderabbit"
	}

	// Reject path separators to prevent executing unintended binaries
	// from worktree-controlled config files.
	if strings.ContainsAny(command, "/\\") {
		return fmt.Errorf("external review command must be a plain name, not a path: %q", command)
	}

	toolPath, err := exec.LookPath(command)
	if err != nil {
		slog.Debug("quality gate: external review tool not found, skipping", "command", command)

		return nil
	}

	mode := effectiveSettings.Workflow.ExternalReview.Mode
	slog.Info("quality gate: external review check", "mode", mode, "command", command, "path", toolPath)
	if mode == "" {
		slog.Info("quality gate: mode is empty, defaulting to ask")
		mode = settings.ExternalReviewAsk
	}

	switch mode {
	case settings.ExternalReviewNever:
		slog.Debug("quality gate: external review mode=never, skipping")

		return nil

	case settings.ExternalReviewAlways:
		// fall through to run

	case settings.ExternalReviewAsk:
		run, promptErr := c.promptUser(ctx, fmt.Sprintf("Run %s review? (this may take several minutes)", command))
		if promptErr != nil {
			slog.Warn("quality gate: external review prompt cancelled, skipping", "err", promptErr)

			return nil
		}

		if !run {
			slog.Debug("quality gate: user declined external review")

			return nil
		}

	default:
		slog.Warn("quality gate: unknown external review mode, skipping", "mode", mode)

		return nil
	}

	// Derive from parent context so stop/abort cancels the review.
	// Previously used a detached context, but that let reviews outlive conductor shutdown.
	reviewCtx, cancel := context.WithTimeout(ctx, externalReviewTimeout)
	defer cancel()

	cmd := exec.CommandContext(reviewCtx, toolPath, "review")
	cmd.Dir = workDir
	output, runErr := cmd.CombinedOutput()
	if runErr != nil {
		return fmt.Errorf("%s review failed:\n%s", command, string(output))
	}

	slog.Debug("quality gate: external review passed", "command", command)

	return nil
}

// qualityGateSecurity runs security scanning when RequireSecurityScan is enabled.
// Runs secret detection and dependency vulnerability scanning, then applies
// hold-the-line filtering so only newly introduced findings block the gate.
func (c *Conductor) qualityGateSecurity(ctx context.Context, workDir string) error {
	s := c.getEffectiveSettings()
	if s == nil || !s.Workflow.Policy.RequireSecurityScan {
		return nil
	}

	slog.Info("quality gate: running security scan")
	c.emit(ConductorEvent{
		Type:    "quality_gate",
		State:   c.machine.State(),
		Message: "Running security scan...",
	})

	secCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	runner := security.NewRunner()
	reports, err := runner.Run(secCtx, workDir)
	if err != nil {
		slog.Warn("quality gate: security scan failed", "err", err)

		return nil // non-fatal; don't block on scan infrastructure failures
	}

	unified := security.ReportsToFindings(reports)
	if len(unified) == 0 {
		slog.Info("quality gate: security scan clean")

		return nil
	}

	// Apply hold-the-line: only block on findings introduced by the agent.
	filtered := c.classifyFindings(ctx, unified)
	if len(filtered) == 0 {
		slog.Info("quality gate: all security findings are pre-existing, passing")

		return nil
	}

	// Only block on High/Critical severity findings.
	var blocking []findings.Finding
	for _, f := range filtered {
		if f.Severity <= findings.SeverityHigh {
			blocking = append(blocking, f)
		}
	}

	if len(blocking) == 0 {
		slog.Info("quality gate: security findings are low severity, passing", "count", len(filtered))

		return nil
	}

	var msgs []string
	for _, f := range blocking {
		msgs = append(msgs, fmt.Sprintf("  [%s] %s: %s", f.Severity, f.Rule, f.Message))
	}

	return fmt.Errorf("security scan found %d blocking issue(s):\n%s", len(blocking), strings.Join(msgs, "\n"))
}

// externalReviewTimeout is the timeout for external review tools (slower than local linters).
const externalReviewTimeout = 5 * time.Minute
