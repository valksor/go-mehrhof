package conductor

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/valksor/go-mehrhof/internal/agent"
	"github.com/valksor/go-mehrhof/internal/coordination"
	"github.com/valksor/go-mehrhof/internal/plugin"
	"github.com/valksor/go-mehrhof/internal/provider"
	"github.com/valksor/go-mehrhof/internal/storage"
	"github.com/valksor/go-mehrhof/internal/workflow"
)

// applyAgentEnv applies environment variables to an agent instance.
// It resolves any ${VAR} references in the env map and applies each key-value pair.
//
// Deprecated: Use coordination.ApplyEnvs instead.
func applyAgentEnv(agentInst agent.Agent, env map[string]string) agent.Agent {
	return coordination.ApplyEnvs(agentInst, env)
}

// resolveAgentForTask resolves the agent based on priority:
// CLI flag > Task config > Workspace default > Auto-detect
// Returns the resolved agent, the source identifier, and any error.
func (c *Conductor) resolveAgentForTask() (agent.Agent, string, error) {
	var agentName string
	var source string

	// Priority 1: CLI flag (opts.AgentName)
	if c.opts.AgentName != "" {
		agentName = c.opts.AgentName
		source = "cli"
	} else if c.taskAgentConfig != nil && c.taskAgentConfig.Name != "" {
		// Priority 2: Task frontmatter agent config
		agentName = c.taskAgentConfig.Name
		source = "task"
	} else {
		// Priority 3: Workspace default or auto-detect
		if cfg, err := c.workspace.LoadConfig(); err == nil && cfg.Agent.Default != "" {
			agentName = cfg.Agent.Default
			source = "workspace"
		} else {
			// Priority 4: Auto-detect
			agentInst, err := c.agents.Detect()
			if err != nil {
				return nil, "", fmt.Errorf("detect agent: %w", err)
			}

			return agentInst, "auto", nil
		}
	}

	// Get the agent by name
	agentInst, err := c.agents.Get(agentName)
	if err != nil {
		return nil, "", fmt.Errorf("get agent %s: %w", agentName, err)
	}

	// Apply inline env vars and args from task if source is "task"
	if source == "task" && c.taskAgentConfig != nil {
		agentInst = applyAgentEnv(agentInst, c.taskAgentConfig.Env)
		if len(c.taskAgentConfig.Args) > 0 {
			agentInst = agentInst.WithArgs(c.taskAgentConfig.Args...)
		}
	}

	return agentInst, source, nil
}

// AgentResolution holds the result of agent resolution for a specific step.
//
// Deprecated: Use coordination.Resolution instead.
type AgentResolution struct {
	Agent     agent.Agent
	Source    string            // Where it was resolved from
	StepName  string            // Which step this is for
	InlineEnv map[string]string // Resolved inline env vars
	Args      []string          // CLI args for this step
}

// resolveAgentForStep resolves the agent for a specific workflow step.
// Priority: CLI step-specific > CLI global > Task step > Task default > Workspace step > Workspace default > Auto.
func (c *Conductor) resolveAgentForStep(ctx context.Context, step workflow.Step) (*AgentResolution, error) {
	// Use the coordination package for cleaner resolution
	resolver := coordination.NewResolver(c.agents, c.workspace)

	req := coordination.ResolveRequest{
		CLIAgent:       c.opts.AgentName,
		CLISStepAgents: c.opts.StepAgents,
		TaskConfig:     c.taskAgentConfig,
		Step:           step,
	}

	resolution, err := resolver.ResolveForStep(ctx, req)
	if err != nil {
		return nil, err
	}

	// Convert to legacy AgentResolution format
	return &AgentResolution{
		Agent:     resolution.Agent,
		Source:    resolution.Source,
		StepName:  resolution.StepName,
		InlineEnv: resolution.InlineEnv,
		Args:      resolution.Args,
	}, nil
}

// GetAgentForStep returns the resolved agent for a step, using cached resolution if available.
// It also persists the resolution in taskWork for task resumption.
func (c *Conductor) GetAgentForStep(ctx context.Context, step workflow.Step) (agent.Agent, error) {
	stepStr := step.String()

	// Check if we have a cached resolution for this step in taskWork
	if c.taskWork != nil && c.taskWork.Agent.Steps != nil {
		if stepInfo, ok := c.taskWork.Agent.Steps[stepStr]; ok && stepInfo.Name != "" {
			// Restore from persisted config
			agentInst, err := c.agents.Get(stepInfo.Name)
			if err == nil {
				// Re-apply inline env
				agentInst = applyAgentEnv(agentInst, stepInfo.InlineEnv)
				// Re-apply args
				if len(stepInfo.Args) > 0 {
					agentInst = agentInst.WithArgs(stepInfo.Args...)
				}

				return agentInst, nil
			}
			// Fall through to re-resolve if stored agent not found
		}
	}

	// Resolve fresh
	resolution, err := c.resolveAgentForStep(ctx, step)
	if err != nil {
		return nil, err
	}

	// Cache the resolution in taskWork for persistence
	if c.taskWork != nil {
		if c.taskWork.Agent.Steps == nil {
			c.taskWork.Agent.Steps = make(map[string]storage.StepAgentInfo)
		}
		c.taskWork.Agent.Steps[stepStr] = storage.StepAgentInfo{
			Name:      resolution.Agent.Name(),
			Source:    resolution.Source,
			InlineEnv: resolution.InlineEnv,
			Args:      resolution.Args,
		}
		// Save updated work.yaml
		if err := c.workspace.SaveWork(c.taskWork); err != nil {
			// Log but don't fail - agent resolution succeeded, persistence failed
			slog.Warn("failed to save agent info to work.yaml", "error", err)
		}
	}

	return resolution.Agent, nil
}

