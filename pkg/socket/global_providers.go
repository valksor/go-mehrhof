package socket

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/valksor/kvelmo/pkg/provider"
	"github.com/valksor/kvelmo/pkg/settings"
)

// --- Provider Token Testing ---

func (g *GlobalSocket) handleProvidersList(_ context.Context, req *Request) (*Response, error) {
	type providerInfo struct {
		Name        string `json:"name"`
		Label       string `json:"label"`
		EnvVar      string `json:"env_var"`
		HelpURL     string `json:"help_url"`
		HelpSteps   string `json:"help_steps"`
		Scopes      string `json:"scopes"`
		TokenPrefix string `json:"token_prefix"`
		Configured  bool   `json:"configured"`
	}

	providers := make([]providerInfo, 0, len(providerLoginConfigs))
	for name, cfg := range providerLoginConfigs {
		token := resolveProviderToken(name)
		providers = append(providers, providerInfo{
			Name:        name,
			Label:       cfg.Name,
			EnvVar:      cfg.EnvVar,
			HelpURL:     cfg.HelpURL,
			HelpSteps:   cfg.HelpSteps,
			Scopes:      cfg.Scopes,
			TokenPrefix: cfg.TokenPrefix,
			Configured:  token != "",
		})
	}

	// Sort alphabetically for stable output
	slices.SortFunc(providers, func(a, b providerInfo) int {
		return strings.Compare(a.Name, b.Name)
	})

	return NewResultResponse(req.ID, map[string]any{
		"providers": providers,
	})
}

func (g *GlobalSocket) handleProvidersTest(ctx context.Context, req *Request) (*Response, error) {
	var params struct {
		Provider string `json:"provider"`
		Token    string `json:"token"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	if params.Provider == "" {
		return NewResultResponse(req.ID, map[string]any{
			"ok":     false,
			"detail": "Provider is required",
		})
	}

	// Resolve token: use configured token if not explicitly provided
	token := params.Token
	if token == "" || token == "__use_configured__" {
		token = resolveProviderToken(params.Provider)
	}
	if token == "" {
		return NewResultResponse(req.ID, map[string]any{
			"ok":     false,
			"detail": "No token configured",
		})
	}

	ok, detail := testProviderToken(ctx, params.Provider, token)

	return NewResultResponse(req.ID, map[string]any{
		"ok":     ok,
		"detail": detail,
	})
}

// providerLoginConfigs is the canonical provider login configuration from pkg/provider.
var providerLoginConfigs = provider.LoginConfigs

func (g *GlobalSocket) handleProviderLogin(ctx context.Context, req *Request) (*Response, error) {
	var params struct {
		Provider string `json:"provider"`
		Token    string `json:"token"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, fmt.Errorf("parse params: %w", err)
	}

	if params.Provider == "" {
		return NewErrorResponse(req.ID, -32602, "provider is required"), nil
	}
	if params.Token == "" {
		return NewErrorResponse(req.ID, -32602, "token is required"), nil
	}

	cfg, ok := providerLoginConfigs[params.Provider]
	if !ok {
		return NewErrorResponse(req.ID, -32602, "unknown provider: "+params.Provider), nil
	}

	// Validate token against provider API
	valid, detail := testProviderToken(ctx, params.Provider, params.Token)

	// Save token regardless of validation result (matches CLI behavior)
	if err := settings.SaveEnvVar(settings.ScopeGlobal, "", cfg.EnvVar, params.Token); err != nil {
		return NewErrorResponse(req.ID, -32603, "failed to save token: "+err.Error()), nil //nolint:nilerr // RPC error response pattern — Go error is nil, JSON-RPC error is in the response
	}

	envPath, envErr := settings.GlobalEnvPath()
	if envErr != nil {
		slog.Warn("failed to resolve global env path", "error", envErr)
	}

	return NewResultResponse(req.ID, map[string]any{
		"saved":    true,
		"valid":    valid,
		"detail":   detail,
		"env_path": envPath,
		"masked":   settings.MaskToken(params.Token),
	})
}

// resolveProviderToken reads the configured token for a provider from env vars and settings .env files.
func resolveProviderToken(providerName string) string {
	cfg, ok := providerLoginConfigs[providerName]
	if !ok {
		return ""
	}

	envVar := cfg.EnvVar
	if envVar == "" {
		return ""
	}

	// Check .env files (global and project)
	envMap, err := settings.LoadEnvMap("")
	if err != nil {
		slog.Warn("resolveProviderToken: failed to load .env", "error", err)

		return ""
	}

	return envMap.Get(envVar)
}

// testProviderToken makes a lightweight API call to verify a token is valid.
func testProviderToken(ctx context.Context, providerName, token string) (bool, string) {
	switch providerName {
	case "github":
		return testHTTPToken(ctx, "https://api.github.com/user", token, "token")
	case "gitlab":
		return testHTTPToken(ctx, "https://gitlab.com/api/v4/user", token, "PRIVATE-TOKEN")
	case "linear":
		return testLinearToken(ctx, token)
	case "wrike":
		return testHTTPToken(ctx, "https://www.wrike.com/api/v4/contacts?me=true", token, "bearer")
	default:
		return false, "Unknown provider: " + providerName
	}
}

// testHTTPToken makes a GET request with the token and checks for 200 OK.
func testHTTPToken(ctx context.Context, url, token, authType string) (bool, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err.Error()
	}

	switch authType {
	case "token":
		req.Header.Set("Authorization", "token "+token)
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+token)
	case "PRIVATE-TOKEN":
		req.Header["PRIVATE-TOKEN"] = []string{token}
	}
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, "Connection failed: " + err.Error()
	}
	defer resp.Body.Close() //nolint:errcheck // close error is irrelevant; the response status is already captured

	if resp.StatusCode == http.StatusOK {
		return true, "Authenticated successfully"
	}

	return false, fmt.Sprintf("Authentication failed (HTTP %d)", resp.StatusCode)
}

// testLinearToken uses the GraphQL API to test a Linear token.
func testLinearToken(ctx context.Context, token string) (bool, string) {
	body := strings.NewReader(`{"query":"{ viewer { id } }"}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.linear.app/graphql", body)
	if err != nil {
		return false, err.Error()
	}

	if strings.HasPrefix(token, "Bearer ") {
		req.Header.Set("Authorization", token)
	} else {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, "Connection failed: " + err.Error()
	}
	defer resp.Body.Close() //nolint:errcheck // close error is irrelevant; the response status is already captured

	if resp.StatusCode == http.StatusOK {
		return true, "Authenticated successfully"
	}

	return false, fmt.Sprintf("Authentication failed (HTTP %d)", resp.StatusCode)
}
