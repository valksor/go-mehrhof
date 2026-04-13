package browser

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds browser configuration.
type Config struct {
	// Headless runs browser without visible window (default: true)
	Headless bool `json:"headless"`

	// Browser is the browser type: "chromium", "firefox", "webkit" (default: "chromium")
	Browser string `json:"browser"`

	// Profile is the name of the global auth profile to use (default: "default")
	Profile string `json:"profile"`

	// Timeout is the default timeout for browser operations in seconds (default: 30)
	Timeout int `json:"timeout"`
}

// DefaultConfig returns the default browser configuration.
func DefaultConfig() *Config {
	return &Config{
		Headless: true,
		Browser:  "chromium",
		Profile:  "default",
		Timeout:  30,
	}
}

// ConfigPath returns the path to the browser config file.
func ConfigPath() string {
	return filepath.Join(Paths(), "browser.json")
}

// LoadConfig loads the browser configuration from disk.
// Returns default config if file doesn't exist.
func LoadConfig() (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}

		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Save writes the configuration to disk.
func (c *Config) Save() error {
	dir := filepath.Dir(ConfigPath())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(ConfigPath(), data, 0o644)
}

// PlaywrightConfigPath returns the path to playwright-cli config file.
// This is passed to playwright-cli via --config flag.
func PlaywrightConfigPath() string {
	return filepath.Join(Paths(), ".playwright", "cli.config.json")
}

// WritePlaywrightConfig writes the playwright-cli configuration file.
func (c *Config) WritePlaywrightConfig() error {
	dir := filepath.Dir(PlaywrightConfigPath())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	// playwright-cli config format
	pwConfig := map[string]any{
		"headless": c.Headless,
		"browser":  c.Browser,
		"timeout":  c.Timeout * 1000, // convert to milliseconds
	}

	data, err := json.MarshalIndent(pwConfig, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(PlaywrightConfigPath(), data, 0o644)
}
