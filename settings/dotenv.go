package settings

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// EnvFile is the name of the environment file.
	EnvFile = ".env"
)

// GlobalEnvPath returns the path to the global .env file.
func GlobalEnvPath() (string, error) {
	dir, err := GlobalDirPath()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, EnvFile), nil
}

// ProjectEnvPath returns the path to the project .env file.
func ProjectEnvPath(projectRoot string) string {
	return filepath.Join(ProjectDirPath(projectRoot), EnvFile)
}

// SaveEnvVar saves a single environment variable to the .env file at the given scope.
// If the variable already exists, its value is updated.
// If it doesn't exist, it's appended.
func SaveEnvVar(scope Scope, projectRoot, key, value string) error {
	var path string
	if scope == ScopeGlobal {
		var err error
		path, err = GlobalEnvPath()
		if err != nil {
			return err
		}
	} else {
		path = ProjectEnvPath(projectRoot)
	}

	return saveEnvVarToFile(path, key, value)
}

// saveEnvVarToFile updates or appends an env var in the specified file.
func saveEnvVarToFile(path, key, value string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create env dir: %w", err)
	}

	// Read existing content
	lines := []string{}
	found := false

	file, err := os.Open(path)
	if err == nil {
		defer func() { _ = file.Close() }()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			trimmed := strings.TrimSpace(line)

			// Check if this line sets our key
			if !strings.HasPrefix(trimmed, "#") {
				idx := strings.Index(trimmed, "=")
				if idx > 0 {
					lineKey := strings.TrimSpace(trimmed[:idx])
					if lineKey == key {
						// Replace this line
						lines = append(lines, fmt.Sprintf("%s=%s", key, value))
						found = true

						continue
					}
				}
			}
			lines = append(lines, line)
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("read env file: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("open env file: %w", err)
	}

	// Append if not found
	if !found {
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}

	// Write back
	content := strings.Join(lines, "\n")
	if len(lines) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write env file: %w", err)
	}

	return nil
}

// InjectEnvVars injects environment variables from an EnvMap into sensitive fields in settings.
// This replaces values loaded from .env files into the settings struct.
func InjectEnvVars(s *Settings, env EnvMap) {
	if token := env.Get("GITHUB_TOKEN"); token != "" {
		s.Providers.GitHub.Token = token
	}
	if token := env.Get("GITLAB_TOKEN"); token != "" {
		s.Providers.GitLab.Token = token
	}
	if token := env.Get("WRIKE_TOKEN"); token != "" {
		s.Providers.Wrike.Token = token
	}
	if token := env.Get("LINEAR_TOKEN"); token != "" {
		s.Providers.Linear.Token = token
	}

	// Agent API keys
	if key := env.Get("ANTHROPIC_API_KEY"); key != "" {
		s.Agent.Anthropic.APIKey = key
	}
	if key := env.Get("OPENAI_API_KEY"); key != "" {
		s.Agent.OpenAI.APIKey = key
	}
}

// MaskToken masks a token for display purposes.
// Shows first 4 and last 4 characters with *** in between.
func MaskToken(token string) string {
	if len(token) < 8 {
		if token == "" {
			return ""
		}

		return "***"
	}

	return token[:4] + "***" + token[len(token)-4:]
}

// IsMaskedToken returns true if the token appears to be masked.
func IsMaskedToken(token string) bool {
	return strings.Contains(token, "***")
}

// MaskSettings returns a copy of settings with sensitive fields masked.
// This should be used before sending settings to the client.
func MaskSettings(s *Settings) *Settings {
	if s == nil {
		return nil
	}

	// Create a shallow copy
	masked := *s

	// Mask provider tokens
	masked.Providers.GitHub.Token = MaskToken(s.Providers.GitHub.Token)
	masked.Providers.GitLab.Token = MaskToken(s.Providers.GitLab.Token)
	masked.Providers.Wrike.Token = MaskToken(s.Providers.Wrike.Token)
	masked.Providers.Linear.Token = MaskToken(s.Providers.Linear.Token)

	// Mask agent API keys
	masked.Agent.Anthropic.APIKey = MaskToken(s.Agent.Anthropic.APIKey)
	masked.Agent.OpenAI.APIKey = MaskToken(s.Agent.OpenAI.APIKey)

	return &masked
}
