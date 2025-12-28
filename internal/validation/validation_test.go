package validation

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/valksor/go-mehrhof/internal/config"
	"github.com/valksor/go-mehrhof/internal/storage"
)

func TestResultAddError(t *testing.T) {
	r := NewResult()

	r.AddError("TEST_CODE", "test message", "test.path", "test.yaml")

	if r.Valid {
		t.Error("expected result to be invalid after adding error")
	}
	if r.Errors != 1 {
		t.Errorf("expected 1 error, got %d", r.Errors)
	}
	if len(r.Findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(r.Findings))
	}
	if r.Findings[0].Severity != SeverityError {
		t.Errorf("expected error severity, got %s", r.Findings[0].Severity)
	}
}

func TestResultAddWarning(t *testing.T) {
	r := NewResult()

	r.AddWarning("TEST_CODE", "test message", "test.path", "test.yaml")

	if !r.Valid {
		t.Error("expected result to be valid after adding only warning")
	}
	if r.Warnings != 1 {
		t.Errorf("expected 1 warning, got %d", r.Warnings)
	}
}

func TestResultMerge(t *testing.T) {
	r1 := NewResult()
	r1.AddError("CODE1", "error 1", "", "")

	r2 := NewResult()
	r2.AddWarning("CODE2", "warning 1", "", "")

	r1.Merge(r2)

	if r1.Errors != 1 {
		t.Errorf("expected 1 error, got %d", r1.Errors)
	}
	if r1.Warnings != 1 {
		t.Errorf("expected 1 warning, got %d", r1.Warnings)
	}
	if len(r1.Findings) != 2 {
		t.Errorf("expected 2 findings, got %d", len(r1.Findings))
	}
}

func TestResultFormatJSON(t *testing.T) {
	r := NewResult()
	r.AddError("TEST", "test message", "path", "file")

	output := r.Format("json")

	if output == "" {
		t.Error("expected non-empty JSON output")
	}
	if output[0] != '{' {
		t.Error("expected JSON output to start with '{'")
	}
}

func TestResultFormatText(t *testing.T) {
	r := NewResult()
	r.AddError("TEST", "test message", "path", "file")

	output := r.Format("text")

	if output == "" {
		t.Error("expected non-empty text output")
	}
}

func TestValidateAgentAliases_CircularDependency(t *testing.T) {
	aliases := map[string]storage.AgentAliasConfig{
		"a": {Extends: "b"},
		"b": {Extends: "a"},
	}
	result := NewResult()
	builtInAgents := []string{"claude"}

	validateAgentAliases(aliases, "config.yaml", builtInAgents, result)

	if result.Valid {
		t.Error("expected circular dependency to be detected")
	}

	// Check that the error code is correct
	foundCircular := false
	for _, f := range result.Findings {
		if f.Code == CodeAgentAliasCircular {
			foundCircular = true
			break
		}
	}
	if !foundCircular {
		t.Error("expected AGENT_ALIAS_CIRCULAR error code")
	}
}

func TestValidateAgentAliases_UndefinedExtends(t *testing.T) {
	aliases := map[string]storage.AgentAliasConfig{
		"custom": {Extends: "nonexistent"},
	}
	result := NewResult()
	builtInAgents := []string{"claude"}

	validateAgentAliases(aliases, "config.yaml", builtInAgents, result)

	if result.Valid {
		t.Error("expected undefined extends to be detected")
	}

	foundUndefined := false
	for _, f := range result.Findings {
		if f.Code == CodeAgentAliasUndefined {
			foundUndefined = true
			break
		}
	}
	if !foundUndefined {
		t.Error("expected AGENT_ALIAS_UNDEFINED error code")
	}
}

