package socket

import (
	"context"
	"os"

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

	var checks []diagnoseCheckResult
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

	envMap, _ := settings.LoadEnvMap("")

	var providers []diagnoseProviderResult
	for _, p := range providerChecks {
		configured := false
		if val := os.Getenv(p.envVar); val != "" {
			configured = true
		} else if envMap != nil {
			if val, ok := envMap[p.envVar]; ok && val != "" {
				configured = true
			}
		}

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
