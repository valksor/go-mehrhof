# Configuration Files

Mehrhof uses configuration files at different levels.

## File Locations

| File                       | Location            | Purpose                 |
| -------------------------- | ------------------- | ----------------------- |
| `.mehrhof/config.yaml`     | Project `.mehrhof/` | Workspace configuration |
| `.mehrhof/.env`            | Project `.mehrhof/` | Secrets (gitignored)    |
| `~/.mehrhof/settings.json` | Home directory      | User preferences        |
| `~/.mehrhof/plugins/`      | Home directory      | Global plugins          |

## Workspace config.yaml

Project-specific workspace configuration in YAML:

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
  default: file # Allow bare references like "task.md"

workflow:
  auto_init: true
  session_retention_days: 30

env:
  CLAUDE_ANTHROPIC_API_KEY: your-key # passed to claude as ANTHROPIC_API_KEY
```

### Sections

#### git

```yaml
git:
  auto_commit: true # Auto-commit after operations
  commit_prefix: "[{key}]" # Commit message prefix template
  branch_pattern: "{type}/{key}--{slug}" # Branch naming template
  sign_commits: false # GPG signing
```

**Template Variables:**
| Variable | Description | Example |
|----------|-------------|---------|
| `{key}` | External key (from filename/frontmatter/CLI) | `FEATURE-123` |
| `{task_id}` | Internal task ID | `a1b2c3d4` |
| `{type}` | Task type from filename prefix | `feature`, `fix` |
| `{slug}` | URL-safe slugified title | `add-user-auth` |

#### agent

```yaml
agent:
  default: claude # Default agent
  timeout: 300 # Timeout seconds
  max_retries: 3 # Retry count
  steps: # Per-step agent overrides (optional)
    planning:
      name: claude-opus # Use Opus for planning
      env:
        MAX_TOKENS: "16384"
      args: ["--max-turns", "15"] # CLI args for this step
    implementing:
      name: claude-sonnet # Use Sonnet for implementation
    reviewing:
      name: claude-opus
    dialogue:
      name: claude-sonnet
```

**Per-Step Configuration:**

| Step            | Description                    |
| --------------- | ------------------------------ |
| `planning`      | Agent for `mehr plan`          |
| `implementing`  | Agent for `mehr implement`     |
| `reviewing`     | Agent for `mehr review`        |
| `dialogue`      | Agent for `mehr chat`          |
| `checkpointing` | Agent for checkpoint summaries |

Each step can specify:

- `name` - Agent name or alias
- `env` - Environment variables for this step
- `args` - CLI arguments for this step

See [Agents - Per-Step Configuration](../concepts/agents.md#per-step-agent-configuration) for details.

#### providers

```yaml
providers:
  default: file # Default provider for bare references
```

With this set, you can use `mehr start task.md` instead of `mehr start file:task.md`.

#### workflow

```yaml
workflow:
  auto_init: true # Auto-initialize workspace
  session_retention_days: 30 # Keep sessions for N days
```

#### env

Environment variables passed to agents, filtered by agent name prefix:

```yaml
env:
  CLAUDE_ANTHROPIC_API_KEY: sk-ant-xxx # passed to claude as ANTHROPIC_API_KEY
  CLAUDE_MAX_TOKENS: "4096" # passed to claude as MAX_TOKENS
  OPENAI_API_KEY: sk-openai-xxx # passed to openai agent as API_KEY
```

**How it works:**

- Variables are filtered by agent name prefix (e.g., `CLAUDE_` for claude agent)
- The prefix is stripped when passed to the agent
- Unrelated prefixes are ignored (claude won't see `OPENAI_*` vars)

This allows configuring multiple agents without conflicts.

## Environment File (.env)

Store secrets locally without committing them to git.

**Location:** `.mehrhof/.env` (gitignored)

```bash
# API keys for agents
ANTHROPIC_API_KEY=sk-ant-...
GLM_API_KEY=your-glm-key

# GitHub token
GITHUB_TOKEN=ghp_...

# Wrike token
WRIKE_TOKEN=your-wrike-token

# Custom keys for aliases
MY_CUSTOM_API_KEY=secret-value
```

### How it works

1. `.env` is loaded at CLI startup, before any other initialization
2. Variables become available to `${VAR}` syntax in `config.yaml`
3. **System environment variables take priority** over `.env` values

### Usage with Agent Aliases

Reference `.env` variables in your agent aliases:

```yaml
# .mehrhof/config.yaml
agents:
  glm:
    extends: claude
    env:
      ANTHROPIC_API_KEY: "${GLM_API_KEY}" # Reads from .env
```

```bash
# .mehrhof/.env
GLM_API_KEY=sk-ant-your-secret-key
```

### Priority Order

1. System environment variables (highest)
2. `.env` file values
3. Defaults

This allows CI/CD to inject secrets without modifying `.env`.

### Security

- File is created with `0600` permissions (owner read/write only)
- Automatically added to `.gitignore` by `mehr init`
- Never commit this file to version control

## User Settings

Persistent user preferences stored as JSON.

**Location:** `~/.mehrhof/settings.json`

```json
{
  "preferred_agent": "claude",
  "target_branch": "main",
  "last_provider": "file",
  "recent_tasks": ["abc12345", "def67890"]
}
```

### Fields

| Field             | Type   | Description          |
| ----------------- | ------ | -------------------- |
| `preferred_agent` | string | Preferred AI agent   |
| `target_branch`   | string | Default merge target |
| `last_provider`   | string | Last used provider   |
| `recent_tasks`    | array  | Last 10 task IDs     |

### Automatic Updates

Settings are updated automatically:

- Starting a task updates `recent_tasks`
- Using `--agent` updates `preferred_agent`
- Using `--target` updates `target_branch`

## Active Task File

Tracks the currently active task.

**Location:** `.mehrhof/.active_task`

```yaml
id: cb9a54db
ref: dir:.mehrhof/my-feature
work_dir: .mehrhof/work/cb9a54db
state: idle
branch: task/cb9a54db
use_git: true
worktree_path: null
started: 2025-01-15T10:30:00Z
```

This file is managed by Mehrhof. Don't edit manually.

## Creating Configuration

### Initialize Workspace

```bash
mehr init
```

Creates:

- `.mehrhof/` directory
- `.mehrhof/config.yaml` with defaults
- `.mehrhof/.env` template for secrets
- Updates `.gitignore`

## Best Practices

### What to Commit

```
.mehrhof/config.yaml    # Workspace configuration (no secrets!)
```

### What to Gitignore

```
.mehrhof/work/          # Task data
.mehrhof/.env           # Secrets
.active_task
```

### Team Configuration

Share non-sensitive settings in `.mehrhof/config.yaml`:

```yaml
git:
  branch_pattern: "{type}/{key}--{slug}"
  commit_prefix: "[{key}]"

providers:
  default: file
```

## Troubleshooting

### YAML Syntax Errors

Validate config.yaml:

```bash
cat .mehrhof/config.yaml | python -c "import yaml,sys; yaml.safe_load(sys.stdin)"
```

Or use the validate command:

```bash
mehr config validate
```

## See Also

- [Configuration Overview](configuration/overview.md) - How config works