func TestValidateAgentAliases_ValidChain(t *testing.T) {
	aliases := map[string]storage.AgentAliasConfig{
		"a": {Extends: "claude"},
		"b": {Extends: "a"},
		"c": {Extends: "b"},
	}
	result := NewResult()
	builtInAgents := []string{"claude"}

	validateAgentAliases(aliases, "config.yaml", builtInAgents, result)

	if !result.Valid {
		t.Errorf("expected valid alias chain, got errors: %+v", result.Findings)
	}
}

func TestValidateAgentAliases_MissingExtends(t *testing.T) {
	aliases := map[string]storage.AgentAliasConfig{
		"bad": {Description: "no extends field"},
	}
	result := NewResult()
	builtInAgents := []string{"claude"}

	validateAgentAliases(aliases, "config.yaml", builtInAgents, result)

	if result.Valid {
		t.Error("expected missing extends to be detected")
	}
}

func TestValidateGitPattern_ValidPlaceholders(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		wantWarning bool
	}{
		{"valid branch pattern", "{type}/{key}--{slug}", false},
		{"valid with task_id", "task/{task_id}", false},
		{"invalid placeholder", "{invalid}", true},
		{"mixed valid and invalid", "{key}/{bad}", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewResult()
			validateGitPattern(tt.pattern, "git.branch_pattern", "config.yaml", result)

			hasWarning := result.Warnings > 0
			if hasWarning != tt.wantWarning {
				t.Errorf("pattern %q: expected warning=%v, got warning=%v", tt.pattern, tt.wantWarning, hasWarning)
			}
		})
	}
}

func TestSlicesContains(t *testing.T) {
	slice := []string{"a", "b", "c"}

	if !slices.Contains(slice, "b") {
		t.Error("expected to find 'b' in slice")
	}
	if slices.Contains(slice, "d") {
		t.Error("expected not to find 'd' in slice")
	}
}

func TestValidatorNew(t *testing.T) {
	v := New("/tmp/test", Options{Strict: true})
	if v == nil {
		t.Fatal("expected non-nil validator")
	}
	if v.workspacePath != "/tmp/test" {
		t.Errorf("expected workspacePath /tmp/test, got %s", v.workspacePath)
	}
	if !v.opts.Strict {
		t.Error("expected Strict option to be true")
	}
}

func TestValidatorSetBuiltInAgents(t *testing.T) {
	v := New("/tmp/test", Options{})
	v.SetBuiltInAgents([]string{"agent1", "agent2"})
	if len(v.builtInAgents) != 2 {
		t.Errorf("expected 2 built-in agents, got %d", len(v.builtInAgents))
	}
}

func TestValidatorWorkspaceConfigPath(t *testing.T) {
	v := New("/tmp/test", Options{})
	path := v.WorkspaceConfigPath()
	expected := "/tmp/test/.mehrhof/config.yaml"
	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}

func TestValidateGitSettings(t *testing.T) {
	tests := []struct {
		name         string
		git          storage.GitSettings
		wantErrors   int
		wantWarnings int
	}{
		{
			name:         "empty branch pattern",
			git:          storage.GitSettings{BranchPattern: ""},
			wantWarnings: 1,
		},
		{
			name:         "valid branch pattern",
			git:          storage.GitSettings{BranchPattern: "{type}/{key}--{slug}"},
			wantWarnings: 0,
		},
		{
			name:         "pattern with double dots",
			git:          storage.GitSettings{BranchPattern: "feature..{key}"},
			wantWarnings: 1,
		},
		{
			name:         "pattern starting with slash",
			git:          storage.GitSettings{BranchPattern: "/feature/{key}"},
			wantWarnings: 1,
		},
		{
			name:         "pattern ending with slash",
			git:          storage.GitSettings{BranchPattern: "feature/{key}/"},
			wantWarnings: 1,
		},
		{
			name:         "valid commit prefix",
			git:          storage.GitSettings{BranchPattern: "{key}", CommitPrefix: "[{key}]"},
			wantWarnings: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewResult()
			validateGitSettings(tt.git, "config.yaml", result)
			if result.Errors != tt.wantErrors {
				t.Errorf("expected %d errors, got %d", tt.wantErrors, result.Errors)
			}
			if result.Warnings != tt.wantWarnings {
				t.Errorf("expected %d warnings, got %d", tt.wantWarnings, result.Warnings)
			}
		})
	}
}

