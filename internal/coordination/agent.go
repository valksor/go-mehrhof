// Package coordination provides agent resolution and coordination services.
// It separates the complexity of agent selection from the conductor.
package coordination

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/valksor/go-mehrhof/internal/agent"
	"github.com/valksor/go-mehrhof/internal/provider"
	"github.com/valksor/go-mehrhof/internal/storage"
	"github.com/valksor/go-mehrhof/internal/workflow"
)

// ErrAgentNotFound indicates no agent was found at a priority level.
var ErrAgentNotFound = errors.New("agent not found at this priority")

// Resolver handles agent resolution with multiple priority sources.
type Resolver struct {
	agents    *agent.Registry
	workspace Workspace
	log       *slog.Logger
}

// Workspace provides access to configuration.
type Workspace interface {
	LoadConfig() (*storage.WorkspaceConfig, error)
}

// NewResolver creates a new agent resolver.
func NewResolver(agents *agent.Registry, workspace Workspace) *Resolver {
	return &Resolver{
		agents:    agents,
		workspace: workspace,
		log:       slog.Default(),
	}
}

// Resolution holds the result of agent resolution.
type Resolution struct {
	Agent     agent.Agent
	Source    string            // Where it was resolved from: cli-step, cli, task-step, task, workspace-step, workspace, auto
	StepName  string            // Which step this is for
	InlineEnv map[string]string // Resolved inline env vars
	Args      []string          // CLI args for this step
}

// ResolveRequest contains all inputs for agent resolution.
type ResolveRequest struct {
	// CLI-provided values
	CLIAgent       string                   // --agent flag
	CLISStepAgents map[string]string        // --agent-plan, --agent-implement, etc.
	WorkspaceCfg   *storage.WorkspaceConfig // Workspace config (optional, can be loaded if nil)

	// Task frontmatter values (may be nil)
	TaskConfig *provider.AgentConfig

	// Step to resolve for
	Step workflow.Step
}

// ResolveForStep resolves an agent for a specific workflow step.
// Priority order:
// 1. CLI step-specific flag (--agent-plan)
// 2. CLI global flag (--agent)
// 3. Task frontmatter step-specific (agent_steps.planning.agent)
// 4. Task frontmatter default (agent)
// 5. Workspace config step-specific (agent.steps.planning.name)
// 6. Workspace config default (agent.default)
// 7. Auto-detect.
func (r *Resolver) ResolveForStep(ctx context.Context, req ResolveRequest) (*Resolution, error) {
	stepStr := req.Step.String()

	// Try each priority level
	for priority := 1; priority <= 7; priority++ {
		resolution, err := r.tryResolveAtPriority(ctx, req, priority, stepStr)
		if errors.Is(err, ErrAgentNotFound) {
			// No agent at this priority, try next one
			continue
		}
		if err != nil {
			return nil, err
		}
		if resolution != nil {
			return resolution, nil
		}
	}

	// Should never reach here since priority 7 (auto-detect) should always succeed
	return nil, fmt.Errorf("failed to resolve agent for step %s", req.Step)
}

