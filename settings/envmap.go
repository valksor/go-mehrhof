package settings

import (
	"bufio"
	"maps"
	"os"
	"sort"
	"strings"
)

// EnvMap holds environment variables loaded from .env files.
// Unlike os.Setenv, this keeps values in-memory without polluting the process environment.
type EnvMap map[string]string

// Get returns the value for a key, or empty string if not found.
func (m EnvMap) Get(key string) string {
	if m == nil {
		return ""
	}

	return m[key]
}

// passthroughEnvKeys is the minimal set of host environment variables a spawned
// subprocess genuinely needs in order to run: HOME (the agent's login/config
// lookup, e.g. ~/.claude), PATH (locating the binary's runtime dependencies),
// TERM (PTY/TUI rendering), plus a few locale/identity/temp vars. Every other
// host variable is dropped, so secrets (provider tokens, cloud credentials, CI
// variables) never leak into agents.
var passthroughEnvKeys = []string{
	"HOME", "PATH", "TERM",
	"LANG", "LC_ALL", "LC_CTYPE",
	"TMPDIR", "TZ",
	"USER", "LOGNAME", "SHELL",
}

// ProcessEnv builds the environment for a spawned subprocess. It combines, in
// increasing precedence: a minimal allowlisted host base (see passthroughEnvKeys),
// kvelmo's config-dir .env (global + project, via LoadEnvMap), and the supplied
// per-process overrides — formatted as a sorted "KEY=VALUE" slice for
// exec.Cmd.Env. The host os.Environ() is NOT inherited wholesale: only the
// allowlisted keys are read, so host secrets never reach agents while HOME/PATH
// and friends stay available for the binary to run. A config-dir or override
// value wins over the host passthrough, so HOME/PATH can be pinned in config.
func ProcessEnv(projectRoot string, overrides map[string]string) []string {
	merged := make(EnvMap)
	// Minimal host base (lowest precedence): read only the allowlisted keys.
	for _, k := range passthroughEnvKeys {
		if v, ok := os.LookupEnv(k); ok {
			merged[k] = v
		}
	}
	// Config-dir .env overrides the host base.
	if env, err := LoadEnvMap(projectRoot); err == nil {
		maps.Copy(merged, env)
	}
	// Per-process overrides win over everything.
	maps.Copy(merged, overrides)

	out := make([]string, 0, len(merged))
	for k, v := range merged {
		out = append(out, k+"="+v)
	}
	sort.Strings(out)

	return out
}

// LoadEnvMap loads environment variables from global and project .env files.
// Project .env values override global .env values.
// Returns an empty map (not nil) if no .env files exist.
func LoadEnvMap(projectRoot string) (EnvMap, error) {
	env := make(EnvMap)

	// Load global .env first
	globalPath, err := GlobalEnvPath()
	if err != nil {
		return nil, err
	}
	if err := loadEnvFileInto(globalPath, env); err != nil {
		return nil, err
	}

	// Load project .env (overrides global)
	if projectRoot != "" {
		projectPath := ProjectEnvPath(projectRoot)
		if err := loadEnvFileInto(projectPath, env); err != nil {
			return nil, err
		}
	}

	return env, nil
}

// loadEnvFileInto parses a .env file and adds values to the map.
// Does nothing if the file doesn't exist.
func loadEnvFileInto(path string, env EnvMap) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=value
		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])

		// Remove surrounding quotes if present
		if len(value) >= 2 {
			if value[0] == '"' && value[len(value)-1] == '"' {
				value = value[1 : len(value)-1]
				// Process escape sequences in double-quoted values
				value = strings.ReplaceAll(value, `\n`, "\n")
				value = strings.ReplaceAll(value, `\t`, "\t")
				value = strings.ReplaceAll(value, `\\`, `\`)
			} else if value[0] == '\'' && value[len(value)-1] == '\'' {
				// Single-quoted values are literal (no escape processing)
				value = value[1 : len(value)-1]
			}
		}

		env[key] = value
	}

	return scanner.Err()
}