func TestValidateAgentSettings(t *testing.T) {
	builtInAgents := []string{"claude"}
	aliases := map[string]storage.AgentAliasConfig{
		"custom": {Extends: "claude"},
	}

	tests := []struct {
		name       string
		agent      storage.AgentSettings
		wantErrors int
	}{
		{
			name:       "valid built-in default",
			agent:      storage.AgentSettings{Default: "claude", Timeout: 60, MaxRetries: 3},
			wantErrors: 0,
		},
		{
			name:       "valid alias default",
			agent:      storage.AgentSettings{Default: "custom", Timeout: 60, MaxRetries: 3},
			wantErrors: 0,
		},
		{
			name:       "unknown default agent",
			agent:      storage.AgentSettings{Default: "unknown", Timeout: 60, MaxRetries: 3},
			wantErrors: 1,
		},
		{
			name:       "timeout out of range negative",
			agent:      storage.AgentSettings{Default: "claude", Timeout: -1, MaxRetries: 3},
			wantErrors: 1,
		},
		{
			name:       "timeout out of range high",
			agent:      storage.AgentSettings{Default: "claude", Timeout: 3601, MaxRetries: 3},
			wantErrors: 1,
		},
		{
			name:       "max retries out of range negative",
			agent:      storage.AgentSettings{Default: "claude", Timeout: 60, MaxRetries: -1},
			wantErrors: 1,
		},
		{
			name:       "max retries out of range high",
			agent:      storage.AgentSettings{Default: "claude", Timeout: 60, MaxRetries: 11},
			wantErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewResult()
			validateAgentSettings(tt.agent, "config.yaml", builtInAgents, aliases, result)
			if result.Errors != tt.wantErrors {
				t.Errorf("expected %d errors, got %d", tt.wantErrors, result.Errors)
			}
		})
	}
}

func TestValidateWorkflowSettings(t *testing.T) {
	tests := []struct {
		name         string
		workflow     storage.WorkflowSettings
		wantWarnings int
	}{
		{
			name:         "valid retention days",
			workflow:     storage.WorkflowSettings{SessionRetentionDays: 30},
			wantWarnings: 0,
		},
		{
			name:         "negative retention days",
			workflow:     storage.WorkflowSettings{SessionRetentionDays: -1},
			wantWarnings: 1,
		},
		{
			name:         "excessive retention days",
			workflow:     storage.WorkflowSettings{SessionRetentionDays: 400},
			wantWarnings: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewResult()
			validateWorkflowSettings(tt.workflow, "config.yaml", result)
			if result.Warnings != tt.wantWarnings {
				t.Errorf("expected %d warnings, got %d", tt.wantWarnings, result.Warnings)
			}
		})
	}
}

func TestValidateEnvVarReferences(t *testing.T) {
	// Set a test env var
	_ = os.Setenv("TEST_VAR_EXISTS", "value")
	defer func() { _ = os.Unsetenv("TEST_VAR_EXISTS") }()

	tests := []struct {
		name         string
		env          map[string]string
		wantWarnings int
	}{
		{
			name:         "no env vars",
			env:          map[string]string{},
			wantWarnings: 0,
		},
		{
			name:         "existing env var",
			env:          map[string]string{"key": "${TEST_VAR_EXISTS}"},
			wantWarnings: 0,
		},
		{
			name:         "missing env var",
			env:          map[string]string{"key": "${NONEXISTENT_VAR_12345}"},
			wantWarnings: 1,
		},
		{
			name:         "multiple missing env vars",
			env:          map[string]string{"key": "${MISSING1}_${MISSING2}"},
			wantWarnings: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewResult()
			validateEnvVarReferences(tt.env, "agents.test.env", "config.yaml", result)
			if result.Warnings != tt.wantWarnings {
				t.Errorf("expected %d warnings, got %d", tt.wantWarnings, result.Warnings)
			}
		})
	}
}

