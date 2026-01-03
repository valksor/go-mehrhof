# Configuration Guide

Mehrhof uses a simple configuration system with sensible defaults.

## How Claude Integration Works

Mehrhof calls Claude CLI as a subprocess:

- **Claude's settings** (API keys, model) are handled by Claude CLI itself
- **Mehrhof's settings** control workflow behavior and are passed to agents
- Environment variables in workspace config are passed to Claude when it runs

## Configuration Methods

| Method | Use Case | Location |
|--------|----------|----------|
| CLI flags | Per-command overrides | `mehr --verbose plan` |
| Workspace config | Project settings | `.mehrhof/config.yaml` |
| Environment file | Secrets (gitignored) | `.mehrhof/.env` |
| User settings | Personal preferences | `~/.mehrhof/settings.json` |

## File Locations

| File | Purpose |
|------|---------|
| `.mehrhof/config.yaml` | Workspace configuration |
| `.mehrhof/.env` | Secrets (gitignored) |
| `.mehrhof/.active_task` | Current task (managed) |
| `~/.mehrhof/settings.json` | User preferences |
| `~/.mehrhof/plugins/` | Global plugins |

## Workspace Configuration

**Location:** `.mehrhof/config.yaml`

```yaml
git:
  auto_commit: true
  commit_prefix: "[{key}]"
  branch_pattern: "{type}/{key}--{slug}"
  sign_commits: false

agent:
  default: claude
  timeout: 300
  max_retries: 3

providers:
  default: file  # Allow bare references like "task.md"

workflow:
  auto_init: true
  session_retention_days: 30

env:
  CLAUDE_ANTHROPIC_API_KEY: "${ANTHROPIC_API_KEY}"
```

### git

Controls version control integration:

| Setting | Default | Description |
|---------|---------|-------------|
| `auto_commit` | `true` | Auto-commit after operations |
| `commit_prefix` | `[{key}]` | Commit message prefix template |
| `branch_pattern` | `{type}/{key}--{slug}` | Branch naming template |
| `sign_commits` | `false` | GPG-sign commits |

**Template variables:**

| Variable | Description | Example |
|----------|-------------|---------|
| `{key}` | External key (from filename/frontmatter) | `FEATURE-123` |
| `{task_id}` | Internal task ID | `a1b2c3d4` |
| `{type}` | Task type from filename prefix | `feature`, `fix` |
| `{slug}` | URL-safe slugified title | `add-user-auth` |

### agent

Controls AI agent behavior:

| Setting | Default | Description |
|---------|---------|-------------|
| `default` | `claude` | Default agent |
| `timeout` | `300` | Timeout in seconds |
| `max_retries` | `3` | Retry attempts |

**Per-step configuration:**

```yaml
agent:
  default: claude
  steps:
    planning:
      name: claude-opus
      env:
        MAX_TOKENS: "16384"
      args: ["--max-turns", "15"]
    implementing:
      name: claude
    reviewing:
      name: claude-opus
```

| Step | Description |
|------|-------------|
| `planning` | Agent for `mehr plan` |
| `implementing` | Agent for `mehr implement` |
| `reviewing` | Agent for `mehr review` |
| `checkpointing` | Agent for checkpoint summaries |

### providers

```yaml
providers:
  default: file  # Allow "mehr start task.md" without prefix
```

### workflow

```yaml
workflow:
  auto_init: true                  # Auto-initialize workspace
  session_retention_days: 30       # Keep sessions for N days
  delete_work_on_finish: false     # Delete work dirs after finish
  delete_work_on_abandon: true     # Delete work dirs on abandon
```

### storage

```yaml
storage:
  work_dir: .mehrhof/work  # Path relative to project root
```

### cache

```yaml
cache:
  enabled: true  # Enable/disable caching globally

github:
  cache:
    disabled: false  # Provider-specific override
```

### env

Environment variables filtered by agent name prefix:

```yaml
env:
  CLAUDE_ANTHROPIC_API_KEY: sk-ant-xxx  # → ANTHROPIC_API_KEY to claude
  CLAUDE_MAX_TOKENS: "4096"              # → MAX_TOKENS to claude
  OPENAI_API_KEY: sk-openai-xxx          # → API_KEY to openai agent
```

Variables are filtered by agent name prefix, stripped when passed.

## Environment File (.env)

Store secrets locally without committing to git.

**Location:** `.mehrhof/.env` (gitignored)

```bash
ANTHROPIC_API_KEY=sk-ant-...
GLM_API_KEY=your-glm-key

GITHUB_TOKEN=ghp_...
JIRA_TOKEN=your-jira-token
```

**How it works:**

1. Loaded at CLI startup before other initialization
2. Variables available to `${VAR}` syntax in config.yaml
3. System environment variables take priority

**Usage with agent aliases:**

```yaml
# .mehrhof/config.yaml
agents:
  glm:
    extends: claude
    env:
      ANTHROPIC_API_KEY: "${GLM_API_KEY}"  # Reads from .env
```

**Security:**
- Created with `0600` permissions
- Automatically added to `.gitignore` by `mehr init`
- Never commit to version control

## User Settings

Personal preferences stored automatically.

**Location:** `~/.mehrhof/settings.json`

```json
{
  "preferred_agent": "claude",
  "target_branch": "main",
  "last_provider": "file",
  "recent_tasks": ["abc12345", "def67890"]
}
```

Updated automatically as you use Mehrhof.

## CLI Flags

Global flags available on all commands:

| Flag | Description |
|------|-------------|
| `-v, --verbose` | Enable verbose output |
| `--no-color` | Disable colored output |

The `NO_COLOR` environment variable is also respected.

## Standard Environment Variables

| Variable | Description |
|----------|-------------|
| `NO_COLOR` | Disable colored output |
| `ANTHROPIC_API_KEY` | Claude API key (used by Claude CLI) |
| `GITHUB_TOKEN` | GitHub API token |
| `MEHR_GITHUB_TOKEN` | GitHub token (takes priority) |

## Quick Reference

| What | How |
|------|-----|
| Verbose output | `mehr --verbose <command>` |
| Disable color | `mehr --no-color <command>` or `NO_COLOR=1` |
| Set default agent | `.mehrhof/config.yaml` → `agent.default` |
| Custom branch pattern | `.mehrhof/config.yaml` → `git.branch_pattern` |
| Set default provider | `.mehrhof/config.yaml` → `providers.default` |

## Best Practices

### What to Commit

```
.mehrhof/config.yaml    # Workspace config (no secrets!)
```

### What to Gitignore

```
.mehrhof/work/          # Task data
.mehrhof/.env           # Secrets
.mehrhof/.active_task   # Current task state
```

### Validate Configuration

```bash
mehr config validate
```

## See Also

- [CLI Reference](../cli/index.md) - All commands
- [Agents](../agents/index.md) - Agent configuration and aliases
- [Troubleshooting](../troubleshooting/index.md) - Common issues
