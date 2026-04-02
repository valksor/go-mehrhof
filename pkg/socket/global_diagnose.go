package socket

import (
	"context"
	"log/slog"

	"github.com/valksor/kvelmo/pkg/agent"
	"github.com/valksor/kvelmo/pkg/settings"
)

type diagnoseCheckResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
	Fix    string `json:"fix,omitempty"`
}

type diagnoseProviderResult struct {
	Name       string `json:"name"`
	Configured bool   `json:"configured"`
}

type diagnoseResponse struct {
	Checks       []diagnoseCheckResult    `json:"checks"`
	GlobalSocket string                   `json:"global_socket"`
	Providers    []diagnoseProviderResult `json:"providers"`
	Issues       []string                 `json:"issues,omitempty"`
}

func (g *GlobalSocket) handleDiagnose(_ context.Context, req *Request) (*Response, error) {
	preflight := agent.RunPreflight() //nolint:contextcheck // RunPreflight manages its own timeouts internally

	checks := make([]diagnoseCheckResult, 0, len(preflight.Checks))
	var issues []string

	for _, c := range preflight.Checks {
		checks = append(checks, diagnoseCheckResult{
			Name:   c.Name,
			Status: string(c.Status),
			Detail: c.Detail,
			Fix:    c.Fix,
		})
		if c.Fix != "" {
			issues = append(issues, c.Fix)
		}
	}

	// Socket is running since we're handling this request
	socketStatus := "running"

	// Check provider tokens
	providerChecks := []struct {
		name   string
		envVar string
	}{
		{"GitHub", "GITHUB_TOKEN"},
		{"GitLab", "GITLAB_TOKEN"},
		{"Linear", "LINEAR_TOKEN"},
		{"Wrike", "WRIKE_TOKEN"},
	}

	envMap, envErr := settings.LoadEnvMap("")
	if envErr != nil {
		slog.Warn("diagnose: failed to load .env", "error", envErr)
	}

	providers := make([]diagnoseProviderResult, 0, len(providerChecks))
	for _, p := range providerChecks {
		configured := envMap.Get(p.envVar) != ""

		providers = append(providers, diagnoseProviderResult{
			Name:       p.name,
			Configured: configured,
		})
	}

	return NewResultResponse(req.ID, diagnoseResponse{
		Checks:       checks,
		GlobalSocket: socketStatus,
		Providers:    providers,
		Issues:       issues,
	})
}