func TestValidatePluginsConfig(t *testing.T) {
	tests := []struct {
		name         string
		plugins      storage.PluginsConfig
		wantWarnings int
	}{
		{
			name:         "empty plugins",
			plugins:      storage.PluginsConfig{},
			wantWarnings: 0,
		},
		{
			name: "config for enabled plugin",
			plugins: storage.PluginsConfig{
				Enabled: []string{"myplugin"},
				Config:  map[string]map[string]interface{}{"myplugin": {"key": "value"}},
			},
			wantWarnings: 0,
		},
		{
			name: "config for non-enabled plugin",
			plugins: storage.PluginsConfig{
				Enabled: []string{},
				Config:  map[string]map[string]interface{}{"myplugin": {"key": "value"}},
			},
			wantWarnings: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewResult()
			validatePluginsConfig(tt.plugins, "config.yaml", result)
			if result.Warnings != tt.wantWarnings {
				t.Errorf("expected %d warnings, got %d", tt.wantWarnings, result.Warnings)
			}
		})
	}
}

func TestValidateGitHubSettings(t *testing.T) {
	tests := []struct {
		name         string
		gh           *storage.GitHubSettings
		wantWarnings int
	}{
		{
			name:         "empty settings",
			gh:           &storage.GitHubSettings{},
			wantWarnings: 0,
		},
		{
			name:         "valid branch pattern",
			gh:           &storage.GitHubSettings{BranchPattern: "{type}/{key}"},
			wantWarnings: 0,
		},
		{
			name:         "invalid branch pattern",
			gh:           &storage.GitHubSettings{BranchPattern: "{invalid}"},
			wantWarnings: 1,
		},
		{
			name:         "valid commit prefix",
			gh:           &storage.GitHubSettings{CommitPrefix: "[{key}]"},
			wantWarnings: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewResult()
			validateGitHubSettings(tt.gh, "config.yaml", result)
			if result.Warnings != tt.wantWarnings {
				t.Errorf("expected %d warnings, got %d", tt.wantWarnings, result.Warnings)
			}
		})
	}
}

func TestValidateAppAgentConfig(t *testing.T) {
	tests := []struct {
		name       string
		agent      config.AgentConfig
		wantErrors int
	}{
		{
			name: "valid config",
			agent: config.AgentConfig{
				Default:    "claude",
				Timeout:    60,
				MaxRetries: 3,
				Claude:     config.ClaudeConfig{MaxTokens: 4096, Temperature: 0.7},
			},
			wantErrors: 0,
		},
		{
			name: "invalid default agent",
			agent: config.AgentConfig{
				Default:    "invalid",
				Timeout:    60,
				MaxRetries: 3,
				Claude:     config.ClaudeConfig{MaxTokens: 4096, Temperature: 0.7},
			},
			wantErrors: 1,
		},
		{
			name: "timeout out of range",
			agent: config.AgentConfig{
				Default:    "claude",
				Timeout:    -1,
				MaxRetries: 3,
				Claude:     config.ClaudeConfig{MaxTokens: 4096, Temperature: 0.7},
			},
			wantErrors: 1,
		},
		{
			name: "max retries out of range",
			agent: config.AgentConfig{
				Default:    "claude",
				Timeout:    60,
				MaxRetries: 15,
				Claude:     config.ClaudeConfig{MaxTokens: 4096, Temperature: 0.7},
			},
			wantErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewResult()
			validateAppAgentConfig(tt.agent, result)
			if result.Errors != tt.wantErrors {
				t.Errorf("expected %d errors, got %d", tt.wantErrors, result.Errors)
			}
		})
	}
}

