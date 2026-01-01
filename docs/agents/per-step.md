# Per-Step Agent Configuration

> **⚠️ Third-Party Integration**: This integration depends on external APIs that may change. Not fully tested beyond unit tests. Behavior may vary depending on the third-party service. Manual validation recommended before production use.


Different workflow steps can use different agents. This is useful for optimizing cost and performance - use a more capable agent for complex planning, and a faster/cheaper agent for implementation.

## Workflow Steps

| Step            | Command          | Description                 |
| --------------- | ---------------- | --------------------------- |
| `planning`      | `mehr plan`      | Requirement analysis, specs |
| `implementing`  | `mehr implement` | Code generation             |
| `reviewing`     | `mehr review`    | Code review (agent-based)   |
| `checkpointing` | (internal)       | Checkpoint summaries        |

## Workspace Configuration

Configure per-step agents in `.mehrhof/config.yaml`:

```yaml
agent:
  default: claude # Default for all steps
  steps:
    planning:
      name: claude # Use for planning (more capable)
    implementing:
      name: claude # Use for implementation
    reviewing:
      name: claude # Use for review
```

## Task Frontmatter

Override per-step agents in individual task files:

```yaml
---
title: Complex OAuth2 Implementation
agent: claude # Task default
agent_steps:
  planning:
    agent: claude # Use for this task's planning
    env:
      MAX_TOKENS: "16384"
    args: ["--max-turns", "15"] # CLI args for planning only
  implementing:
    agent: glm-fast # Use custom alias for implementation
---
```

## CLI Flags

Override per-step agents at runtime:

```bash
# Override specific steps
mehr start --agent-planning claude file:task.md
mehr plan --agent-planning claude
mehr implement --agent-implementing claude

# --agent still overrides ALL steps
mehr start --agent claude file:task.md  # Uses claude for everything
```

Available CLI flags by command:

| Command          | Flag                   | Description         |
| ---------------- | ---------------------- | ------------------- |
| `mehr start`     | `--agent-planning`     | Planning step agent |
|                  | `--agent-implementing` | Implementation step |
|                  | `--agent-reviewing`    | Review step         |
| `mehr plan`      | `--agent-planning`     | Planning step agent |
| `mehr implement` | `--agent-implementing` | Implementation step |
| `mehr review`    | `--agent-reviewing`    | Review step agent   |

## Per-Step Priority Resolution

For each workflow step, the agent is resolved with this priority (highest to lowest):

1. **CLI step-specific** - `--agent-planning`, `--agent-implementing`, etc.
2. **CLI global** - `--agent` overrides all steps
3. **Task frontmatter step** - `agent_steps.planning.agent`
4. **Task frontmatter default** - `agent`
5. **Workspace config step** - `agent.steps.planning.name`
6. **Workspace config default** - `agent.default`
7. **Auto-detect** - First available agent

## Example: Cost Optimization

Use expensive models only where needed:

```yaml
# .mehrhof/config.yaml
agents:
  claude-opus:
    extends: claude
    description: "Claude Opus 4"
    args: ["--model", "claude-opus-4-20250514"]

agent:
  default: claude # Default agent
  steps:
    planning:
      name: claude-opus # Opus for complex analysis
    reviewing:
      name: claude-opus # Opus for thorough review
    implementing:
      name: claude # Use for code generation
```

## Step Agent Persistence

When a task starts, the resolved agents for each step are persisted in `work.yaml`:

```yaml
agent:
  name: claude
  source: workspace
  steps:
    planning:
      name: claude-opus
      source: workspace-step
    implementing:
      name: claude
      source: workspace
```

This ensures consistent agent usage when resuming tasks across sessions.

## Per-Task Agent Configuration

You can also specify which agent to use directly in the task file frontmatter:

```yaml
---
title: My Feature
agent: glm
---
# Feature description...
```

With CLI arguments:

```yaml
---
title: Complex Feature
agent: claude
agent_args: ["--model", "opus", "--max-turns", "20"]
---
# Feature description...
```

With inline environment variables:

```yaml
---
title: My Feature
agent: claude
agent_env:
  ANTHROPIC_API_KEY: "${PROJECT_API_KEY}"
  MAX_TOKENS: "8192"
---
# Feature description...
```

### Agent Selection Priority

Agent selection follows this priority (highest to lowest):

1. **CLI flag** - `--agent` always wins
2. **Task frontmatter** - `agent:` field in task file
3. **Workspace default** - `agent.default` in config.yaml
4. **Auto-detect** - First available agent

```bash
# CLI flag overrides task frontmatter
mehr start --agent opus file:task.md  # Uses opus, not task's agent

# Without flag, uses task's agent
mehr start file:task.md  # Uses agent from frontmatter
```

## Agent Persistence

Once a task starts, the agent choice is persisted in `work.yaml`. Subsequent commands (`plan`, `implement`, `review`) automatically use the same agent:

```bash
mehr start file:task.md  # Agent resolved from frontmatter: glm
mehr plan                # Continues with glm
mehr implement           # Continues with glm
mehr status              # Shows: Agent: glm (from task)
```

## Inline Environment Variables

Use `agent_env` to set environment variables without creating an alias:

```yaml
---
title: Special Task
agent: claude
agent_env:
  ANTHROPIC_API_KEY: "${SPECIAL_KEY}"
  CUSTOM_PROMPT_PREFIX: "Be concise"
---
```

- Variables with `${VAR}` syntax are expanded from system environment
- Inline env vars are stored unresolved and expanded at runtime
- Useful for one-off configurations that don't warrant a permanent alias

## General Settings

Configure agent behavior in `.mehrhof/config.yaml`:

```yaml
agent:
  default: claude
  timeout: 300 # seconds
  max_retries: 3
```

| Setting     | Config Key          | Default       |
| ----------- | ------------------- | ------------- |
| Timeout     | `agent.timeout`     | 300 (seconds) |
| Max Retries | `agent.max_retries` | 3             |
