# mehr agents

Manage AI agents including built-in agents and user-defined aliases.

## Commands

### mehr agents list

List all registered agents.

```bash
mehr agents list
```

**Output:**

```
NAME      TYPE      EXTENDS  AVAILABLE  DESCRIPTION
claude    built-in  -        yes        -
codex     built-in  -        no         -
glm       alias     claude   yes        Claude with GLM API key
glm-fast  alias     glm      yes        GLM with lower token limit
```

**Columns:**
| Column | Description |
|--------|-------------|
| NAME | Agent identifier (use with `--agent` flag) |
| TYPE | `built-in` or `alias` |
| EXTENDS | Base agent for aliases |
| AVAILABLE | Whether the agent is available (CLI installed, etc.) |
| DESCRIPTION | Human-readable description |

## Configuring Agent Aliases

Aliases are defined in `.mehrhof/config.yaml`:

```yaml
agents:
  glm:
    extends: claude # Required: base agent to wrap
    description: "Claude with GLM API key" # Optional: shown in list
    env: # Optional: environment variables
      ANTHROPIC_API_KEY: "${GLM_API_KEY}" # ${VAR} references system env vars
      CUSTOM_HEADER: "X-GLM-Request"

  glm-fast:
    extends: glm # Aliases can extend other aliases
    description: "GLM with lower tokens"
    env:
      MAX_TOKENS: "2048"
```

### Configuration Fields

| Field         | Required | Description                                            |
| ------------- | -------- | ------------------------------------------------------ |
| `extends`     | Yes      | Name of the agent to wrap (built-in or another alias)  |
| `description` | No       | Human-readable description shown in `mehr agents list` |
| `env`         | No       | Environment variables passed to the agent              |

### Environment Variable Expansion

Values in `env` support shell variable expansion:

```yaml
env:
  ANTHROPIC_API_KEY: "${GLM_API_KEY}" # Expands $GLM_API_KEY
  MAX_TOKENS: "${MAX_TOKENS:-4096}" # With default (shell syntax)
  LITERAL_VALUE: "sk-ant-123" # Literal value (no expansion)
```

## Using Aliases

### At Task Start

```bash
mehr start --agent glm file:task.md
```

### In Auto Mode

```bash
mehr auto --agent glm file:task.md
```

### Change Default Agent

Set in workspace config:

```yaml
# .mehrhof/config.yaml
agent:
  default: glm # Use alias as default
```

## Common Patterns

### Multiple API Keys

Use aliases for different API keys or accounts:

```yaml
agents:
  work-claude:
    extends: claude
    description: "Work account"
    env:
      ANTHROPIC_API_KEY: "${WORK_CLAUDE_KEY}"

  personal-claude:
    extends: claude
    description: "Personal account"
    env:
      ANTHROPIC_API_KEY: "${PERSONAL_CLAUDE_KEY}"
```

### Different Configurations

Configure different model parameters:

```yaml
agents:
  claude-fast:
    extends: claude
    description: "Fast responses"
    env:
      MAX_TOKENS: "2048"

  claude-thorough:
    extends: claude
    description: "Detailed responses"
    env:
      MAX_TOKENS: "16000"
```

### Chained Aliases

Build on top of other aliases:

```yaml
agents:
  glm:
    extends: claude
    env:
      ANTHROPIC_API_KEY: "${GLM_API_KEY}"

  glm-fast:
    extends: glm # Inherits GLM's API key
    env:
      MAX_TOKENS: "2048"

  glm-thorough:
    extends: glm # Inherits GLM's API key
    env:
      MAX_TOKENS: "16000"
```

## Troubleshooting

### "alias extends unknown agent"

The `extends` field references an agent that doesn't exist:

```
Error: alias "foo" extends unknown agent "bar"
```

Fix: Ensure the base agent name is correct (check with `mehr agents list`).

### "circular alias dependency"

Aliases cannot extend themselves (directly or indirectly):

```
Error: circular alias dependency detected: a
```

Fix: Review your alias chain and break the cycle.

### Alias Not Available

If an alias shows `AVAILABLE: no`, check that:

1. The base agent is available (`claude` CLI installed)
2. Any required environment variables are set

```bash
claude --version
```

## See Also

- [AI Agents](../agents/index.md) - How agents work, aliases, and per-step configuration
- [Configuration Guide](../configuration/index.md) - Config file reference