func TestValidateClaudeConfig(t *testing.T) {
	tests := []struct {
		name         string
		claude       config.ClaudeConfig
		wantErrors   int
		wantWarnings int
	}{
		{
			name:         "valid config",
			claude:       config.ClaudeConfig{MaxTokens: 4096, Temperature: 0.7},
			wantErrors:   0,
			wantWarnings: 0,
		},
		{
			name:         "max tokens too low",
			claude:       config.ClaudeConfig{MaxTokens: 0, Temperature: 0.7},
			wantWarnings: 1,
		},
		{
			name:         "max tokens too high",
			claude:       config.ClaudeConfig{MaxTokens: 300000, Temperature: 0.7},
			wantWarnings: 1,
		},
		{
			name:       "temperature too low",
			claude:     config.ClaudeConfig{MaxTokens: 4096, Temperature: -0.1},
			wantErrors: 1,
		},
		{
			name:       "temperature too high",
			claude:     config.ClaudeConfig{MaxTokens: 4096, Temperature: 2.5},
			wantErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewResult()
			validateClaudeConfig(tt.claude, result)
			if result.Errors != tt.wantErrors {
				t.Errorf("expected %d errors, got %d", tt.wantErrors, result.Errors)
			}
			if result.Warnings != tt.wantWarnings {
				t.Errorf("expected %d warnings, got %d", tt.wantWarnings, result.Warnings)
			}
		})
	}
}

func TestValidateAppStorageConfig(t *testing.T) {
	tests := []struct {
		name         string
		storage      config.StorageConfig
		wantWarnings int
	}{
		{
			name:         "valid config",
			storage:      config.StorageConfig{MaxBlueprints: 100, SessionRetentionDays: 30},
			wantWarnings: 0,
		},
		{
			name:         "max blueprints too low",
			storage:      config.StorageConfig{MaxBlueprints: 0, SessionRetentionDays: 30},
			wantWarnings: 1,
		},
		{
			name:         "max blueprints too high",
			storage:      config.StorageConfig{MaxBlueprints: 20000, SessionRetentionDays: 30},
			wantWarnings: 1,
		},
		{
			name:         "retention days negative",
			storage:      config.StorageConfig{MaxBlueprints: 100, SessionRetentionDays: -1},
			wantWarnings: 1,
		},
		{
			name:         "retention days too high",
			storage:      config.StorageConfig{MaxBlueprints: 100, SessionRetentionDays: 400},
			wantWarnings: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewResult()
			validateAppStorageConfig(tt.storage, result)
			if result.Warnings != tt.wantWarnings {
				t.Errorf("expected %d warnings, got %d", tt.wantWarnings, result.Warnings)
			}
		})
	}
}

func TestValidateAppGitConfig(t *testing.T) {
	tests := []struct {
		name         string
		git          config.GitConfig
		wantWarnings int
	}{
		{
			name:         "empty config",
			git:          config.GitConfig{},
			wantWarnings: 0,
		},
		{
			name:         "valid branch pattern",
			git:          config.GitConfig{BranchPattern: "{type}/{key}"},
			wantWarnings: 0,
		},
		{
			name:         "invalid branch pattern",
			git:          config.GitConfig{BranchPattern: "{invalid}"},
			wantWarnings: 1,
		},
		{
			name:         "valid commit prefix",
			git:          config.GitConfig{CommitPrefix: "[{key}]"},
			wantWarnings: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewResult()
			validateAppGitConfig(tt.git, result)
			if result.Warnings != tt.wantWarnings {
				t.Errorf("expected %d warnings, got %d", tt.wantWarnings, result.Warnings)
			}
		})
	}
}

