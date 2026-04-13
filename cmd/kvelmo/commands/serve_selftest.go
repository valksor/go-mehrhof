package commands

import (
	"log/slog"
	"os/exec"

	"github.com/valksor/kvelmo/settings"
)

// runStartupChecks performs non-blocking health checks at startup.
// Warnings are logged via slog; nothing blocks the server from starting.
func runStartupChecks() {
	// Check git availability
	if _, err := exec.LookPath("git"); err != nil {
		slog.Warn("git not found in PATH", "fix", "install git for full functionality")
	}

	// Check provider tokens
	checkProviderTokens()
}

func checkProviderTokens() {
	tokens := []string{"GITHUB_TOKEN", "GITLAB_TOKEN", "WRIKE_TOKEN", "LINEAR_TOKEN"}
	hasAny := false

	// Check .env files for provider tokens
	if env, err := settings.LoadEnvMap(""); err == nil {
		for _, t := range tokens {
			if env.Get(t) != "" {
				hasAny = true

				break
			}
		}
	}

	if !hasAny {
		slog.Info("no provider tokens configured", "fix", "run 'kvelmo github login' or configure tokens via 'kvelmo config set'")
	}
}