// registerAliasAgents registers user-defined agent aliases from workspace config.
// Aliases can extend built-in agents or other aliases (chained).
func (c *Conductor) registerAliasAgents(cfg *storage.WorkspaceConfig) error {
	if len(cfg.Agents) == 0 {
		return nil
	}

	// Track resolved aliases to handle chained aliases via topological sort
	resolved := make(map[string]bool)
	// Track aliases currently being resolved to detect circular dependencies
	resolving := make(map[string]bool)

	var resolve func(name string) error
	resolve = func(name string) error {
		if resolved[name] {
			return nil
		}

		if resolving[name] {
			return fmt.Errorf("circular alias dependency detected: %s", name)
		}

		alias, ok := cfg.Agents[name]
		if !ok {
			return nil // Not an alias, skip
		}

		resolving[name] = true

		// Check if base agent exists in registry
		if _, err := c.agents.Get(alias.Extends); err != nil {
			// Base agent not found - might be another alias, try to resolve it first
			if _, isAlias := cfg.Agents[alias.Extends]; isAlias {
				if err := resolve(alias.Extends); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("alias %q extends unknown agent %q", name, alias.Extends)
			}
		}

		// Get the base agent (now guaranteed to exist)
		base, err := c.agents.Get(alias.Extends)
		if err != nil {
			return fmt.Errorf("get base agent for alias %q: %w", name, err)
		}

		// Resolve environment variable references
		env := agent.ResolveEnvReferences(alias.Env)

		// Create and register the alias agent
		aliasAgent := agent.NewAlias(name, base, env, alias.Args, alias.Description)
		if err := c.agents.Register(aliasAgent); err != nil {
			return fmt.Errorf("register alias %q: %w", name, err)
		}

		resolved[name] = true
		resolving[name] = false

		return nil
	}

	// Resolve all aliases
	for name := range cfg.Agents {
		if err := resolve(name); err != nil {
			return err
		}
	}

	return nil
}

// loadPlugins discovers and loads enabled plugins.
func (c *Conductor) loadPlugins(ctx context.Context, cfg *storage.WorkspaceConfig) error {
	// Skip if no plugins are enabled
	if len(cfg.Plugins.Enabled) == 0 {
		return nil
	}

	// Get plugin directories
	globalDir, err := plugin.DefaultGlobalDir()
	if err != nil {
		return fmt.Errorf("get global plugins dir: %w", err)
	}
	projectDir := plugin.DefaultProjectDir(c.workspace.Root())

	// Create plugin discovery and registry
	discovery := plugin.NewDiscovery(globalDir, projectDir)
	c.plugins = plugin.NewRegistry(discovery)

	// Configure enabled plugins
	c.plugins.SetEnabled(cfg.Plugins.Enabled)
	c.plugins.SetConfig(cfg.Plugins.Config)

	// Discover and load plugins
	if err := c.plugins.DiscoverAndLoad(ctx); err != nil {
		return fmt.Errorf("discover and load plugins: %w", err)
	}

	// Register provider plugins
	for _, info := range c.plugins.Providers() {
		if info.Process == nil {
			continue
		}

		adapter := plugin.NewProviderAdapter(info.Manifest, info.Process)
		providerInfo := provider.ProviderInfo{
			Name:         info.Manifest.Provider.Name,
			Description:  info.Manifest.Description,
			Schemes:      info.Manifest.Provider.Schemes,
			Priority:     info.Manifest.Provider.Priority,
			Capabilities: adapter.Capabilities(),
		}

		// Register the provider
		if err := c.providers.Register(providerInfo, func(ctx context.Context, cfg provider.Config) (any, error) {
			return adapter, nil
		}); err != nil {
			// Log but continue - don't fail if one plugin can't register
			continue
		}
	}

	// Register agent plugins
	for _, info := range c.plugins.Agents() {
		if info.Process == nil {
			continue
		}

		adapter := plugin.NewAgentAdapter(info.Manifest, info.Process)
		if err := c.agents.Register(adapter); err != nil {
			// Log but continue
			continue
		}
	}

	// Register workflow plugins (phases, guards, effects)
	workflowPlugins := c.plugins.Workflows()
	if len(workflowPlugins) > 0 {
		// Build a new machine with plugin extensions
		builder := workflow.NewMachineBuilder()

		for _, info := range workflowPlugins {
			if info.Process == nil {
				continue
			}

			adapter := plugin.NewWorkflowAdapter(info.Manifest, info.Process)

			// Initialize adapter with plugin-specific config
			pluginCfg := cfg.Plugins.Config[info.Manifest.Name]
			if err := adapter.Initialize(ctx, pluginCfg); err != nil {
				// Log warning but continue - don't fail if one plugin can't initialize
				continue
			}

			// Store adapter for lifecycle management
			c.workflowAdapters = append(c.workflowAdapters, adapter)

			// Register phases with the machine builder
			for _, phase := range adapter.BuildPhaseDefinitions() {
				if err := builder.RegisterPhase(phase); err != nil {
					// Log warning but continue
					continue
				}
			}
		}

		// Replace the default machine with the configured one
		c.machine = builder.Build(c.eventBus)
	}

	return nil
}

// GetPluginRegistry returns the plugin registry.
func (c *Conductor) GetPluginRegistry() *plugin.Registry {
	return c.plugins
}
