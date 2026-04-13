package browser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNodeModulesDir(t *testing.T) {
	got := NodeModulesDir()
	if !filepath.IsAbs(got) {
		t.Errorf("NodeModulesDir() = %q is not absolute", got)
	}
	if filepath.Base(got) != "node_modules" {
		t.Errorf("NodeModulesDir() base = %q, want node_modules", filepath.Base(got))
	}
}

func TestPlaywrightCLIDir(t *testing.T) {
	got := PlaywrightCLIDir()
	if !filepath.IsAbs(got) {
		t.Errorf("PlaywrightCLIDir() = %q is not absolute", got)
	}
	if filepath.Base(got) != "cli" {
		t.Errorf("PlaywrightCLIDir() base = %q, want cli", filepath.Base(got))
	}
}

func TestConfigPath(t *testing.T) {
	got := ConfigPath()
	if !filepath.IsAbs(got) {
		t.Errorf("ConfigPath() = %q is not absolute", got)
	}
	if filepath.Base(got) != "browser.json" {
		t.Errorf("ConfigPath() base = %q, want browser.json", filepath.Base(got))
	}
}

func TestPlaywrightConfigPath(t *testing.T) {
	got := PlaywrightConfigPath()
	if !filepath.IsAbs(got) {
		t.Errorf("PlaywrightConfigPath() = %q is not absolute", got)
	}
	if filepath.Base(got) != "cli.config.json" {
		t.Errorf("PlaywrightConfigPath() base = %q, want cli.config.json", filepath.Base(got))
	}
}

func TestLoadConfig_DefaultsWhenMissing(t *testing.T) {
	// If the file does not exist, LoadConfig returns the default config.
	if _, err := os.Stat(ConfigPath()); os.IsNotExist(err) {
		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig() returned error when file is missing: %v", err)
		}
		if cfg == nil {
			t.Fatal("LoadConfig() returned nil config")
		}
		if !cfg.Headless {
			t.Error("default config Headless should be true")
		}
		if cfg.Browser != "chromium" {
			t.Errorf("default config Browser = %q, want chromium", cfg.Browser)
		}
	} else {
		// File exists — just verify LoadConfig doesn't return an error
		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}
		if cfg == nil {
			t.Fatal("LoadConfig() returned nil config")
		}
	}
}

func TestIsInstalled_False(t *testing.T) {
	// In a test environment the playwright runtime is not installed.
	if IsInstalled() {
		t.Skip("playwright runtime is installed — skipping not-installed assertion")
	}
	// IsInstalled() returned false — that is the expected behaviour.
}

func TestConfigSave(t *testing.T) {
	path := ConfigPath()

	// Back up the original file if it exists, so the test is non-destructive.
	var original []byte
	if data, err := os.ReadFile(path); err == nil {
		original = data
		t.Cleanup(func() { _ = os.WriteFile(path, original, 0o644) })
	} else {
		t.Cleanup(func() { _ = os.Remove(path) })
	}

	cfg := DefaultConfig()
	cfg.Browser = "firefox"
	cfg.Timeout = 60

	if err := cfg.Save(); err != nil {
		t.Fatalf("Config.Save() error = %v", err)
	}

	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() after Save() error = %v", err)
	}
	if loaded.Browser != "firefox" {
		t.Errorf("Browser = %q, want firefox", loaded.Browser)
	}
	if loaded.Timeout != 60 {
		t.Errorf("Timeout = %d, want 60", loaded.Timeout)
	}
}

func TestWritePlaywrightConfig(t *testing.T) {
	path := PlaywrightConfigPath()

	var original []byte
	if data, err := os.ReadFile(path); err == nil {
		original = data
		t.Cleanup(func() { _ = os.WriteFile(path, original, 0o644) })
	} else {
		t.Cleanup(func() { _ = os.Remove(path) })
	}

	cfg := DefaultConfig()
	cfg.Headless = false
	cfg.Browser = "webkit"
	cfg.Timeout = 45

	if err := cfg.WritePlaywrightConfig(); err != nil {
		t.Fatalf("Config.WritePlaywrightConfig() error = %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("WritePlaywrightConfig() did not create file: %v", err)
	}
}

func TestIsInstalled_AllFilesExist(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create all three required files for IsInstalled() to return true.
	node := filepath.Join(tmpDir, ".valksor", "kvelmo", "runtime", "node")
	if err := os.MkdirAll(filepath.Dir(node), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(node, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}

	cliEntry := filepath.Join(tmpDir, ".valksor", "kvelmo", "runtime", "node_modules", "@playwright", "cli", "playwright-cli.js")
	if err := os.MkdirAll(filepath.Dir(cliEntry), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cliEntry, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	wrapper := filepath.Join(tmpDir, ".valksor", "kvelmo", "bin", "playwright-cli")
	if err := os.MkdirAll(filepath.Dir(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wrapper, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}

	if !IsInstalled() {
		t.Error("IsInstalled() should return true when all required files exist")
	}
}

func TestVersion_NotInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	// With empty HOME, nothing is installed → Version returns error
	_, err := Version()
	if err == nil {
		t.Error("Version() should return error when playwright is not installed")
	}
}

func TestNpmBinaryPath(t *testing.T) {
	got := NpmBinaryPath()
	if !filepath.IsAbs(got) {
		t.Errorf("NpmBinaryPath() = %q is not absolute", got)
	}
	if filepath.Base(got) != "npm-cli.js" {
		t.Errorf("NpmBinaryPath() base = %q, want npm-cli.js", filepath.Base(got))
	}
}
