package settings

import (
	"bufio"
	"os"
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