func TestValidateAppProvidersConfig(t *testing.T) {
	tests := []struct {
		name         string
		providers    config.ProvidersConfig
		wantWarnings int
	}{
		{
			name:         "empty default",
			providers:    config.ProvidersConfig{Default: ""},
			wantWarnings: 0,
		},
		{
			name:         "valid file provider",
			providers:    config.ProvidersConfig{Default: "file"},
			wantWarnings: 0,
		},
		{
			name:         "valid directory provider",
			providers:    config.ProvidersConfig{Default: "directory"},
			wantWarnings: 0,
		},
		{
			name:         "valid github provider",
			providers:    config.ProvidersConfig{Default: "github"},
			wantWarnings: 0,
		},
		{
			name:         "invalid provider",
			providers:    config.ProvidersConfig{Default: "invalid"},
			wantWarnings: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewResult()
			validateAppProvidersConfig(tt.providers, result)
			if result.Warnings != tt.wantWarnings {
				t.Errorf("expected %d warnings, got %d", tt.wantWarnings, result.Warnings)
			}
		})
	}
}

func TestValidateAppUIConfig(t *testing.T) {
	tests := []struct {
		name       string
		ui         config.UIConfig
		wantErrors int
	}{
		{
			name:       "valid text format",
			ui:         config.UIConfig{Format: "text", Progress: "spinner"},
			wantErrors: 0,
		},
		{
			name:       "valid json format",
			ui:         config.UIConfig{Format: "json", Progress: "dots"},
			wantErrors: 0,
		},
		{
			name:       "valid none progress",
			ui:         config.UIConfig{Format: "text", Progress: "none"},
			wantErrors: 0,
		},
		{
			name:       "invalid format",
			ui:         config.UIConfig{Format: "invalid", Progress: "spinner"},
			wantErrors: 1,
		},
		{
			name:       "invalid progress",
			ui:         config.UIConfig{Format: "text", Progress: "invalid"},
			wantErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewResult()
			validateAppUIConfig(tt.ui, result)
			if result.Errors != tt.wantErrors {
				t.Errorf("expected %d errors, got %d", tt.wantErrors, result.Errors)
			}
		})
	}
}

func TestValidateCrossConfig(t *testing.T) {
	builtInAgents := []string{"claude"}

	tests := []struct {
		name         string
		app          *config.Config
		ws           *storage.WorkspaceConfig
		wantWarnings int
		wantInfos    int
	}{
		{
			name: "matching configs",
			app: &config.Config{
				Agent: config.AgentConfig{Default: "claude"},
				Git:   config.GitConfig{BranchPattern: "{key}"},
			},
			ws: &storage.WorkspaceConfig{
				Agent:  storage.AgentSettings{Default: "claude"},
				Git:    storage.GitSettings{BranchPattern: "{key}"},
				Agents: map[string]storage.AgentAliasConfig{},
			},
			wantWarnings: 0,
			wantInfos:    0,
		},
		{
			name: "different branch patterns",
			app: &config.Config{
				Agent: config.AgentConfig{Default: "claude"},
				Git:   config.GitConfig{BranchPattern: "{key}"},
			},
			ws: &storage.WorkspaceConfig{
				Agent:  storage.AgentSettings{Default: "claude"},
				Git:    storage.GitSettings{BranchPattern: "{type}/{key}"},
				Agents: map[string]storage.AgentAliasConfig{},
			},
			wantInfos: 1,
		},
		{
			name: "different commit prefixes",
			app: &config.Config{
				Agent: config.AgentConfig{Default: "claude"},
				Git:   config.GitConfig{CommitPrefix: "[{key}]"},
			},
			ws: &storage.WorkspaceConfig{
				Agent:  storage.AgentSettings{Default: "claude"},
				Git:    storage.GitSettings{CommitPrefix: "{key}:"},
				Agents: map[string]storage.AgentAliasConfig{},
			},
			wantInfos: 1,
		},
		{
			name: "unknown workspace default agent",
			app: &config.Config{
				Agent: config.AgentConfig{Default: "claude"},
				Git:   config.GitConfig{},
			},
			ws: &storage.WorkspaceConfig{
				Agent:  storage.AgentSettings{Default: "unknown"},
				Git:    storage.GitSettings{},
				Agents: map[string]storage.AgentAliasConfig{},
			},
			wantWarnings: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewResult()
			validateCrossConfig(tt.app, tt.ws, builtInAgents, result)
			if result.Warnings != tt.wantWarnings {
				t.Errorf("expected %d warnings, got %d", tt.wantWarnings, result.Warnings)
			}
			// Count info findings
			infos := 0
			for _, f := range result.Findings {
				if f.Severity == SeverityInfo {
					infos++
				}
			}
			if infos != tt.wantInfos {
				t.Errorf("expected %d infos, got %d", tt.wantInfos, infos)
			}
		})
	}
}

