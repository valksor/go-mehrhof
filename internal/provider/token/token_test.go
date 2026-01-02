package token

import (
	"errors"
	"os"
	"testing"
)

func cleanupEnv(vars ...string) func() {
	// Track whether each var was originally set (not just empty string)
	wasSet := make(map[string]bool)
	saved := make(map[string]string)
	for _, v := range vars {
		val, exists := os.LookupEnv(v)
		wasSet[v] = exists
		saved[v] = val
		_ = os.Unsetenv(v)
	}

	return func() {
		for k := range saved {
			if wasSet[k] {
				_ = os.Setenv(k, saved[k])
			} else {
				_ = os.Unsetenv(k)
			}
		}
	}
}

func TestResolveToken(t *testing.T) {
	t.Run("MEHR prefixed token has priority", func(t *testing.T) {
		t.Setenv("MEHR_TEST_TOKEN", "mehr-token")
		t.Setenv("TEST_TOKEN", "default-token")

		tok, err := ResolveToken(Config("TEST", "config-token"))
		if err != nil {
			t.Fatalf("ResolveToken error = %v", err)
		}
		if tok != "mehr-token" {
			t.Errorf("token = %q, want %q", tok, "mehr-token")
		}
	})

	t.Run("default env var fallback", func(t *testing.T) {
		t.Setenv("TEST_TOKEN", "default-token")

		tok, err := ResolveToken(Config("TEST", "config-token").WithEnvVars("TEST_TOKEN"))
		if err != nil {
			t.Fatalf("ResolveToken error = %v", err)
		}
		if tok != "default-token" {
			t.Errorf("token = %q, want %q", tok, "default-token")
		}
	})

	t.Run("config token fallback", func(t *testing.T) {
		tok, err := ResolveToken(Config("TEST", "config-token"))
		if err != nil {
			t.Fatalf("ResolveToken error = %v", err)
		}
		if tok != "config-token" {
			t.Errorf("token = %q, want %q", tok, "config-token")
		}
	})

	t.Run("no token available returns ErrNoToken", func(t *testing.T) {
		_, err := ResolveToken(Config("TEST", ""))
		if !errors.Is(err, ErrNoToken) {
			t.Errorf("error = %v, want %v", err, ErrNoToken)
		}
	})

	t.Run("CLI fallback is used when provided", func(t *testing.T) {
		fallbackCalled := false
		cfg := Config("TEST", "").
			WithCLIFallback(func() string {
				fallbackCalled = true

				return "cli-token"
			})

		tok, err := ResolveToken(cfg)
		if err != nil {
			t.Fatalf("ResolveToken error = %v", err)
		}
		if tok != "cli-token" {
			t.Errorf("token = %q, want %q", tok, "cli-token")
		}
		if !fallbackCalled {
			t.Error("CLI fallback was not called")
		}
	})

	t.Run("CLI fallback is not used when env var is set", func(t *testing.T) {
		t.Setenv("TEST_TOKEN", "env-token")

		fallbackCalled := false
		cfg := Config("TEST", "").
			WithEnvVars("TEST_TOKEN").
			WithCLIFallback(func() string {
				fallbackCalled = true

				return "cli-token"
			})

		tok, err := ResolveToken(cfg)
		if err != nil {
			t.Fatalf("ResolveToken error = %v", err)
		}
		if tok != "env-token" {
			t.Errorf("token = %q, want %q", tok, "env-token")
		}
		if fallbackCalled {
			t.Error("CLI fallback was called when env var was set")
		}
	})

	t.Run("multiple default env vars are checked in order", func(t *testing.T) {
		// First env var in list
		cfg := Config("TEST", "").WithEnvVars("TEST_TOKEN", "TEST_ALT_TOKEN")

		t.Setenv("TEST_ALT_TOKEN", "alt-token")
		tok, err := ResolveToken(cfg)
		if err != nil {
			t.Fatalf("ResolveToken error = %v", err)
		}
		if tok != "alt-token" {
			t.Errorf("token = %q, want %q", tok, "alt-token")
		}

		// First env var takes priority - need a new subtest to test this
	})

	t.Run("first env var takes priority", func(t *testing.T) {
		cfg := Config("TEST", "").WithEnvVars("TEST_TOKEN", "TEST_ALT_TOKEN")

		t.Setenv("TEST_TOKEN", "first-token")
		tok, err := ResolveToken(cfg)
		if err != nil {
			t.Fatalf("ResolveToken error = %v", err)
		}
		if tok != "first-token" {
			t.Errorf("token = %q, want %q", tok, "first-token")
		}
	})
}

func TestConfig(t *testing.T) {
	cfg := Config("PROVIDER", "my-token")

	if cfg.ProviderName != "PROVIDER" {
		t.Errorf("ProviderName = %q, want %q", cfg.ProviderName, "PROVIDER")
	}
	if cfg.ConfigToken != "my-token" {
		t.Errorf("ConfigToken = %q, want %q", cfg.ConfigToken, "my-token")
	}
}

func TestWithEnvVars(t *testing.T) {
	cfg := Config("TEST", "").
		WithEnvVars("VAR1", "VAR2")

	if len(cfg.DefaultEnvVars) != 2 {
		t.Errorf("len(DefaultEnvVars) = %d, want 2", len(cfg.DefaultEnvVars))
	}
	if cfg.DefaultEnvVars[0] != "VAR1" {
		t.Errorf("DefaultEnvVars[0] = %q, want %q", cfg.DefaultEnvVars[0], "VAR1")
	}
	if cfg.DefaultEnvVars[1] != "VAR2" {
		t.Errorf("DefaultEnvVars[1] = %q, want %q", cfg.DefaultEnvVars[1], "VAR2")
	}
}

func TestWithCLIFallback(t *testing.T) {
	fallback := func() string { return "cli-token" }
	cfg := Config("TEST", "").WithCLIFallback(fallback)

	if cfg.OptionalCLIFallback == nil {
		t.Error("OptionalCLIFallback was not set")
	}
}

func TestMustResolveToken(t *testing.T) {
	t.Run("panics when no token available", func(t *testing.T) {
		defer cleanupEnv("MEHR_TEST_TOKEN", "TEST_TOKEN")()

		defer func() {
			if r := recover(); r == nil {
				t.Error("MustResolveToken did not panic")
			}
		}()

		_ = MustResolveToken(Config("TEST", ""))
	})

	t.Run("returns token when available", func(t *testing.T) {
		t.Setenv("TEST_TOKEN", "test-token")

		tok := MustResolveToken(Config("TEST", "").WithEnvVars("TEST_TOKEN"))
		if tok != "test-token" {
			t.Errorf("token = %q, want %q", tok, "test-token")
		}
	})
}
