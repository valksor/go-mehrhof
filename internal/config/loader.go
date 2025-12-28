package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/valksor/go-envconfig"
)

// DefaultConfigPaths returns paths to check for .env files
func DefaultConfigPaths() []string {
	home, _ := os.UserHomeDir()
	return []string{
		filepath.Join(home, ".mehrhof", ".env"), // Global user config (lowest priority)
		".env",                                  // Project directory
		".env.local",                            // Local overrides (highest priority)
	}
}

// Load reads configuration from environment and .env files
func Load(ctx context.Context) (*Config, error) {
	// Start with defaults
	cfg := NewDefault()

	// Collect env maps from files (earlier = lower priority)
	envMaps := []map[string]string{}

	for _, path := range DefaultConfigPaths() {
		data, err := os.ReadFile(path)
		if err != nil {
			continue // File doesn't exist or can't be read
		}
		envMaps = append(envMaps, envconfig.ReadDotenvBytes(data))
	}

	// Add system environment (highest priority)
	envMaps = append(envMaps, envconfig.GetEnvs())

	// Merge all sources
	merged := envconfig.MergeEnvMaps(envMaps...)

	// Fill config struct from merged environment
	if err := envconfig.FillStructFromEnv("mehr", reflect.ValueOf(cfg).Elem(), merged); err != nil {
		return nil, fmt.Errorf("fill config: %w", err)
	}

	// Validate configuration
	validator := envconfig.NewValidator()
	if err := validator.ValidateStruct(cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	// Custom validation
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate performs additional validation beyond struct tags
func (c *Config) Validate() error {
	// Validate agent selection
	switch c.Agent.Default {
	case "claude":
		// OK
	default:
		return fmt.Errorf("invalid agent: %s (must be claude)", c.Agent.Default)
	}

	// Validate UI format
	switch c.UI.Format {
	case "text", "json":
		// OK
	default:
		return fmt.Errorf("invalid UI format: %s (must be text or json)", c.UI.Format)
	}

	// Validate progress style
	switch c.UI.Progress {
	case "spinner", "dots", "none":
		// OK
	default:
		return fmt.Errorf("invalid progress style: %s (must be spinner, dots, or none)", c.UI.Progress)
	}

	return nil
}