func TestResultAddInfo(t *testing.T) {
	r := NewResult()
	r.AddInfo("TEST_CODE", "test message", "test.path", "test.yaml")

	if !r.Valid {
		t.Error("expected result to be valid after adding info")
	}
	if r.Errors != 0 {
		t.Errorf("expected 0 errors, got %d", r.Errors)
	}
	if r.Warnings != 0 {
		t.Errorf("expected 0 warnings, got %d", r.Warnings)
	}
	if len(r.Findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(r.Findings))
	}
	if r.Findings[0].Severity != SeverityInfo {
		t.Errorf("expected info severity, got %s", r.Findings[0].Severity)
	}
}

func TestResultAddErrorWithSuggestion(t *testing.T) {
	r := NewResult()
	r.AddErrorWithSuggestion("TEST_CODE", "test message", "test.path", "test.yaml", "fix it")

	if r.Valid {
		t.Error("expected result to be invalid after adding error")
	}
	if r.Findings[0].Suggestion != "fix it" {
		t.Errorf("expected suggestion 'fix it', got %s", r.Findings[0].Suggestion)
	}
}

func TestResultAddWarningWithSuggestion(t *testing.T) {
	r := NewResult()
	r.AddWarningWithSuggestion("TEST_CODE", "test message", "test.path", "test.yaml", "fix it")

	if !r.Valid {
		t.Error("expected result to be valid after adding warning")
	}
	if r.Findings[0].Suggestion != "fix it" {
		t.Errorf("expected suggestion 'fix it', got %s", r.Findings[0].Suggestion)
	}
}

func TestResultMergeNil(t *testing.T) {
	r := NewResult()
	r.AddError("CODE1", "error 1", "", "")

	r.Merge(nil)

	if r.Errors != 1 {
		t.Errorf("expected 1 error after merging nil, got %d", r.Errors)
	}
}

func TestResultFormatTextWithSuggestion(t *testing.T) {
	r := NewResult()
	r.AddErrorWithSuggestion("TEST", "test message", "path", "file", "fix suggestion")

	output := r.Format("text")

	if output == "" {
		t.Error("expected non-empty text output")
	}
	if !strings.Contains(output, "Suggestion:") {
		t.Error("expected suggestion in output")
	}
}

func TestResultFormatTextValid(t *testing.T) {
	r := NewResult()

	output := r.Format("text")

	if output == "" {
		t.Error("expected non-empty text output")
	}
	if !strings.Contains(output, "VALID") {
		t.Error("expected VALID in output")
	}
}

func TestResultFormatTextWithWarnings(t *testing.T) {
	r := NewResult()
	r.AddWarning("TEST", "test warning", "path", "file")

	output := r.Format("text")

	if output == "" {
		t.Error("expected non-empty text output")
	}
}

func TestResultFormatTextNoFile(t *testing.T) {
	r := NewResult()
	r.AddError("TEST", "test message", "path", "")

	output := r.Format("text")

	if output == "" {
		t.Error("expected non-empty text output")
	}
}

