package quality

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/valksor/kvelmo/internal/findings"
)

// LinterChecker adapts a Linter into a Checker by running Lint() and
// converting the Report to findings. Linters that are unavailable or
// detect the wrong project type return an empty finding set (not an error).
type LinterChecker struct {
	linter Linter
}

// NewLinterChecker wraps an existing Linter as a Checker.
func NewLinterChecker(l Linter) *LinterChecker {
	return &LinterChecker{linter: l}
}

// Name returns the underlying linter's name.
func (lc *LinterChecker) Name() string { return lc.linter.Name() }

// Check runs the linter and converts its report to findings.
func (lc *LinterChecker) Check(ctx context.Context, workDir string) ([]findings.Finding, error) {
	report, err := lc.linter.Lint(ctx, workDir)
	if err != nil {
		return nil, err
	}

	if report.Coverage == CoverageSkipped || report.Coverage == CoverageUnavailable {
		return nil, nil
	}

	return report.ToFindings(), nil
}

// NodeScriptsChecker runs npm scripts (lint, typecheck) as a quality check.
// Only runs scripts that exist in package.json.
type NodeScriptsChecker struct {
	scripts []string // scripts to run, e.g. []string{"lint", "typecheck"}
}

// NewNodeScriptsChecker creates a checker that runs the given npm scripts.
func NewNodeScriptsChecker(scripts ...string) *NodeScriptsChecker {
	if len(scripts) == 0 {
		scripts = []string{"lint", "typecheck"}
	}

	return &NodeScriptsChecker{scripts: scripts}
}

// Name returns the checker name.
func (n *NodeScriptsChecker) Name() string { return "node-scripts" }

// Check runs npm/bun scripts and returns findings from their output.
func (n *NodeScriptsChecker) Check(ctx context.Context, workDir string) ([]findings.Finding, error) {
	pkgScripts, err := readNodeScripts(workDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // no package.json → skip silently
		}

		return nil, fmt.Errorf("read package.json: %w", err)
	}

	runner := detectNodeRunner(workDir)
	if _, err := exec.LookPath(runner); err != nil {
		return nil, nil //nolint:nilerr // runner not installed → skip silently
	}

	var allFindings []findings.Finding
	for _, script := range n.scripts {
		if _, ok := pkgScripts[script]; !ok {
			continue
		}

		cmd := exec.CommandContext(ctx, runner, "run", script)
		cmd.Dir = workDir

		var buf bytes.Buffer
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		if runErr := cmd.Run(); runErr != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			// Parse output into per-line findings for hold-the-line compatibility.
			parsed := parseLineFindings("node-scripts-"+script, buf.String())
			if len(parsed) > 0 {
				allFindings = append(allFindings, parsed...)
			} else {
				// Fallback: single finding if output couldn't be parsed into lines.
				allFindings = append(allFindings, findings.Finding{
					ID:       "node-scripts-" + script,
					Severity: findings.SeverityHigh,
					Category: findings.CategoryQuality,
					Rule:     script,
					Message:  fmt.Sprintf("%s run %s failed:\n%s", runner, script, buf.String()),
					Source:   "node-scripts-" + script,
				})
			}
		}
	}

	return allFindings, nil
}

// readNodeScripts parses the "scripts" object from package.json.
// Rejects files larger than 1 MB to prevent memory exhaustion from malicious worktrees.
func readNodeScripts(workDir string) (map[string]string, error) {
	pkgPath := filepath.Join(workDir, "package.json")
	info, err := os.Stat(pkgPath)
	if err != nil {
		return nil, err
	}
	const maxPackageJSONSize = 1 << 20 // 1 MB
	if info.Size() > maxPackageJSONSize {
		return nil, fmt.Errorf("package.json too large (%d bytes, max %d)", info.Size(), maxPackageJSONSize)
	}
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return nil, err
	}

	// Reuse the same minimal JSON parse approach.
	type pkgJSON struct {
		Scripts map[string]string `json:"scripts"`
	}
	var pkg pkgJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}

	if pkg.Scripts == nil {
		return map[string]string{}, nil
	}

	return pkg.Scripts, nil
}

// PythonChecker runs ruff or py_compile for Python projects.
type PythonChecker struct{}

// NewPythonChecker creates a new Python quality checker.
func NewPythonChecker() *PythonChecker { return &PythonChecker{} }

