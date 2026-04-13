package socket

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/valksor/kvelmo/agent"
	"github.com/valksor/kvelmo/settings"
)

// --- Settings Handlers (new two-tier system) ---

// SettingsGetParams holds params for settings.get.
type SettingsGetParams struct {
	ProjectPath string `json:"project_path,omitempty"` // Path to project for project-level settings
}

func (g *GlobalSocket) handleSettingsGet(ctx context.Context, req *Request) (*Response, error) {
	var params SettingsGetParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "invalid params: "+err.Error()), nil //nolint:nilerr // JSON-RPC error response
		}
	}

	// Determine project path
	projectPath := params.ProjectPath
	if projectPath == "" {
		// Try to get from first registered worktree (sorted for determinism)
		g.mu.RLock()
		paths := make([]string, 0, len(g.worktrees))
		for _, w := range g.worktrees {
			paths = append(paths, w.Path)
		}
		g.mu.RUnlock()
		slices.Sort(paths)
		if len(paths) > 0 {
			projectPath = paths[0]
		}
	}

	// Load effective settings (merged global + project)
	effective, global, project, err := settings.LoadEffective(projectPath)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	// Generate schema with custom agents added to agent selection options
	schema := settings.GenerateSchemaWithCustomAgents(effective)

	// Mask sensitive fields before sending to client
	effectiveMasked := settings.MaskSettings(effective)
	globalMasked := settings.MaskSettings(global)
	projectMasked := settings.MaskSettings(project)

	return NewResultResponse(req.ID, settings.SettingsResponse{
		Schema:    schema,
		Effective: effectiveMasked,
		Global:    globalMasked,
		Project:   projectMasked,
	})
}

func (g *GlobalSocket) handleSettingsSet(ctx context.Context, req *Request) (*Response, error) {
	var params struct {
		Scope       settings.Scope `json:"scope"`
		Values      map[string]any `json:"values"`
		ProjectPath string         `json:"project_path,omitempty"`
	}
	if req.Params == nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "params required"), nil
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
	}

	// Validate scope
	if params.Scope != settings.ScopeGlobal && params.Scope != settings.ScopeProject {
		return NewErrorResponse(req.ID, ErrCodeInvalidParams, "scope must be 'global' or 'project'"), nil
	}

	// Determine project path for project scope
	projectPath := params.ProjectPath
	if params.Scope == settings.ScopeProject && projectPath == "" {
		g.mu.RLock()
		paths := make([]string, 0, len(g.worktrees))
		for _, w := range g.worktrees {
			paths = append(paths, w.Path)
		}
		g.mu.RUnlock()
		slices.Sort(paths)
		if len(paths) > 0 {
			projectPath = paths[0]
		}
		if projectPath == "" {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, "project_path required for project scope"), nil
		}
	}

	// Load current settings for the target scope
	var current *settings.Settings
	var err error

	if params.Scope == settings.ScopeGlobal {
		current, err = settings.LoadGlobal()
	} else {
		current, err = settings.LoadProject(projectPath)
	}
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}
	if current == nil {
		current = &settings.Settings{}
	}

	// Apply each value
	for path, value := range params.Values {
		// Skip masked tokens
		if strVal, ok := value.(string); ok && settings.IsMaskedToken(strVal) {
			continue
		}

		// Check if this is a sensitive field
		if settings.IsSensitivePath(path) {
			// Write to .env file
			envVar := settings.GetEnvVarForPath(path)
			if strVal, ok := value.(string); ok && strVal != "" {
				if err := settings.SaveEnvVar(params.Scope, projectPath, envVar, strVal); err != nil {
					return NewErrorResponse(req.ID, ErrCodeInternal, fmt.Sprintf("save env var: %v", err)), nil
				}
			}

			continue
		}

		// Regular field - update settings struct
		if err := settings.SetValue(current, path, value); err != nil {
			return NewErrorResponse(req.ID, ErrCodeInvalidParams, err.Error()), nil
		}
	}

	// Save settings
	if params.Scope == settings.ScopeGlobal {
		err = settings.SaveGlobal(current)
	} else {
		err = settings.SaveProject(projectPath, current)
	}
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	// Return updated effective settings
	effective, global, project, err := settings.LoadEffective(projectPath)
	if err != nil {
		return NewErrorResponse(req.ID, ErrCodeInternal, err.Error()), nil
	}

	return NewResultResponse(req.ID, settings.SettingsResponse{
		Schema:    settings.GenerateSchemaWithCustomAgents(effective),
		Effective: settings.MaskSettings(effective),
		Global:    settings.MaskSettings(global),
		Project:   settings.MaskSettings(project),
	})
}