func TestValidateAppConfig(t *testing.T) {
	cfg := &config.Config{
		Agent: config.AgentConfig{
			Default:    "claude",
			Timeout:    60,
			MaxRetries: 3,
			Claude:     config.ClaudeConfig{MaxTokens: 4096, Temperature: 0.7},
		},
		Storage: config.StorageConfig{
			MaxBlueprints:        100,
			SessionRetentionDays: 30,
		},
		Git: config.GitConfig{
			BranchPattern: "{type}/{key}",
		},
		Providers: config.ProvidersConfig{
			Default: "file",
		},
		UI: config.UIConfig{
			Format:   "text",
			Progress: "spinner",
		},
	}

	result := NewResult()
	validateAppConfig(cfg, result)

	if !result.Valid {
		t.Errorf("expected valid config, got errors: %+v", result.Findings)
	}
}

func TestValidateWorkspaceConfig(t *testing.T) {
	builtInAgents := []string{"claude"}
	cfg := &storage.WorkspaceConfig{
		Git: storage.GitSettings{
			BranchPattern: "{type}/{key}",
		},
		Agent: storage.AgentSettings{
			Default:    "claude",
			Timeout:    60,
			MaxRetries: 3,
		},
		Workflow: storage.WorkflowSettings{
			SessionRetentionDays: 30,
		},
		Agents: map[string]storage.AgentAliasConfig{
			"custom": {Extends: "claude"},
		},
		Plugins: storage.PluginsConfig{
			Enabled: []string{"myplugin"},
			Config:  map[string]map[string]interface{}{"myplugin": {"key": "value"}},
		},
		GitHub: &storage.GitHubSettings{
			BranchPattern: "{type}/{key}",
		},
	}

	result := NewResult()
	validateWorkspaceConfig(cfg, "config.yaml", builtInAgents, result)

	if !result.Valid {
		t.Errorf("expected valid config, got errors: %+v", result.Findings)
	}
}

func TestValidatorValidate_WorkspaceOnly(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a minimal workspace
	ws, err := storage.OpenWorkspace(tmpDir)
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	v := New(tmpDir, Options{WorkspaceOnly: true})

	ctx := t.Context()
	result, err := v.Validate(ctx)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	// Should be valid with default config
	if result == nil {
		t.Fatal("result is nil")
	}
}

func TestValidatorValidate_AppOnly(t *testing.T) {
	tmpDir := t.TempDir()

	v := New(tmpDir, Options{AppOnly: true})

	ctx := t.Context()
	result, err := v.Validate(ctx)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}
}

func TestValidatorValidate_StrictMode(t *testing.T) {
	tmpDir := t.TempDir()

	// Create workspace with config that has warnings
	ws, err := storage.OpenWorkspace(tmpDir)
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	v := New(tmpDir, Options{Strict: true, WorkspaceOnly: true})

	ctx := t.Context()
	result, err := v.Validate(ctx)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}
}

func TestValidatorValidate_NoConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Create workspace without config file
	ws, err := storage.OpenWorkspace(tmpDir)
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	// Just create the directory structure without config
	if err := os.MkdirAll(ws.TaskRoot(), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	v := New(tmpDir, Options{WorkspaceOnly: true})

	ctx := t.Context()
	result, err := v.Validate(ctx)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	// Should be valid - no config means defaults are used
	if result == nil {
		t.Fatal("result is nil")
	}
	if !result.Valid {
		t.Error("expected valid result when no config exists")
	}
}

func TestValidatorValidate_Full(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a minimal workspace
	ws, err := storage.OpenWorkspace(tmpDir)
	if err != nil {
		t.Fatalf("OpenWorkspace: %v", err)
	}
	if err := ws.EnsureInitialized(); err != nil {
		t.Fatalf("EnsureInitialized: %v", err)
	}

	v := New(tmpDir, Options{})

	ctx := t.Context()
	result, err := v.Validate(ctx)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}
}
