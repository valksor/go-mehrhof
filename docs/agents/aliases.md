# Agent Aliases

Aliases let you create custom agents that wrap existing agents with specific environment variables and CLI arguments. This is useful for:

- **Multiple API keys** - Different accounts or projects
- **Different models** - Use specific Claude models via CLI flags
- **Different configurations** - Fast vs thorough responses
- **Team sharing** - Share agent configs via the repo

## Defining Aliases

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

## Using Aliases

```bash
# Set the environment variable
export GLM_API_KEY="sk-ant-..."

# Use the alias
mehr start --agent glm file:task.md
mehr auto --agent glm-fast file:task.md
```

## Listing Available Agents

```bash
mehr agents list
```

Output:

```
NAME      TYPE      EXTENDS  AVAILABLE  DESCRIPTION
claude    built-in  -        yes        -
codex     built-in  -        no         -
glm       alias     claude   yes        Claude with GLM API key
```

## How Aliases Work

1. When you use `--agent opus`, Mehrhof finds the alias config
2. It wraps the base agent (`claude`) with the configured env vars and CLI args
3. Environment variable references (`${VAR}`) are expanded at runtime
4. CLI args are passed to the underlying Claude CLI command
5. The wrapped agent handles the actual AI operations

See [mehr agents](../cli/agents.md) for the full CLI reference.
