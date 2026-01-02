package agent

import "os"

// ResolveEnvReferences expands environment variable references in a map of values.
// It supports both ${VAR} and $VAR syntax using os.ExpandEnv.
// If a referenced variable is not set, it will be replaced with an empty string.
func ResolveEnvReferences(env map[string]string) map[string]string {
	if env == nil {
		return nil
	}

	result := make(map[string]string, len(env))
	for k, v := range env {
		result[k] = os.ExpandEnv(v)
	}

	return result
}
