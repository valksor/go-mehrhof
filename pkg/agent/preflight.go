package agent

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// CheckStatus represents the result of a single preflight check.
type CheckStatus string

const (
	CheckPassed  CheckStatus = "passed"
	CheckFailed  CheckStatus = "failed"
	CheckWarning CheckStatus = "warning"
)

// CheckResult holds the result of a single preflight check.
type CheckResult struct {
	Name   string      `json:"name"`
	Status CheckStatus `json:"status"`
	Detail string      `json:"detail,omitempty"` // e.g., "2.43.0" or error message
	Fix    string      `json:"fix,omitempty"`    // Actionable fix instruction
}

// PreflightResult holds the results of all preflight checks.
type PreflightResult struct {
	Checks         []CheckResult `json:"checks"`
	AgentAvailable bool          `json:"agent_available"`
	SimulationMode bool          `json:"simulation_mode"`
}

// HasIssues returns true if any check failed.
func (r *PreflightResult) HasIssues() bool {
	for _, c := range r.Checks {
		if c.Status == CheckFailed {
			return true
		}
	}

	return false
}

// RunPreflight performs all pre-flight checks.
func RunPreflight() *PreflightResult {
	result := &PreflightResult{}

	// Check git
	if version, err := getVersion("git", "--version"); err == nil {
		result.Checks = append(result.Checks, CheckResult{
			Name:   "git",
			Status: CheckPassed,
			Detail: version,
		})
	} else {
		result.Checks = append(result.Checks, CheckResult{
			Name:   "git",
			Status: CheckFailed,
			Detail: "not found",
			Fix:    "Install git: https://git-scm.com/downloads",
		})
	}

	// Check Claude CLI
	claudeOK := false
	if version, err := getVersion("claude", "--version"); err == nil {
		result.Checks = append(result.Checks, CheckResult{
			Name:   "claude",
			Status: CheckPassed,
			Detail: version,
		})
		claudeOK = true

		// Check Claude authentication
		if authErr := checkClaudeAuth(); authErr != nil {
			result.Checks = append(result.Checks, CheckResult{
				Name:   "claude-auth",
				Status: CheckWarning,
				Detail: authErr.Error(),
				Fix:    "Authenticate Claude: claude login",
			})
		} else {
			result.Checks = append(result.Checks, CheckResult{
				Name:   "claude-auth",
				Status: CheckPassed,
				Detail: "authenticated",
			})
		}
	} else {
		result.Checks = append(result.Checks, CheckResult{
			Name:   "claude",
			Status: CheckFailed,
			Detail: "not found",
			Fix:    "Install Claude CLI: https://docs.anthropic.com/en/docs/claude-code/getting-started",
		})
	}

	// Check Codex CLI
	codexOK := false
	if version, err := getVersion("codex", "--version"); err == nil {
		result.Checks = append(result.Checks, CheckResult{
			Name:   "codex",
			Status: CheckPassed,
			Detail: version,
		})
		codexOK = true
	} else {
		status := CheckFailed
		fix := "Install at least one AI agent CLI (Claude or Codex)"
		if claudeOK {
			status = CheckWarning // Not critical if Claude is available
			fix = ""
		}
		result.Checks = append(result.Checks, CheckResult{
			Name:   "codex",
			Status: status,
			Detail: "not found",
			Fix:    fix,
		})
	}

	result.AgentAvailable = claudeOK || codexOK
	result.SimulationMode = !result.AgentAvailable

	return result
}

// PrintPreflight prints preflight results to stdout with symbols.
func PrintPreflight(r *PreflightResult) {
	fmt.Println()
	fmt.Println("  Pre-flight checks")
	fmt.Println("  ─────────────────────────────────────")
	for _, c := range r.Checks {
		symbol := "✓"

		switch c.Status {
		case CheckPassed:
			// default value is fine
		case CheckFailed:
			symbol = "✗"
		case CheckWarning:
			symbol = "⚠"
		}
		detail := ""
		if c.Detail != "" {
			detail = " (" + c.Detail + ")"
		}
		fmt.Printf("  %-14s %s %s%s\n", c.Name+":", symbol, c.Status, detail)
		if c.Fix != "" {
			fmt.Printf("  %-14s   → %s\n", "", c.Fix)
		}
	}
	fmt.Println()
}

// checkClaudeAuth verifies Claude CLI authentication by running a quick check.
func checkClaudeAuth() error {
	path, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude not found: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check Claude Code CLI login status (not API — kvelmo spawns claude as subprocess).
	cmd := exec.CommandContext(ctx, path, "auth", "status")
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := strings.TrimSpace(string(output))
		if strings.Contains(outputStr, "auth") || strings.Contains(outputStr, "login") ||
			strings.Contains(outputStr, "401") || strings.Contains(outputStr, "403") ||
			strings.Contains(outputStr, "token") || strings.Contains(outputStr, "credential") {
			return errors.New("not authenticated")
		}

		// Other errors (network, etc.) — treat as warning but not auth failure
		return fmt.Errorf("auth check failed: %s", outputStr)
	}

	return nil
}

// getVersion runs a command and returns a cleaned version string.
func getVersion(name string, args ...string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, args...)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	version := strings.TrimSpace(string(output))
	if idx := strings.Index(version, "\n"); idx > 0 {
		version = version[:idx]
	}

	return cleanVersion(version), nil
}

// cleanVersion extracts just the version number from output.
func cleanVersion(s string) string {
	s = strings.TrimSpace(s)
	prefixes := []string{"git version ", "claude ", "codex "}
	for _, p := range prefixes {
		if strings.HasPrefix(strings.ToLower(s), strings.ToLower(p)) {
			s = strings.TrimSpace(s[len(p):])

			break
		}
	}
	if idx := strings.Index(s, " "); idx > 0 {
		candidate := s[:idx]
		if len(candidate) > 0 && candidate[0] >= '0' && candidate[0] <= '9' {
			return candidate
		}
	}
	if len(s) > 20 {
		return s[:20] + "..."
	}

	return s
}