// --- Config Validation ---

func (g *GlobalSocket) handleConfigValidate(_ context.Context, req *Request) (*Response, error) {
	type check struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Detail string `json:"detail,omitempty"`
		Fix    string `json:"fix,omitempty"`
	}

	type result struct {
		Valid  bool    `json:"valid"`
		Checks []check `json:"checks"`
	}

	res := result{Valid: true}

	// Check settings can be loaded.
	effective := settings.DefaultSettings()
	global, settingsErr := settings.LoadGlobal()
	if settingsErr != nil {
		res.Valid = false
		res.Checks = append(res.Checks, check{
			Name:   "Settings",
			Status: "error",
			Detail: settingsErr.Error(),
			Fix:    "Run 'kvelmo config init' or fix YAML syntax in config file",
		})
	} else {
		if global != nil {
			settings.Merge(effective, global)
		}
		res.Checks = append(res.Checks, check{
			Name:   "Settings",
			Status: "ok",
			Detail: "valid",
		})
	}

	// Run preflight checks (git, agent CLIs).
	// Determine effective default agent so we only fail validity for relevant agents.
	defaultAgent := effective.Agent.Default
	if defaultAgent == "" {
		defaultAgent = "claude"
	}
	preflight := agent.RunPreflight() //nolint:contextcheck // RunPreflight manages its own timeouts internally
	for _, c := range preflight.Checks {
		status := "ok"
		switch c.Status {
		case agent.CheckPassed:
			// ok
		case agent.CheckFailed:
			// CLI agent checks (claude, codex) only fail validity when that
			// agent is the configured default.  Other checks (git) always
			// affect validity.
			if (c.Name == "claude" || c.Name == "codex") && c.Name != defaultAgent {
				status = "warning"
			} else {
				status = "error"
				res.Valid = false
			}
		case agent.CheckWarning:
			status = "warning"
		}
		res.Checks = append(res.Checks, check{
			Name:   c.Name,
			Status: status,
			Detail: c.Detail,
			Fix:    c.Fix,
		})
	}

	// Check provider tokens (informational, not required for validity).
	providerChecks := []struct {
		name   string
		envVar string
	}{
		{"GitHub", "GITHUB_TOKEN"},
		{"GitLab", "GITLAB_TOKEN"},
		{"Linear", "LINEAR_TOKEN"},
		{"Wrike", "WRIKE_TOKEN"},
	}

	envMap, envMapErr := settings.LoadEnvMap("")

	for _, p := range providerChecks {
		hasToken := envMapErr == nil && envMap.Get(p.envVar) != ""
		if hasToken {
			res.Checks = append(res.Checks, check{
				Name:   p.name,
				Status: "ok",
				Detail: "token configured",
			})
		} else {
			res.Checks = append(res.Checks, check{
				Name:   p.name,
				Status: "warning",
				Detail: "not configured",
				Fix:    fmt.Sprintf("Set %s or run 'kvelmo provider login %s'", p.envVar, strings.ToLower(p.name)),
			})
		}
	}

	// Check agent default is valid if settings loaded.
	if effective.Agent.Default != "" {
		allowed := []string{"claude", "codex", "openai", "anthropic", "ollama"}
		valid := slices.Contains(allowed, effective.Agent.Default)
		if !valid {
			if _, ok := effective.CustomAgents[effective.Agent.Default]; ok {
				valid = true
			}
		}
		if !valid {
			res.Valid = false
			res.Checks = append(res.Checks, check{
				Name:   "agent.default",
				Status: "error",
				Detail: fmt.Sprintf("unknown agent %q", effective.Agent.Default),
				Fix:    "Set agent.default to 'claude', 'codex', 'openai', 'anthropic', 'ollama', or a configured custom agent",
			})
		}
	}

	return NewResultResponse(req.ID, res)
}
