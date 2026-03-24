// conductor_quality.go — quality gate checks run during the Review phase.
package conductor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/valksor/kvelmo/pkg/failclass"
	"github.com/valksor/kvelmo/pkg/findings"
	"github.com/valksor/kvelmo/pkg/quality"
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
func (c *Conductor) runQualityGateChecks(ctx context.Context) error {
	workDir := c.getWorkDir()

	// Language-specific checks are mutually exclusive by project type.
	// Each gate function creates its own context with timeout via qualityCtx().
	//nolint:contextcheck // each language gate creates its own 60s timeout context via qualityCtx(); detached from parent so one slow gate doesn't starve the next
	if _, err := os.Stat(filepath.Join(workDir, "go.mod")); err == nil {
		if err := c.qualityGateGo(workDir); err != nil {
			return err
		}
	} else if _, err := os.Stat(filepath.Join(workDir, "package.json")); err == nil {
		if err := c.qualityGateNode(workDir); err != nil {
			return err
		}
	} else if _, err := os.Stat(filepath.Join(workDir, "setup.py")); err == nil {
		if err := c.qualityGatePython(workDir); err != nil {
			return err
		}
	} else if _, err := os.Stat(filepath.Join(workDir, "pyproject.toml")); err == nil {
		if err := c.qualityGatePython(workDir); err != nil {
			return err
		}
	} else {
		slog.Warn("quality gate: unrecognised project type, skipping language checks", "dir", workDir)
	}

	// Run slop detection on changed files (language-agnostic).
	if err := c.qualityGateSlop(ctx, workDir); err != nil {
		return err
	}

	// Run structured quality gates (QualityRunner) with hold-the-line filtering.
	if err := c.runStructuredQualityGates(ctx, workDir); err != nil {
		return err
	}

	// External review tool runs for all project types if installed and configured.
	return c.qualityGateExternalReview(ctx, workDir)
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

// qualityGateGo runs go vet for Go projects.
func (c *Conductor) qualityGateGo(workDir string) error {
	ctx, cancel := qualityCtx()
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "vet", "./...")
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go vet failed:\n%s", string(output))
	}

	return nil
}

// qualityGateNode runs npm lint and typecheck for Node.js projects.
// Each script is only executed when it exists in package.json.
func (c *Conductor) qualityGateNode(workDir string) error {
	scripts, err := readPackageJSONScripts(workDir)
	if err != nil {
		slog.Warn("quality gate: cannot read package.json scripts", "err", err)

		return nil
	}

	for _, script := range []string{"lint", "typecheck"} {
		if _, ok := scripts[script]; !ok {
			continue
		}

		ctx, cancel := qualityCtx()
		out, runErr := runNPMScript(ctx, workDir, script)
		cancel()
		if runErr != nil {
			return fmt.Errorf("npm run %s failed:\n%s", script, out)
		}
	}

	return nil
}

// qualityGatePython checks Python syntax on changed .py files.
// Uses ruff when available, otherwise falls back to py_compile.
func (c *Conductor) qualityGatePython(workDir string) error {
	if ruffPath, err := exec.LookPath("ruff"); err == nil {
		ctx, cancel := qualityCtx()
		defer cancel()
		cmd := exec.CommandContext(ctx, ruffPath, "check", ".")
		cmd.Dir = workDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("ruff check failed:\n%s", string(output))
		}

		return nil
	}

	// Fallback: py_compile on all .py files in the working directory.
	// Skip if python3 is not available (consistent with ruff check above).
	if _, err := exec.LookPath("python3"); err != nil {
		return nil
	}

	pyFiles, err := collectPyFiles(workDir)
	if err != nil || len(pyFiles) == 0 {
		return nil
	}

	ctx, cancel := qualityCtx()
	defer cancel()

	// Process files in batches to avoid exceeding OS command-line length limits.
	// Large projects can have thousands of .py files; ~100 files per batch is safe.
	const batchSize = 100
	for i := 0; i < len(pyFiles); i += batchSize {
		end := i + batchSize
		if end > len(pyFiles) {
			end = len(pyFiles)
		}
		batch := pyFiles[i:end]

		args := append([]string{"-m", "py_compile"}, batch...)
		cmd := exec.CommandContext(ctx, "python3", args...)
		cmd.Dir = workDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("python -m py_compile failed:\n%s", string(output))
		}
	}

	return nil
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

	reviewCtx, cancel := externalReviewCtx()
	defer cancel()

	//nolint:contextcheck // externalReviewCtx() creates a dedicated 5-minute timeout; independent of parent context so reviews get their full time budget
	cmd := exec.CommandContext(reviewCtx, toolPath, "review")
	cmd.Dir = workDir
	output, runErr := cmd.CombinedOutput()
	if runErr != nil {
		return fmt.Errorf("%s review failed:\n%s", command, string(output))
	}

	slog.Debug("quality gate: external review passed", "command", command)

	return nil
}

