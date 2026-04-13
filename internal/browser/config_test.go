package browser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigPath_IsAbsolute(t *testing.T) {
	t.Parallel()

	got := ConfigPath()
	if !filepath.IsAbs(got) {
		t.Errorf("ConfigPath() = %q, want absolute path", got)
	}
}

func TestConfigPath_HasExpectedComponents(t *testing.T) {
	t.Parallel()

	got := ConfigPath()

	if !strings.Contains(got, ".valksor") {
		t.Errorf("ConfigPath() = %q, want path containing .valksor", got)
	}
	if !strings.Contains(got, "kvelmo") {
		t.Errorf("ConfigPath() = %q, want path containing kvelmo", got)
	}
	if filepath.Base(got) != "browser.json" {
		t.Errorf("ConfigPath() base = %q, want browser.json", filepath.Base(got))
	}
}

func TestPlaywrightConfigPath_IsAbsolute(t *testing.T) {
	t.Parallel()

	got := PlaywrightConfigPath()
	if !filepath.IsAbs(got) {
		t.Errorf("PlaywrightConfigPath() = %q, want absolute path", got)
	}
}

func TestPlaywrightConfigPath_EndsWithExpectedFilename(t *testing.T) {
	t.Parallel()

	got := PlaywrightConfigPath()
	if filepath.Base(got) != "cli.config.json" {
		t.Errorf("PlaywrightConfigPath() base = %q, want cli.config.json", filepath.Base(got))
	}
}

func TestLoadConfig_DefaultWhenFileDoesNotExist(t *testing.T) {
	// Use a temp HOME so the config file definitely doesn't exist.
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v, want nil", err)
	}
	if cfg == nil {
		t.Fatal("LoadConfig() returned nil config")
	}

	want := DefaultConfig()
	if cfg.Headless != want.Headless {
		t.Errorf("Headless = %v, want %v", cfg.Headless, want.Headless)
	}
	if cfg.Browser != want.Browser {
		t.Errorf("Browser = %q, want %q", cfg.Browser, want.Browser)
	}
	if cfg.Profile != want.Profile {
		t.Errorf("Profile = %q, want %q", cfg.Profile, want.Profile)
	}
	if cfg.Timeout != want.Timeout {
		t.Errorf("Timeout = %d, want %d", cfg.Timeout, want.Timeout)
	}
}

func TestLoadConfig_ReadsValidJSONFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Write a valid JSON config file to the expected path.
	cfgPath := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}

	tests := []struct {
		name     string
		headless bool
		browser  string
		profile  string
		timeout  int
	}{
		{
			name:     "chromium headless",
			headless: true,
			browser:  "chromium",
			profile:  "default",
			timeout:  30,
		},
		{
			name:     "firefox non-headless custom profile",
			headless: false,
			browser:  "firefox",
			profile:  "work",
			timeout:  60,
		},
		{
			name:     "webkit with long timeout",
			headless: true,
			browser:  "webkit",
			profile:  "staging",
			timeout:  120,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := &Config{
				Headless: tt.headless,
				Browser:  tt.browser,
				Profile:  tt.profile,
				Timeout:  tt.timeout,
			}
			data, err := json.Marshal(input)
			if err != nil {
				t.Fatalf("json.Marshal error = %v", err)
			}
			if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
				t.Fatalf("WriteFile error = %v", err)
			}

			got, err := LoadConfig()
			if err != nil {
				t.Fatalf("LoadConfig() error = %v", err)
			}
			if got.Headless != tt.headless {
				t.Errorf("Headless = %v, want %v", got.Headless, tt.headless)
			}
			if got.Browser != tt.browser {
				t.Errorf("Browser = %q, want %q", got.Browser, tt.browser)
			}
			if got.Profile != tt.profile {
				t.Errorf("Profile = %q, want %q", got.Profile, tt.profile)
			}
			if got.Timeout != tt.timeout {
				t.Errorf("Timeout = %d, want %d", got.Timeout, tt.timeout)
			}
		})
	}
}

func TestLoadConfig_InvalidJSONReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfgPath := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(cfgPath, []byte("not valid json {{{"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	_, err := LoadConfig()
	if err == nil {
		t.Error("LoadConfig() = nil error for invalid JSON, want error")
	}
}

func TestConfigSave_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	tests := []struct {
		name     string
		headless bool
		browser  string
		profile  string
		timeout  int
	}{
		{
			name:     "save and reload defaults",
			headless: true,
			browser:  "chromium",
			profile:  "default",
			timeout:  30,
		},
		{
			name:     "save and reload custom values",
			headless: false,
			browser:  "firefox",
			profile:  "ci",
			timeout:  90,
		},
		{
			name:     "save and reload webkit",
			headless: true,
			browser:  "webkit",
			profile:  "prod",
			timeout:  15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Headless: tt.headless,
				Browser:  tt.browser,
				Profile:  tt.profile,
				Timeout:  tt.timeout,
			}

			if err := cfg.Save(); err != nil {
				t.Fatalf("Config.Save() error = %v", err)
			}

			loaded, err := LoadConfig()
			if err != nil {
				t.Fatalf("LoadConfig() after Save() error = %v", err)
			}

			if loaded.Headless != tt.headless {
				t.Errorf("Headless = %v, want %v", loaded.Headless, tt.headless)
			}
			if loaded.Browser != tt.browser {
				t.Errorf("Browser = %q, want %q", loaded.Browser, tt.browser)
			}
			if loaded.Profile != tt.profile {
				t.Errorf("Profile = %q, want %q", loaded.Profile, tt.profile)
			}
			if loaded.Timeout != tt.timeout {
				t.Errorf("Timeout = %d, want %d", loaded.Timeout, tt.timeout)
			}
		})
	}
}

func TestWritePlaywrightConfig_JSONStructure(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	tests := []struct {
		name          string
		headless      bool
		browser       string
		timeout       int
		wantTimeoutMS float64
	}{
		{
			name:          "default headless chromium",
			headless:      true,
			browser:       "chromium",
			timeout:       30,
			wantTimeoutMS: 30000,
		},
		{
			name:          "non-headless firefox",
			headless:      false,
			browser:       "firefox",
			timeout:       60,
			wantTimeoutMS: 60000,
		},
		{
			name:          "webkit long timeout",
			headless:      true,
			browser:       "webkit",
			timeout:       120,
			wantTimeoutMS: 120000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Headless: tt.headless,
				Browser:  tt.browser,
				Profile:  "default",
				Timeout:  tt.timeout,
			}

			if err := cfg.WritePlaywrightConfig(); err != nil {
				t.Fatalf("WritePlaywrightConfig() error = %v", err)
			}

			data, err := os.ReadFile(PlaywrightConfigPath())
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}

			var parsed map[string]any
			if err := json.Unmarshal(data, &parsed); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}

			if parsed["headless"] != tt.headless {
				t.Errorf("headless = %v, want %v", parsed["headless"], tt.headless)
			}
			if parsed["browser"] != tt.browser {
				t.Errorf("browser = %v, want %v", parsed["browser"], tt.browser)
			}
			timeoutVal, ok := parsed["timeout"].(float64)
			if !ok {
				t.Fatalf("timeout is not a number: %T %v", parsed["timeout"], parsed["timeout"])
			}
			if timeoutVal != tt.wantTimeoutMS {
				t.Errorf("timeout = %v, want %v (milliseconds)", timeoutVal, tt.wantTimeoutMS)
			}
		})
	}
}