// tryResolveAtPriority attempts resolution at a specific priority level.
// Returns the resolution if successful, nil if no agent found at this level (continue to next),
// or an error if resolution failed unexpectedly.
func (r *Resolver) tryResolveAtPriority(_ context.Context, req ResolveRequest, priority int, stepStr string) (*Resolution, error) {
	switch priority {
	case 1: // CLI step-specific
		if name, ok := req.CLISStepAgents[stepStr]; ok && name != "" {
			agentInst, err := r.agents.Get(name)
			if err != nil {
				return nil, fmt.Errorf("get agent %s: %w", name, err)
			}

			return &Resolution{
				Agent:    agentInst,
				Source:   "cli-step",
				StepName: stepStr,
			}, nil
		}

	case 2: // CLI global
		if req.CLIAgent != "" {
			agentInst, err := r.agents.Get(req.CLIAgent)
			if err != nil {
				return nil, fmt.Errorf("get agent %s: %w", req.CLIAgent, err)
			}

			return &Resolution{
				Agent:    agentInst,
				Source:   "cli",
				StepName: stepStr,
			}, nil
		}

	case 3: // Task frontmatter step-specific
		if req.TaskConfig != nil && req.TaskConfig.Steps != nil {
			if stepCfg, ok := req.TaskConfig.Steps[stepStr]; ok && stepCfg.Name != "" {
				agentInst, err := r.agents.Get(stepCfg.Name)
				if err != nil {
					return nil, fmt.Errorf("get agent %s: %w", stepCfg.Name, err)
				}

				return &Resolution{
					Agent:     agentInst,
					Source:    "task-step",
					StepName:  stepStr,
					InlineEnv: stepCfg.Env,
					Args:      stepCfg.Args,
				}, nil
			}
		}

	case 4: // Task frontmatter default
		if req.TaskConfig != nil && req.TaskConfig.Name != "" {
			agentInst, err := r.agents.Get(req.TaskConfig.Name)
			if err != nil {
				return nil, fmt.Errorf("get agent %s: %w", req.TaskConfig.Name, err)
			}

			return &Resolution{
				Agent:     agentInst,
				Source:    "task",
				StepName:  stepStr,
				InlineEnv: req.TaskConfig.Env,
				Args:      req.TaskConfig.Args,
			}, nil
		}

	case 5, 6: // Workspace config
		cfg := req.WorkspaceCfg
		if cfg == nil {
			var err error
			cfg, err = r.workspace.LoadConfig()
			if err != nil {
				// No workspace config available, continue to next priority
				return nil, ErrAgentNotFound
			}
		}
		if cfg != nil {
			if priority == 5 {
				// Workspace step-specific
				if stepCfg, ok := cfg.Agent.Steps[stepStr]; ok && stepCfg.Name != "" {
					agentInst, err := r.agents.Get(stepCfg.Name)
					if err != nil {
						// Workspace config agent not found - log warning but continue
						r.log.Warn("Workspace step agent not found", "step", stepStr, "agent", stepCfg.Name, "error", err)

						return nil, ErrAgentNotFound
					}

					return &Resolution{
						Agent:     agentInst,
						Source:    "workspace-step",
						StepName:  stepStr,
						InlineEnv: stepCfg.Env,
						Args:      stepCfg.Args,
					}, nil
				}
			} else {
				// Workspace default
				if cfg.Agent.Default != "" {
					agentInst, err := r.agents.Get(cfg.Agent.Default)
					if err != nil {
						// Workspace default agent not found - log warning but continue
						r.log.Warn("Workspace default agent not found", "agent", cfg.Agent.Default, "error", err)

						return nil, ErrAgentNotFound
					}

					return &Resolution{
						Agent:    agentInst,
						Source:   "workspace",
						StepName: stepStr,
					}, nil
				}
			}
		}

	case 7: // Auto-detect
		agentInst, err := r.agents.Detect()
		if err != nil {
			return nil, fmt.Errorf("detect agent: %w", err)
		}

		return &Resolution{
			Agent:    agentInst,
			Source:   "auto",
			StepName: stepStr,
		}, nil
	}

	return nil, ErrAgentNotFound
}

// ApplyEnvs applies environment variables to an agent instance.
func ApplyEnvs(agentInst agent.Agent, env map[string]string) agent.Agent {
	if len(env) == 0 {
		return agentInst
	}
	resolvedEnv := agent.ResolveEnvReferences(env)
	for k, v := range resolvedEnv {
		agentInst = agentInst.WithEnv(k, v)
	}

	return agentInst
}

// ApplyArgs applies CLI arguments to an agent instance.
func ApplyArgs(agentInst agent.Agent, args ...string) agent.Agent {
	if len(args) == 0 {
		return agentInst
	}

	return agentInst.WithArgs(args...)
}
