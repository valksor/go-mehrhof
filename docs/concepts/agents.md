# AI Agents

Mehrhof orchestrates AI agents to help with planning and implementation. It delegates AI operations to external CLI tools - primarily Claude CLI.

## How It Works

Mehrhof doesn't connect to AI APIs directly. Instead, it calls Claude CLI as a subprocess. Whatever configuration you have for Claude will be used automatically.

```
User → mehr plan → Claude CLI → AI Response → Mehrhof processes output
```

## Available Agents

### Claude (Default)

Claude is the default AI agent. Mehrhof calls the `claude` CLI command.

**Prerequisites:**

- Claude CLI installed and configured
- Your Claude settings (API keys, model preferences) already set up

```bash
# Verify Claude works
claude --version
```

## Selecting an Agent

### At Start Time

Specify the agent when starting a task:

```bash
mehr start --agent claude file:task.md
mehr start --agent glm file:task.md    # Use an alias
```

### List Available Agents

```bash
mehr agents list
```

Output:

```
NAME      TYPE      EXTENDS  AVAILABLE  DESCRIPTION
claude    built-in  -        yes        -
glm       alias     claude   yes        Claude with GLM API key
```

### Change Default Agent

Set your preferred agent in workspace config:

```yaml
# .mehrhof/config.yaml
agent:
  default: claude # or an alias name
```

Or via environment variable:

```bash
export MEHR_AGENT_DEFAULT=claude
```

## Agent Aliases

Aliases let you create custom agents that wrap existing agents with specific environment variables and CLI arguments. This is useful for:

- **Multiple API keys** - Different accounts or projects
- **Different models** - Use specific Claude models via CLI flags
- **Different configurations** - Fast vs thorough responses
- **Team sharing** - Share agent configs via the repo

### Defining Aliases

Add aliases to `.mehrhof/config.yaml`:

```yaml
agents:
  opus:
    extends: claude
    description: "Claude Opus model"
    args: ["--model", "claude-opus-4-20250514"] # CLI flags

  sonnet-fast:
    extends: claude
    description: "Claude Sonnet with limited turns"
    args: ["--model", "claude-sonnet-4-20250514", "--max-turns", "3"]

  glm:
    extends: claude # Base agent to wrap
    description: "Claude with GLM API key" # Shown in list
    env:
      ANTHROPIC_API_KEY: "${GLM_API_KEY}" # ${VAR} expands system env

  glm-opus:
    extends: glm # Aliases can extend aliases
    description: "GLM with Opus model"
    args: ["--model", "claude-opus-4-20250514"]
```

### Using Aliases

```bash
# Set the environment variable
export GLM_API_KEY="sk-ant-..."

# Use the alias
mehr start --agent glm file:task.md
mehr yolo --agent glm-fast file:task.md
```

### How Aliases Work

1. When you use `--agent opus`, Mehrhof finds the alias config
2. It wraps the base agent (`claude`) with the configured env vars and CLI args
3. Environment variable references (`${VAR}`) are expanded at runtime
4. CLI args are passed to the underlying Claude CLI command
5. The wrapped agent handles the actual AI operations

See [mehr agents](../cli/agents.md) for the full CLI reference.

## Per-Task Agent Configuration

You can specify which agent to use directly in the task file frontmatter. This is useful when different tasks need different agents or configurations.

### Task Frontmatter

Add `agent` and optionally `agent_env` or `agent_args` to your task file:

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

### Priority Order

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

## Per-Step Agent Configuration

Different workflow steps can use different agents. This is useful for optimizing cost and performance - use a more capable agent for complex planning, and a faster/cheaper agent for implementation.

### Workflow Steps

| Step            | Command          | Description                 |
| --------------- | ---------------- | --------------------------- |
| `planning`      | `mehr plan`      | Requirement analysis, specs |
| `implementing`  | `mehr implement` | Code generation             |
| `reviewing`     | `mehr review`    | Code review (agent-based)   |
| `dialogue`      | `mehr talk`      | Interactive conversation    |
| `checkpointing` | (internal)       | Checkpoint summaries        |

### Workspace Configuration

Configure per-step agents in `.mehrhof/config.yaml`:

```yaml
agent:
  default: claude-sonnet # Default for all steps
  steps:
    planning:
      name: claude # Use Opus for planning (more capable)
    implementing:
      name: claude-sonnet # Use Sonnet for implementation (faster)
    reviewing:
      name: claude # Use Opus for review
```

### Task Frontmatter

Override per-step agents in individual task files:

```yaml
---
title: Complex OAuth2 Implementation
agent: claude-sonnet # Task default
agent_steps:
  planning:
    agent: claude # Use Opus for this task's planning
    env:
      MAX_TOKENS: "16384"
    args: ["--max-turns", "15"] # CLI args for planning only
  implementing:
    agent: glm-fast # Use custom alias for implementation
---
```

### CLI Flags

Override per-step agents at runtime:

```bash
# Override specific steps
mehr start --agent-planning claude file:task.md
mehr plan --agent-planning claude
mehr implement --agent-implementing claude-sonnet
mehr talk --agent-dialogue claude

# --agent still overrides ALL steps
mehr start --agent claude file:task.md  # Uses claude for everything
```

Available CLI flags by command:

| Command          | Flag                   | Description         |
| ---------------- | ---------------------- | ------------------- |
| `mehr start`     | `--agent-planning`     | Planning step agent |
|                  | `--agent-implementing` | Implementation step |
|                  | `--agent-reviewing`    | Review step         |
|                  | `--agent-dialogue`     | Dialogue step       |
| `mehr plan`      | `--agent-planning`     | Planning step agent |
| `mehr implement` | `--agent-implementing` | Implementation step |
| `mehr review`    | `--agent-reviewing`    | Review step agent   |
| `mehr talk`      | `--agent-dialogue`     | Dialogue step agent |

### Per-Step Priority Resolution

For each workflow step, the agent is resolved with this priority (highest to lowest):

1. **CLI step-specific** - `--agent-planning`, `--agent-implementing`, etc.
2. **CLI global** - `--agent` overrides all steps
3. **Task frontmatter step** - `agent_steps.planning.agent`
4. **Task frontmatter default** - `agent`
5. **Workspace config step** - `agent.steps.planning.name`
6. **Workspace config default** - `agent.default`
7. **Auto-detect** - First available agent

### Example: Cost Optimization

Use expensive models only where needed:

```yaml
# .mehrhof/config.yaml
agents:
  claude-opus:
    extends: claude
    description: "Claude Opus 4"
    args: ["--model", "claude-opus-4-20250514"]

  claude-sonnet:
    extends: claude
    description: "Claude Sonnet 4"
    args: ["--model", "claude-sonnet-4-20250514"]

agent:
  default: claude-sonnet # Fast/cheap default
  steps:
    planning:
      name: claude-opus # Opus for complex analysis
    reviewing:
      name: claude-opus # Opus for thorough review
    implementing:
      name: claude-sonnet # Sonnet for code generation
    dialogue:
      name: claude-sonnet # Sonnet for quick interactions
```

### Step Agent Persistence

When a task starts, the resolved agents for each step are persisted in `work.yaml`:

```yaml
agent:
  name: claude-sonnet
  source: workspace
  steps:
    planning:
      name: claude-opus
      source: workspace-step
    implementing:
      name: claude-sonnet
      source: workspace
```

This ensures consistent agent usage when resuming tasks across sessions.

### Agent Persistence

Once a task starts, the agent choice is persisted in `work.yaml`. Subsequent commands (`plan`, `implement`, `talk`) automatically use the same agent:

```bash
mehr start file:task.md  # Agent resolved from frontmatter: glm
mehr plan                # Continues with glm
mehr implement           # Continues with glm
mehr status              # Shows: Agent: glm (from task)
```

### Inline Environment Variables

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

## Agent Configuration

### General Settings

| Setting     | Environment Variable    | Default       |
| ----------- | ----------------------- | ------------- |
| Timeout     | `MEHR_AGENT_TIMEOUT`    | 300 (seconds) |
| Max Retries | `MEHR_AGENT_MAXRETRIES` | 3             |

## How Agents Work

### Planning Phase

During `mehr plan`, the agent:

1. Receives the task source content
2. Reads any existing notes
3. Analyzes the requirements
4. Generates SPEC files with implementation details

### Implementation Phase

During `mehr implement`, the agent:

1. Reads all SPEC files
2. Considers notes and context
3. Generates or modifies code files
4. Provides a summary of changes

### Talk Mode

During `mehr talk`, the agent:

1. Maintains conversation context
2. Answers questions about the task
3. Accepts clarifications and notes
4. Updates the understanding for future phases

## Agent Output

Agents produce structured output:

```
<<FILE:path/to/file.go>>
package main

func main() {
    // Generated code
}
<<END FILE>>

<<SUMMARY>>
Created main.go with basic structure.
<<END SUMMARY>>
```

This format allows Mehrhof to:

- Extract file changes
- Apply modifications safely
- Track what was generated

## Session Logging

All agent interactions are logged:

```
.mehrhof/work/<id>/sessions/
├── 2025-01-15T10-30-00-planning.yaml
├── 2025-01-15T11-00-00-talk.yaml
└── 2025-01-15T11-30-00-implementing.yaml
```

Each session includes:

- Timestamps
- Message history
- Token usage
- Cost tracking

## Troubleshooting

### Claude Not Working

Ensure Claude CLI is properly installed and configured:

```bash
# Check Claude is available
claude --version

# Test Claude works
claude "Hello"
```

If Claude has issues, fix them in your Claude CLI configuration first.

### "Agent timeout"

Increase the timeout:

```bash
export MEHR_AGENT_TIMEOUT=600  # 10 minutes
```

### "Rate limited"

The agent will retry automatically up to `MEHR_AGENT_MAXRETRIES` times. If issues persist, wait before retrying.

### Verbose Output

See agent interactions in real-time:

```bash
mehr plan --verbose
mehr implement --verbose
```