// Name returns the checker name.
func (p *PythonChecker) Name() string { return "python" }

// Check runs ruff (preferred) or py_compile on changed .py files.
// Callers should verify this is a Python project before calling (e.g. via DetectCheckers).
func (p *PythonChecker) Check(ctx context.Context, workDir string) ([]findings.Finding, error) {
	if ruffPath, err := exec.LookPath("ruff"); err == nil {
		return p.checkRuff(ctx, workDir, ruffPath)
	}

	return p.checkPyCompile(ctx, workDir)
}

func (p *PythonChecker) checkRuff(ctx context.Context, workDir, ruffPath string) ([]findings.Finding, error) {
	cmd := exec.CommandContext(ctx, ruffPath, "check", ".", "--output-format=text")
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	return parseLineFindings("ruff", string(output)), nil
}

func (p *PythonChecker) checkPyCompile(ctx context.Context, workDir string) ([]findings.Finding, error) {
	if _, err := exec.LookPath("python3"); err != nil {
		return nil, nil //nolint:nilerr // python3 not installed → skip silently
	}

	pyFiles, err := collectPythonFiles(workDir)
	if err != nil {
		return nil, fmt.Errorf("collect python files: %w", err)
	}
	if len(pyFiles) == 0 {
		return nil, nil
	}

	const batchSize = 100

	var allFindings []findings.Finding
	for i := 0; i < len(pyFiles); i += batchSize {
		end := min(i+batchSize, len(pyFiles))
		batch := pyFiles[i:end]

		args := append([]string{"-m", "py_compile"}, batch...)
		cmd := exec.CommandContext(ctx, "python3", args...)
		cmd.Dir = workDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			allFindings = append(allFindings, parseLineFindings("py_compile", string(output))...)
		}
	}

	return allFindings, nil
}

// parseLineFindings converts multi-line tool output into findings.
// IDs are derived from a hash of source+message to remain stable across runs
// and avoid collisions when called multiple times with the same source (e.g.,
// py_compile batches).
func parseLineFindings(source, output string) []findings.Finding {
	var result []findings.Finding
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		h := sha256.Sum256([]byte(source + "\x00" + line))
		result = append(result, findings.Finding{
			ID:       source + "-" + hex.EncodeToString(h[:8]),
			Severity: findings.SeverityHigh,
			Category: findings.CategoryQuality,
			Rule:     source,
			Message:  line,
			Source:   source,
		})
	}

	return result
}

// collectPythonFiles walks workDir and returns .py file paths relative to workDir.
func collectPythonFiles(workDir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(workDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip symlinks to prevent traversal outside the project tree.
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".venv" || name == "venv" || name == "__pycache__" || name == ".git" || name == "node_modules" {
				return filepath.SkipDir
			}

			return nil
		}
		if strings.HasSuffix(d.Name(), ".py") {
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

// DetectCheckers returns the appropriate checkers for the given work directory
// based on detected project type. Unlike the old mutually-exclusive approach,
// this returns checkers for ALL detected project types (e.g., a repo with both
// go.mod and package.json gets both Go and Node checkers). This is intentional
// for monorepo and polyglot projects. This is the recommended way to build a
// checker set for RunGates.
func DetectCheckers(workDir string) []Checker {
	var checkers []Checker

	if _, err := os.Stat(filepath.Join(workDir, "go.mod")); err == nil {
		checkers = append(checkers, NewLinterChecker(NewGoVet()))
	}

	if _, err := os.Stat(filepath.Join(workDir, "package.json")); err == nil {
		checkers = append(checkers, NewNodeScriptsChecker("lint", "typecheck"))
	}

	if hasPythonProject(workDir) {
		checkers = append(checkers, NewPythonChecker())
	}

	// Always include slop detection.
	checkers = append(checkers, NewSlopChecker())

	return checkers
}

// detectNodeRunner returns "bun" if a bun lockfile exists, otherwise "npm".
func detectNodeRunner(workDir string) string {
	for _, lock := range []string{"bun.lockb", "bun.lock"} {
		if _, err := os.Stat(filepath.Join(workDir, lock)); err == nil {
			return "bun"
		}
	}

	return "npm"
}

func hasPythonProject(workDir string) bool {
	for _, marker := range []string{"setup.py", "pyproject.toml", "requirements.txt"} {
		if _, err := os.Stat(filepath.Join(workDir, marker)); err == nil {
			return true
		}
	}

	return false
}
