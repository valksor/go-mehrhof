# AI Agents

> **Integration Notice**: Agent implementations depend on third-party APIs that may change. Manual testing recommended before production use.

Mehrhof orchestrates AI agents to help with planning and implementation. It delegates AI operations to external CLI tools.

## How It Works

Mehrhof doesn't connect to AI APIs directly. It calls agent CLIs as subprocesses:

```
User → mehr plan → Agent CLI → AI Response → Mehrhof processes output
```

## Available Agents

| Agent | Description |
|-------|-------------|
| [Claude](claude.md) | Anthropic's Claude AI (Default) |
| [Gemini](gemini.md) | Google Gemini with 1M token context |
| [Aider](aider.md) | Git-aware AI pair programming |
| [Ollama](ollama.md) | Local AI inference |
| [Copilot](copilot.md) | GitHub Copilot integration |
| [OpenRouter](openrouter.md) | Access to 100+ AI models |
| [Codex](codex.md) | Alternative AI agent |

## Basic Configuration

Set the default agent in `.mehrhof/config.yaml`:

```yaml
agent:
  default: claude
  timeout: 300
  max_retries: 3
```

Or specify via CLI:

```bash
mehr start --agent claude file:task.md
```

---

## Agent Aliases

Aliases let you create custom agents with specific environment variables and CLI arguments.

### Defining Aliases

```yaml
# .mehrhof/config.yaml
agents:
  opus:
    extends: claude
    description: "Claude Opus model"
    args: ["--model", "claude-opus-4-20250514"]

  sonnet-fast:
    extends: claude
    description: "Sonnet with limited turns"
    args: ["--model", "claude-sonnet-4-20250514", "--max-turns", "3"]

  glm:
    extends: claude
    description: "Claude with GLM API key"
    env:
      ANTHROPIC_API_KEY: "${GLM_API_KEY}"  # Expands from .env or system env

  glm-opus:
    extends: glm  # Aliases can extend aliases
    description: "GLM with Opus model"
    args: ["--model", "claude-opus-4-20250514"]
```

### Using Aliases

```bash
export GLM_API_KEY="sk-ant-..."
mehr start --agent glm file:task.md
mehr auto --agent opus file:task.md
```

### Listing Agents

```bash
mehr agents list
```

```
NAME      TYPE      EXTENDS  AVAILABLE  DESCRIPTION
claude    built-in  -        yes        -
glm       alias     claude   yes        Claude with GLM API key
opus      alias     claude   yes        Claude Opus model
```

---

## Per-Step Agent Configuration

Different workflow steps can use different agents - use a capable agent for planning, a faster one for implementation.

### Workflow Steps

| Step | Command | Description |
|------|---------|-------------|
| `planning` | `mehr plan` | Requirement analysis, specifications |
| `implementing` | `mehr implement` | Code generation |
| `reviewing` | `mehr review` | Code review |
| `checkpointing` | (internal) | Checkpoint summaries |

### Workspace Configuration

```yaml
# .mehrhof/config.yaml
agent:
  default: claude
  steps:
    planning:
      name: claude-opus  # Expensive model for complex analysis
    implementing:
      name: claude       # Standard for code generation
    reviewing:
      name: claude-opus  # Thorough review
```

### Task Frontmatter

Override per-step agents in task files:

```yaml
---
title: Complex OAuth2 Implementation
agent: claude
agent_steps:
  planning:
    agent: claude-opus
    env:
      MAX_TOKENS: "16384"
    args: ["--max-turns", "15"]
  implementing:
    agent: sonnet-fast
---
```

### CLI Flags

```bash
mehr plan --agent-planning claude-opus
mehr implement --agent-implementing sonnet-fast
mehr review --agent-reviewing claude-opus

# --agent overrides ALL steps
mehr start --agent claude file:task.md
```

### Priority Resolution

For each step, agent is resolved (highest to lowest):

1. CLI step-specific (`--agent-planning`)
2. CLI global (`--agent`)
3. Task frontmatter step (`agent_steps.planning.agent`)
4. Task frontmatter default (`agent`)
5. Workspace config step (`agent.steps.planning.name`)
6. Workspace config default (`agent.default`)
7. Auto-detect

---

## Per-Task Agent Configuration

Specify agents directly in task frontmatter:

```yaml
---
title: My Feature
agent: glm
agent_args: ["--max-turns", "20"]
agent_env:
  ANTHROPIC_API_KEY: "${PROJECT_API_KEY}"
---
```

### Task Agent Priority

1. CLI flag (`--agent`) - always wins
2. Task frontmatter (`agent:`)
3. Workspace default (`agent.default`)
4. Auto-detect

### Agent Persistence

Once a task starts, the agent is persisted in `work.yaml`:

```bash
mehr start file:task.md  # Agent resolved: glm
mehr plan                # Continues with glm
mehr implement           # Continues with glm
```

---

## How Agents Work

### Planning Phase (`mehr plan`)

1. Receives task source content
2. Reads any existing notes
3. Analyzes requirements
4. Generates SPEC files with implementation details

### Implementation Phase (`mehr implement`)

1. Reads all SPEC files
2. Considers notes and context
3. Generates or modifies code files
4. Provides summary of changes

### Agent Output Format

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

---

## Session Logging

All agent interactions are logged:

```
.mehrhof/work/<id>/sessions/
├── 2025-01-15T10-30-00-planning.yaml
├── 2025-01-15T11-00-00-talk.yaml
└── 2025-01-15T11-30-00-implementing.yaml
```

Each session includes timestamps, message history, token usage, and cost tracking.

---

## Troubleshooting

### Claude Not Working

```bash
claude --version  # Check installed
claude "Hello"    # Test works
```

### Agent Timeout

Increase timeout in config:

```yaml
agent:
  timeout: 600  # 10 minutes
```

### Rate Limited

Agent retries automatically up to `max_retries` times. Wait before retrying if issues persist.

### Verbose Output

```bash
mehr plan --verbose
mehr implement --verbose
```

## See Also

- [mehr agents](../cli/agents.md) - CLI reference
- [Configuration Guide](../configuration/index.md) - Full config options
