package apiagent

import (
	"net/http"
	"time"
)

// APIConfig holds configuration shared across all API-based agents.
type APIConfig struct {
	// API connection
	APIKey  string // API key (empty for keyless backends like Ollama)
	BaseURL string // API base URL
	Model   string // Model identifier

	// HTTP settings
	HTTPClient *http.Client  // Custom HTTP client (nil uses default with timeout)
	Timeout    time.Duration // Request timeout (default: 10m)

	// Execution settings
	WorkDir     string        // Working directory for tool execution
	MaxTurns    int           // Maximum conversation turns to prevent runaway loops (default: 50)
	MaxTokens   int           // Maximum tokens per response (0 = provider default)
	ExecTimeout time.Duration // Per-tool execution timeout (default: 2m)

	// Budget limits
	TokenBudget int64 // Max tokens for this execution (0 = unlimited)

	// System prompt override (empty uses default)
	SystemPrompt string
}

// DefaultAPIConfig returns sensible defaults.
func DefaultAPIConfig() APIConfig {
	return APIConfig{
		Timeout:     10 * time.Minute,
		MaxTurns:    50,
		ExecTimeout: 2 * time.Minute,
	}
}

// httpClient returns the configured HTTP client or a default one.
func (c *APIConfig) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}

	return &http.Client{
		Timeout: c.Timeout,
	}
}
