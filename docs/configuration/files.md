# Configuration Files

Mehrhof uses several configuration files at different levels.

## File Locations

| File                       | Location            | Purpose                      |
| -------------------------- | ------------------- | ---------------------------- |
| `~/.mehrhof/.env`          | Home directory      | User-level defaults          |
| `.env`                     | Project root        | Project-level settings       |
| `.env.local`               | Project root        | Local overrides (gitignored) |
| `.mehrhof/config.yaml`     | Project `.mehrhof/` | Workspace configuration      |
| `~/.mehrhof/settings.json` | Home directory      | User preferences             |
| `~/.mehrhof/plugins/`      | Home directory      | Global plugins               |

## Priority Order

Files are loaded in this order (later overrides earlier):

```
1. ~/.mehrhof/.env          (lowest)
2. .env
3. .env.local               (highest file)
4. Environment variables    (highest overall)
```

## .env Files

### Format

Standard dotenv format:

```bash
# Comment
KEY=value
ANOTHER_KEY="value with spaces"
```

### Project .env

Shared project settings (commit to repo):

```bash
# .env
MEHR_GIT_COMMITPREFIX=[{key}]
MEHR_GIT_BRANCHPATTERN={type}/{key}--{slug}
MEHR_STORAGE_ROOT=.mehrhofs
```

### Local .env.local

Personal overrides (add to .gitignore):

```bash
# .env.local
MEHR_UI_VERBOSE=true
MEHR_AGENT_TIMEOUT=600
```

### User ~/.mehrhof/.env

User-level defaults across all projects:

```bash
# ~/.mehrhof/.env
MEHR_AGENT_DEFAULT=claude
MEHR_UI_COLOR=true
```

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
| `dialogue`      | Agent for `mehr talk`          |
| `checkpointing` | Agent for checkpoint summaries |

Each step can specify:

- `name` - Agent name or alias
- `env` - Environment variables for this step
- `args` - CLI arguments for this step

See [Agents - Per-Step Configuration](../concepts/agents.md#per-step-agent-configuration) for details.

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
- Updates `.gitignore`

### Create .env.local

```bash
cat > .env.local << 'EOF'
MEHR_UI_VERBOSE=true
MEHR_AGENT_TIMEOUT=600
EOF
```

### Create User Config

```bash
mkdir -p ~/.mehrhof
cat > ~/.mehrhof/.env << 'EOF'
MEHR_AGENT_DEFAULT=claude
MEHR_UI_COLOR=true
EOF
```

## Best Practices

### What to Commit

```
.env              # Shared project settings (no secrets!)
.mehrhof/config.yaml # Workspace configuration
```

### What to Gitignore

```
.env.local        # Personal overrides
.mehrhof/work/       # Task data
.mehrhof/.active_task
```

### Team Configuration

Share non-sensitive settings in `.env`:

```bash
# .env - safe to commit
MEHR_GIT_BRANCHPATTERN={type}/{key}--{slug}
MEHR_GIT_COMMITPREFIX=[{key}]
```

Keep personal overrides in `.env.local`:

```bash
# .env.local - not committed
MEHR_UI_VERBOSE=true
```

## Troubleshooting

### Config Not Loading

Check file exists and is readable:

```bash
ls -la .env .env.local
```

### Wrong Priority

Environment variables always win:

```bash
# This overrides any file
export MEHR_AGENT_TIMEOUT=600
```

### YAML Syntax Errors

Validate config.yaml:

```bash
cat .mehrhof/config.yaml | python -c "import yaml,sys; yaml.safe_load(sys.stdin)"
```

## See Also

- [Environment Variables](configuration/environment.md) - All variables
- [Configuration Overview](configuration/overview.md) - How config works
