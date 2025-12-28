# Configuration Overview

Mehrhof uses a simple configuration system with sensible defaults. Configure behavior through workspace configuration files or command-line flags.

## Important: How Claude Integration Works

Mehrhof calls Claude CLI as a subprocess. This means:

- **Claude's settings** (API keys, model, etc.) are handled by Claude CLI itself
- **Mehrhof's settings** (workspace config, CLI flags) control Mehrhof's behavior
- Environment variables in workspace config are passed to Claude when it runs

## Configuration Methods

| Method               | Use Case               | Example                    |
| -------------------- | ---------------------- | -------------------------- |
| **CLI flags**        | Per-command overrides  | `mehr --verbose plan`      |
| **Workspace config** | Project-level settings | `.mehrhof/config.yaml`     |
| **User settings**    | Personal preferences   | `~/.mehrhof/settings.json` |

## Quick Reference

| What                  | How                                           |
| --------------------- | --------------------------------------------- |
| Verbose output        | `mehr --verbose <command>`                    |
| Disable color         | `mehr --no-color <command>` or `NO_COLOR=1`   |
| Set default agent     | `.mehrhof/config.yaml` → `agent.default`      |
| Custom branch pattern | `.mehrhof/config.yaml` → `git.branch_pattern` |

## Workspace Configuration

Project-level settings in `.mehrhof/config.yaml`:

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

# Environment variables passed to agents
env:
  ANTHROPIC_API_KEY: your-api-key-here
```

### Configuration Sections

#### git

Controls version control integration:

| Setting          | Default                | Description                    |
| ---------------- | ---------------------- | ------------------------------ |
| `auto_commit`    | `true`                 | Auto-commit after operations   |
| `commit_prefix`  | `[{key}]`              | Commit message prefix template |
| `branch_pattern` | `{type}/{key}--{slug}` | Branch name pattern            |
| `sign_commits`   | `false`                | GPG-sign commits               |

#### agent

Controls AI agent behavior:

| Setting       | Default  | Description        |
| ------------- | -------- | ------------------ |
| `default`     | `claude` | Default agent      |
| `timeout`     | `300`    | Timeout in seconds |
| `max_retries` | `3`      | Retry attempts     |

#### providers

Controls task source behavior:

| Setting   | Default | Description                          |
| --------- | ------- | ------------------------------------ |
| `default` | (none)  | Default provider for bare references |

#### workflow

Controls workflow behavior:

| Setting                  | Default | Description               |
| ------------------------ | ------- | ------------------------- |
| `auto_init`              | `true`  | Auto-initialize workspace |
| `session_retention_days` | `30`    | Keep sessions for N days  |

## CLI Flags

Global flags available on all commands:

| Flag            | Description                           |
| --------------- | ------------------------------------- |
| `-v, --verbose` | Enable verbose output (debug logging) |
| `--no-color`    | Disable colored output                |

The `NO_COLOR` environment variable is also respected for disabling colors.

## User Settings

Personal preferences stored in `~/.mehrhof/settings.json`:

```json
{
  "preferred_agent": "claude",
  "target_branch": "main",
  "last_provider": "file",
  "recent_tasks": ["abc123", "def456"]
}
```

These are updated automatically as you use Mehrhof.

## Standard Environment Variables

Mehrhof respects these standard environment variables:

| Variable            | Description                                |
| ------------------- | ------------------------------------------ |
| `NO_COLOR`          | Disable colored output (any value)         |
| `ANTHROPIC_API_KEY` | Claude API key (used by Claude CLI)        |
| `GITHUB_TOKEN`      | GitHub API token                           |
| `MEHR_GITHUB_TOKEN` | GitHub token (alternative, takes priority) |

## Common Configurations

### CI/CD Environment

For automated pipelines, use CLI flags:

```bash
mehr --no-color --verbose plan
```

Or set standard env vars:

```bash
export NO_COLOR=1
mehr plan
```

### Team Settings

Share project-level settings in `.mehrhof/config.yaml`:

```yaml
git:
  branch_pattern: "feature/{key}--{slug}"
  commit_prefix: "[{key}]"

providers:
  default: file
```

## Next Steps

- [Configuration Files](configuration/files.md) - File locations and formats
