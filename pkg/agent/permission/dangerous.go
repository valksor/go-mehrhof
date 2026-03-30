// Package permission provides dangerous operation detection for agent tools.
package permission

import (
	"path/filepath"
	"regexp"
	"strings"
)

// DangerLevel indicates how risky an operation is.
type DangerLevel int

const (
	// Safe operations can proceed without concern.
	Safe DangerLevel = iota
	// Caution operations might be dangerous depending on context.
	Caution
	// Dangerous operations are almost always destructive.
	Dangerous
)

func (d DangerLevel) String() string {
	switch d {
	case Safe:
		return "safe"
	case Caution:
		return "caution"
	case Dangerous:
		return "dangerous"
	default:
		return "unknown"
	}
}

// Result holds the detection result.
type Result struct {
	Level  DangerLevel
	Reason string
}

// DetectDanger analyzes a tool invocation for dangerous operations.
// Returns the danger level and reason if not safe.
func DetectDanger(tool string, input map[string]any) Result {
	switch strings.ToLower(tool) {
	case "bash":
		return detectBashDanger(input)
	case "write":
		return detectWriteDanger(input)
	case "edit":
		return detectEditDanger(input)
	default:
		return Result{Level: Safe}
	}
}

func detectBashDanger(input map[string]any) Result {
	cmd, ok := input["command"].(string)
	if !ok {
		return Result{Level: Safe}
	}

	// Dangerous: system destruction commands
	if matched, reason := matchDangerousCommand(cmd); matched {
		return Result{Level: Dangerous, Reason: reason}
	}

	// Caution: potentially risky but context-dependent
	if matched, reason := matchCautionCommand(cmd); matched {
		return Result{Level: Caution, Reason: reason}
	}

	return Result{Level: Safe}
}

func detectWriteDanger(input map[string]any) Result {
	path, ok := input["file_path"].(string)
	if !ok {
		return Result{Level: Safe}
	}

	return detectPathDanger(path)
}

func detectEditDanger(input map[string]any) Result {
	path, ok := input["file_path"].(string)
	if !ok {
		return Result{Level: Safe}
	}

	return detectPathDanger(path)
}

func detectPathDanger(path string) Result {
	// Clean and normalize the path for consistent matching
	cleanPath := filepath.Clean(path)
	lowerPath := strings.ToLower(cleanPath)

	// Dangerous: system files (anchored matching to avoid false positives)
	for _, pattern := range dangerousPaths {
		if matchDangerousPath(lowerPath, pattern) {
			return Result{
				Level:  Dangerous,
				Reason: "Modifies system file: " + pattern,
			}
		}
	}

	// Caution: sensitive files
	baseName := strings.ToLower(filepath.Base(cleanPath))
	for _, pattern := range cautionPaths {
		// Special handling for .env - match as exact filename or with extension
		if pattern == ".env" {
			if baseName == ".env" || strings.HasPrefix(baseName, ".env.") {
				return Result{
					Level:  Caution,
					Reason: "Modifies sensitive file: " + pattern,
				}
			}

			continue
		}
		// Other patterns: substring match is acceptable for keywords like "password", "credentials"
		if strings.Contains(lowerPath, pattern) {
			return Result{
				Level:  Caution,
				Reason: "Modifies sensitive file: " + pattern,
			}
		}
	}

	return Result{Level: Safe}
}

// matchDangerousPath checks if path matches a dangerous pattern.
// Hidden directory patterns (/.ssh/, /.gnupg/) match anywhere in the path.
// Absolute patterns (/etc/, /proc/, /dev/) require anchored matching.
func matchDangerousPath(path, pattern string) bool {
	// Hidden directory patterns (like /.ssh/, /.gnupg/) can appear anywhere
	// These are user directory patterns that could be in any home directory
	if strings.HasPrefix(pattern, "/.") {
		if trimmed, found := strings.CutSuffix(pattern, "/"); found {
			// Match if pattern appears anywhere in path
			return strings.Contains(path, pattern) ||
				strings.HasSuffix(path, trimmed)
		}
	}

	// For directory patterns (ending with /), use anchored prefix matching
	if dirPattern, found := strings.CutSuffix(pattern, "/"); found {
		return path == dirPattern || strings.HasPrefix(path, pattern)
	}

	// For file patterns, require exact match or path within
	return path == pattern || strings.HasPrefix(path, pattern+"/")
}

var dangerousPaths = []string{
	"/etc/passwd",
	"/etc/shadow",
	"/etc/sudoers",
	"/proc/",
	"/sys/",
	"/dev/",
	"/.ssh/",
	"/.gnupg/",
}

var cautionPaths = []string{
	".env",
	"credentials",
	"secrets",
	".secret",
	"password",
	"api_key",
	"apikey",
	"private_key",
	"id_rsa",
	"id_ed25519",
}