// qualityGateSlop runs the slop detector on changed files to catch
// low-quality AI-generated code patterns.
func (c *Conductor) qualityGateSlop(ctx context.Context, workDir string) error {
	slopCtx, cancel := context.WithTimeout(ctx, 60*time.Second) //nolint:mnd // matches qualityCtx timeout
	defer cancel()

	checker := quality.NewSlopChecker()

	ff, err := checker.Check(slopCtx, workDir)
	if err != nil {
		slog.Warn("quality gate: slop detection failed", "err", err)

		return nil // non-fatal; don't block on detection failures
	}

	if len(ff) == 0 {
		return nil
	}

	// Classify findings via hold-the-line to only gate on new slop.
	filtered := c.classifyFindings(ctx, ff)
	if len(filtered) == 0 {
		slog.Info("quality gate: all slop findings are pre-existing, passing")

		return nil
	}

	var msgs []string
	for _, f := range filtered {
		if f.File != "" {
			msgs = append(msgs, fmt.Sprintf("  %s:%d: [%s] %s", f.File, f.Line, f.Rule, f.Message))
		} else {
			msgs = append(msgs, fmt.Sprintf("  [%s] %s", f.Rule, f.Message))
		}
	}

	slog.Warn("quality gate: slop detected", "count", len(filtered))

	return fmt.Errorf("slop detection found %d issue(s):\n%s", len(filtered), strings.Join(msgs, "\n"))
}

// qualityCtx returns a context with the standard 60-second quality-gate timeout.
func qualityCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 60*time.Second)
}

// externalReviewCtx returns a context with a 5-minute timeout for external review tools.
// External review tools are significantly slower than local linters.
func externalReviewCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Minute)
}

// readPackageJSONScripts parses the "scripts" object from package.json.
func readPackageJSONScripts(workDir string) (map[string]string, error) {
	data, err := os.ReadFile(filepath.Join(workDir, "package.json"))
	if err != nil {
		return nil, err
	}

	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}

	if pkg.Scripts == nil {
		return map[string]string{}, nil
	}

	return pkg.Scripts, nil
}

// runNPMScript invokes npm run <script> with --if-present and returns
// (combined output, error). A non-zero exit code is returned as an error.
func runNPMScript(ctx context.Context, workDir, script string) (string, error) {
	cmd := exec.CommandContext(ctx, "npm", "run", script, "--if-present")
	cmd.Dir = workDir

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()

	return buf.String(), err
}

// collectPyFiles walks workDir recursively and returns .py file paths
// relative to workDir. Skips common virtual environment and cache directories
// (.venv, venv, __pycache__, .git, node_modules).
func collectPyFiles(workDir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(workDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip common virtual environment and cache directories
		if d.IsDir() {
			name := d.Name()
			if name == ".venv" || name == "venv" || name == "__pycache__" || name == ".git" || name == "node_modules" {
				return filepath.SkipDir
			}

			return nil
		}
		if strings.HasSuffix(d.Name(), ".py") {
			// Use relative path so cmd.Dir = workDir works correctly
			relPath, relErr := filepath.Rel(workDir, path)
			if relErr != nil {
				return relErr
			}
			files = append(files, relPath)
		}

		return nil
	})

	return files, err
}
