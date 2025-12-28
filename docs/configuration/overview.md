# Configuration Overview

Mehrhof uses a layered configuration system with sensible defaults. You can customize behavior through environment variables, configuration files, or command-line flags.

## Important: How Claude Integration Works

Mehrhof calls Claude CLI as a subprocess. This means:

- **Claude's settings** (API keys, model, etc.) are handled by Claude CLI itself
- **Mehrhof's settings** (`MEHR_*` variables) control Mehrhof's behavior
- Environment variables set in Mehrhof config are passed to Claude when it runs

## Configuration Priority

Configuration is loaded from multiple sources. Later sources override earlier ones:

```
1. Built-in defaults     (lowest priority)
2. User config          ~/.mehrhof/.env
3. Project config       .env
4. Local overrides      .env.local
5. Environment vars     MEHR_* variables  (highest priority)
```

## Quick Reference

| What             | How                             |
| ---------------- | ------------------------------- |
| Increase timeout | `export MEHR_AGENT_TIMEOUT=600` |
| JSON output      | `export MEHR_UI_FORMAT=json`    |
| Disable color    | `export MEHR_UI_COLOR=false`    |

## Configuration Sections

### Agent

Controls AI agent behavior:

| Setting                 | Default | Description        |
| ----------------------- | ------- | ------------------ |
| `MEHR_AGENT_DEFAULT`    | claude  | Default agent      |
| `MEHR_AGENT_TIMEOUT`    | 300     | Timeout in seconds |
| `MEHR_AGENT_MAXRETRIES` | 3       | Retry attempts     |

### Storage

Controls data persistence:

| Setting                             | Default  | Description           |
| ----------------------------------- | -------- | --------------------- |
| `MEHR_STORAGE_ROOT`                 | .mehrhof | Storage directory     |
| `MEHR_STORAGE_MAXBLUEPRINTS`        | 100      | Max stored blueprints |
| `MEHR_STORAGE_SESSIONRETENTIONDAYS` | 30       | Session log retention |

### Git

Controls version control integration:

| Setting                  | Default        | Description           |
| ------------------------ | -------------- | --------------------- |
| `MEHR_GIT_AUTOCOMMIT`    | true           | Auto-commit changes   |
| `MEHR_GIT_COMMITPREFIX`  | [task]         | Commit message prefix |
| `MEHR_GIT_BRANCHPATTERN` | task/{task_id} | Branch name pattern   |

### UI

Controls output formatting:

| Setting            | Default | Description               |
| ------------------ | ------- | ------------------------- |
| `MEHR_UI_COLOR`    | true    | Colored output            |
| `MEHR_UI_FORMAT`   | text    | Output format (text/json) |
| `MEHR_UI_VERBOSE`  | false   | Verbose output            |
| `MEHR_UI_PROGRESS` | spinner | Progress style            |

## Default Configuration

Without any customization, Mehrhof uses:

```yaml
agent:
  default: claude
  timeout: 300
  maxretries: 3
  claude:
    model: claude-sonnet-4-20250514
    maxtokens: 8192
    temperature: 0.7

storage:
  root: .mehrhof
  maxblueprints: 100
  sessionretentiondays: 30

git:
  autocommit: true
  commitprefix: "[task]"
  branchpattern: "task/{task_id}"
  signcommits: false

ui:
  color: true
  format: text
  verbose: false
  progress: spinner
```

## Viewing Current Configuration

Configuration is applied at runtime. To verify settings:

```bash
# Check specific variable
echo $MEHR_AGENT_DEFAULT

# Run with verbose flag
mehr status --verbose
```

## Common Configurations

### Development Environment

For local development with extended timeouts:

```bash
# .env.local
MEHR_AGENT_TIMEOUT=600
MEHR_UI_VERBOSE=true
```

### CI/CD Environment

For automated pipelines:

```bash
export MEHR_UI_COLOR=false
export MEHR_UI_FORMAT=json
export MEHR_UI_PROGRESS=none
```

### Team Settings

Share project-level settings in `.env`:

```bash
# .env (committed to repo)
MEHR_GIT_COMMITPREFIX=[feature]
MEHR_GIT_BRANCHPATTERN=feature/{task_id}
```

## Next Steps

- [Environment Variables](configuration/environment.md) - Complete variable reference
- [Configuration Files](configuration/files.md) - File locations and formats