// Dangerous commands - almost always destructive.
var dangerousPatterns = []struct {
	pattern *regexp.Regexp
	reason  string
}{
	// rm -rf with dangerous targets (root, critical system dirs, home, or wildcard)
	// Using case-insensitive mode ((?i)) to catch uppercase flags (-RF) and paths (/ROOT)
	{regexp.MustCompile(`(?i)\brm\s+-[a-z]*r[a-z]*f[a-z]*\s+(/\s*$|/(home|usr|var|etc|bin|sbin|lib|opt|root)(/|$)|~|\*)`), "Recursive delete with dangerous target"},
	{regexp.MustCompile(`(?i)\brm\s+-[a-z]*f[a-z]*r[a-z]*\s+(/\s*$|/(home|usr|var|etc|bin|sbin|lib|opt|root)(/|$)|~|\*)`), "Recursive delete with dangerous target"},
	{regexp.MustCompile(`(?i)\brm\s+--recursive\s+(/\s*$|/(home|usr|var|etc|bin|sbin|lib|opt|root)(/|$)|~|\*)`), "Recursive delete with dangerous target"},
	{regexp.MustCompile(`\bdd\s+.*of=/dev/`), "Direct disk write"},
	{regexp.MustCompile(`\bmkfs\b`), "Filesystem format"},
	{regexp.MustCompile(`\bfdisk\b`), "Partition modification"},
	{regexp.MustCompile(`\bparted\b`), "Partition modification"},
	{regexp.MustCompile(`\breboot\b`), "System reboot"},
	{regexp.MustCompile(`\bshutdown\b`), "System shutdown"},
	{regexp.MustCompile(`\bhalt\b`), "System halt"},
	{regexp.MustCompile(`\bpoweroff\b`), "System poweroff"},
	{regexp.MustCompile(`\binit\s+0\b`), "System shutdown via init"},
	{regexp.MustCompile(`\bsystemctl\s+(poweroff|reboot|halt)\b`), "System power control"},
	// Fork bomb: :(){:|:&};:
	{regexp.MustCompile(`:\s*\(\s*\)\s*\{`), "Fork bomb"},
	{regexp.MustCompile(`>/dev/sd[a-z]`), "Direct disk overwrite"},
	{regexp.MustCompile(`\bchmod\s+777\s+/`), "World-writable root"},
	{regexp.MustCompile(`\bchown\s+.*\s+/`), "Change root ownership"},
}

// Caution commands - risky but sometimes legitimate.
var cautionPatterns = []struct {
	pattern *regexp.Regexp
	reason  string
}{
	{regexp.MustCompile(`\brm\s+-[a-z]*r`), "Recursive delete"},
	{regexp.MustCompile(`\brm\s+--recursive`), "Recursive delete"},
	{regexp.MustCompile(`\bgit\s+push\s+.*--force`), "Force push may overwrite remote history"},
	{regexp.MustCompile(`\bgit\s+push\s+-f\b`), "Force push may overwrite remote history"},
	{regexp.MustCompile(`\bgit\s+reset\s+--hard`), "Hard reset discards uncommitted changes"},
	{regexp.MustCompile(`\bgit\s+clean\s+-[a-z]*f`), "Git clean removes untracked files"},
	{regexp.MustCompile(`\bgit\s+checkout\s+--\s+\.`), "Checkout discards all local changes"},
	{regexp.MustCompile(`\bkill\s+-9\b`), "Force kill process"},
	{regexp.MustCompile(`\bkill\s+-KILL\b`), "Force kill process"},
	{regexp.MustCompile(`\bkillall\b`), "Kill all matching processes"},
	{regexp.MustCompile(`\bpkill\b`), "Kill processes by pattern"},
	{regexp.MustCompile(`\bchmod\s+[0-7]?[0-7]{2}[1-7]\b`), "World-accessible permissions"},
	{regexp.MustCompile(`\bsudo\b`), "Elevated privileges"},
	{regexp.MustCompile(`\bdoas\b`), "Elevated privileges"},
	{regexp.MustCompile(`\bsu\s+-?\s*\w*$`), "Switch user"},
	{regexp.MustCompile(`\bcurl\s+.*\|\s*(ba)?sh`), "Pipe to shell"},
	{regexp.MustCompile(`\bwget\s+.*\|\s*(ba)?sh`), "Pipe to shell"},
	{regexp.MustCompile(`\bnpm\s+publish\b`), "Publish to npm"},
	{regexp.MustCompile(`\bdocker\s+push\b`), "Push docker image"},
	{regexp.MustCompile(`\bdocker\s+system\s+prune\b`), "Docker system cleanup"},
}

func matchDangerousCommand(cmd string) (bool, string) {
	for _, p := range dangerousPatterns {
		if p.pattern.MatchString(cmd) {
			return true, p.reason
		}
	}

	return false, ""
}

func matchCautionCommand(cmd string) (bool, string) {
	for _, p := range cautionPatterns {
		if p.pattern.MatchString(cmd) {
			return true, p.reason
		}
	}

	return false, ""
}

// EnforceEnvironment applies environment-specific restrictions to danger levels.
// In prod: Dangerous operations remain Dangerous with a production label, Caution operations are elevated to Dangerous.
// In staging: Dangerous operations are labeled with a staging warning.
// In dev: No additional restrictions.
func EnforceEnvironment(env string, result Result) Result {
	switch env {
	case "prod":
		switch result.Level {
		case Safe:
			// No restrictions for safe operations
		case Dangerous:
			return Result{Level: Dangerous, Reason: result.Reason + " (blocked in production)"}
		case Caution:
			return Result{Level: Dangerous, Reason: result.Reason + " (elevated in production)"}
		}
	case "staging":
		if result.Level == Dangerous {
			return Result{Level: Dangerous, Reason: result.Reason + " (dangerous in staging)"}
		}
	}

	return result
}
