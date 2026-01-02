// Package token provides shared token resolution utilities for providers.
package token

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// DefaultEnvSuffix is the default suffix for environment variable names.
const DefaultEnvSuffix = "_TOKEN"

// ErrNoToken is returned when no token can be resolved.
var ErrNoToken = errors.New("no token found")

// ResolverConfig defines the token sources for a provider.
type ResolverConfig struct {
	// ProviderName is used to construct the MEHR_{PROVIDER_NAME}_TOKEN env var.
	// Should be in uppercase (e.g., "GITHUB", "NOTION", "JIRA").
	ProviderName string

	// DefaultEnvVars are fallback environment variables to check.
	// Common patterns: ["GITHUB_TOKEN"], ["NOTION_TOKEN"], ["JIRA_TOKEN"].
	DefaultEnvVars []string

	// ConfigToken is the value from the configuration file (config.yaml).
	ConfigToken string

	// OptionalCLIFallback is an optional function to get a token from a CLI tool.
	// Examples: gh CLI auth token, az account get-access-token, etc.
	OptionalCLIFallback func() string
}

// ResolveToken resolves a provider token from multiple sources.
// Priority order:
//  1. MEHR_{PROVIDER_NAME}_TOKEN env var
//  2. DefaultEnvVars (e.g., GITHUB_TOKEN)
//  3. ConfigToken (from config.yaml)
//  4. OptionalCLIFallback result
//
// Returns ErrNoToken if no token is found.
func ResolveToken(cfg ResolverConfig) (string, error) {
	// 1. Check MEHR_{PROVIDER_NAME}_TOKEN
	if cfg.ProviderName != "" {
		mehrKey := "MEHR_" + cfg.ProviderName + DefaultEnvSuffix
		if token := os.Getenv(mehrKey); token != "" {
			return token, nil
		}
	}

	// 2. Check default env vars
	for _, envVar := range cfg.DefaultEnvVars {
		if token := os.Getenv(envVar); token != "" {
			return token, nil
		}
	}

	// 3. Check config token
	if cfg.ConfigToken != "" {
		return cfg.ConfigToken, nil
	}

	// 4. Try CLI fallback
	if cfg.OptionalCLIFallback != nil {
		if token := cfg.OptionalCLIFallback(); token != "" {
			return token, nil
		}
	}

	return "", ErrNoToken
}

// MustResolveToken is like ResolveToken but panics on error.
// Useful for package-level initialization or when token is required.
func MustResolveToken(cfg ResolverConfig) string {
	token, err := ResolveToken(cfg)
	if err != nil {
		provider := cfg.ProviderName
		if provider == "" {
			provider = "provider"
		}
		panic(fmt.Sprintf("%s: %s (set %s_TOKEN environment variable)",
			strings.ToLower(provider), ErrNoToken, strings.ToUpper(provider)))
	}

	return token
}

// Config builds a ResolverConfig with just the config token.
// Useful when you want to start with a base config and modify it.
func Config(providerName, configToken string) ResolverConfig {
	return ResolverConfig{
		ProviderName: providerName,
		ConfigToken:  configToken,
	}
}

// WithCLIFallback adds a CLI fallback function to the config.
func (c ResolverConfig) WithCLIFallback(fn func() string) ResolverConfig {
	c.OptionalCLIFallback = fn

	return c
}

// WithEnvVars adds environment variables to check.
func (c ResolverConfig) WithEnvVars(envVars ...string) ResolverConfig {
	c.DefaultEnvVars = append(c.DefaultEnvVars, envVars...)

	return c
}
