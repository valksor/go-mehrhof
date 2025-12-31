# AI Agents

Mehrhof orchestrates AI agents to help with planning and implementation. It delegates AI operations to external CLI tools.

## How It Works

Mehrhof doesn't connect to AI APIs directly. Instead, it calls agent CLIs as subprocesses. Whatever configuration you have for the agent will be used automatically.

```
User → mehr plan → Agent CLI → AI Response → Mehrhof processes output
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

### Codex

Codex is an alternative AI agent. Mehrhof calls the `codex` CLI command.

**Prerequisites:**

- Codex CLI installed and configured
- Your Codex settings (API keys, model preferences) already set up

```bash
# Verify Codex works
codex --version
```

### Aider

Aider is a Git-aware AI pair programming assistant. Mehrhof wraps the `aider` CLI for code generation tasks.

**Prerequisites:**

- Aider CLI installed (`pip install aider-chat`)
- API key configured (supports OpenAI, Anthropic, and other providers)

```bash
# Verify Aider works
aider --version
```

**Key Features:**

- Git-aware: Understands repository structure and history
- Multi-file editing: Can modify multiple files in a single session
- Auto-commits disabled: Changes are applied without automatic commits (Mehrhof manages commits)

**Configuration:**

```yaml
# .mehrhof/config.yaml
agents:
  aider-gpt4:
    extends: aider
    description: "Aider with GPT-4"
    args: ["--model", "gpt-4"]

  aider-claude:
    extends: aider
    description: "Aider with Claude"
    args: ["--model", "claude-3-opus-20240229"]
```

### Ollama

Ollama provides local AI inference for privacy and cost savings. Mehrhof wraps the `ollama run` command.

**Prerequisites:**

- Ollama installed and running (`ollama serve`)
- At least one model downloaded (`ollama pull codellama`)

```bash
# Verify Ollama works
ollama --version

# Pull a coding model
ollama pull codellama
```

**Key Features:**

- **Local inference**: No API calls, complete privacy
- **Free to use**: No per-token costs
- **Multiple models**: Support for various open-source models

**Default Model:** `codellama`

**Configuration:**

```yaml
# .mehrhof/config.yaml
agents:
  ollama-llama3:
    extends: ollama
    description: "Ollama with Llama 3"
    args: ["--model", "llama3:70b"]

  ollama-codellama:
    extends: ollama
    description: "Ollama with CodeLlama"
    args: ["--model", "codellama:34b"]

  ollama-deepseek:
    extends: ollama
    description: "Ollama with DeepSeek Coder"
    args: ["--model", "deepseek-coder:33b"]
```

**Popular Models for Coding:**

| Model | Size | Best For |
|-------|------|----------|
| `codellama` | 7B-34B | General code generation |
| `llama3` | 8B-70B | Reasoning and complex tasks |
| `deepseek-coder` | 1.3B-33B | Code completion |
| `mistral` | 7B | Fast inference |
| `mixtral` | 8x7B | High quality, larger context |

### GitHub Copilot

GitHub Copilot agent wraps the `gh copilot` CLI extension for shell command suggestions and code explanations.

**Prerequisites:**

- GitHub CLI installed (`gh`)
- Copilot extension installed (`gh extension install github/gh-copilot`)
- Active GitHub Copilot subscription

```bash
# Verify Copilot works
gh copilot --version
```

**Key Features:**

- **Suggest mode**: Generate shell commands from natural language
- **Explain mode**: Explain what commands do
- **Target types**: Shell, Git, or GitHub CLI commands

**Configuration:**

```yaml
# .mehrhof/config.yaml
agents:
  copilot-shell:
    extends: copilot
    description: "Copilot for shell commands"
    args: ["--target", "shell"]

  copilot-git:
    extends: copilot
    description: "Copilot for git commands"
    args: ["--target", "git"]
```

**Limitations:**

- Cloud-only (requires GitHub subscription)
- Best for command-line tasks, not code generation
- Context sent to GitHub servers

### OpenRouter

OpenRouter provides unified access to 100+ AI models through a single API. Useful for accessing models from OpenAI, Anthropic, Google, Meta, and others.

**Prerequisites:**

- OpenRouter API key (get one at https://openrouter.ai/keys)

```bash
# Set API key
export OPENROUTER_API_KEY="sk-or-..."
```

**Key Features:**

- **Model variety**: Access to Claude, GPT-4, Gemini, Llama, and many more
- **Cost optimization**: Choose models based on price/performance
- **Fallback support**: Configure backup models
- **Streaming**: Full streaming support for real-time responses

**Default Model:** `anthropic/claude-3.5-sonnet`

**Configuration:**

```yaml
# .mehrhof/config.yaml
agents:
  openrouter-gpt4:
    extends: openrouter
    description: "OpenRouter with GPT-4"
    args: ["--model", "openai/gpt-4-turbo"]

  openrouter-gemini:
    extends: openrouter
    description: "OpenRouter with Gemini"
    args: ["--model", "google/gemini-pro-1.5"]

  openrouter-llama:
    extends: openrouter
    description: "OpenRouter with Llama 3.1"
    args: ["--model", "meta-llama/llama-3.1-405b-instruct"]
```

**Popular Models:**

| Model | Provider | Best For |
|-------|----------|----------|
| `anthropic/claude-3.5-sonnet` | Anthropic | General coding |
| `openai/gpt-4-turbo` | OpenAI | Complex reasoning |
| `google/gemini-pro-1.5` | Google | Large context |
| `meta-llama/llama-3.1-405b-instruct` | Meta | Open-source alternative |

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
codex     built-in  -        no         -
glm       alias     claude   yes        Claude with GLM API key
```

### Change Default Agent

Set your preferred agent in workspace config:

```yaml
# .mehrhof/config.yaml
agent:
  default: claude # or an alias name
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
mehr auto --agent glm-fast file:task.md
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
| `checkpointing` | (internal)       | Checkpoint summaries        |

### Workspace Configuration

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

### Task Frontmatter

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

### CLI Flags

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

### Step Agent Persistence

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

### Agent Persistence

Once a task starts, the agent choice is persisted in `work.yaml`. Subsequent commands (`plan`, `implement`, `review`) automatically use the same agent:

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

Increase the timeout in `.mehrhof/config.yaml`:

```yaml
agent:
  timeout: 600 # 10 minutes
```

### "Rate limited"

The agent will retry automatically up to `agent.max_retries` times. If issues persist, wait before retrying.

### Verbose Output

See agent interactions in real-time:

```bash
mehr plan --verbose
mehr implement --verbose
```
