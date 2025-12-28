# Environment Variables

Complete reference for all `MEHR_*` environment variables.

## Important Note

Mehrhof calls Claude CLI for AI operations. It doesn't use API keys directly - your Claude CLI configuration handles authentication. The environment variables below configure Mehrhof's behavior, not AI authentication.

## Agent Configuration

### General Agent Settings

| Variable                | Type   | Default | Description                       |
| ----------------------- | ------ | ------- | --------------------------------- |
| `MEHR_AGENT_DEFAULT`    | string | claude  | Default agent                     |
| `MEHR_AGENT_TIMEOUT`    | int    | 300     | Agent timeout in seconds          |
| `MEHR_AGENT_MAXRETRIES` | int    | 3       | Maximum retry attempts on failure |

## Storage Configuration

| Variable                            | Type   | Default  | Description                     |
| ----------------------------------- | ------ | -------- | ------------------------------- |
| `MEHR_STORAGE_ROOT`                 | string | .mehrhof | Root directory for task storage |
| `MEHR_STORAGE_MAXBLUEPRINTS`        | int    | 100      | Maximum stored blueprints       |
| `MEHR_STORAGE_SESSIONRETENTIONDAYS` | int    | 30       | Days to keep session logs       |

## Git Configuration

| Variable                 | Type   | Default        | Description                  |
| ------------------------ | ------ | -------------- | ---------------------------- |
| `MEHR_GIT_AUTOCOMMIT`    | bool   | true           | Auto-commit after operations |
| `MEHR_GIT_COMMITPREFIX`  | string | [task]         | Prefix for commit messages   |
| `MEHR_GIT_BRANCHPATTERN` | string | task/{task_id} | Pattern for branch names     |
| `MEHR_GIT_SIGNCOMMITS`   | bool   | false          | GPG-sign commits             |

## UI Configuration

| Variable           | Type   | Default | Description                                  |
| ------------------ | ------ | ------- | -------------------------------------------- |
| `MEHR_UI_COLOR`    | bool   | true    | Enable colored output                        |
| `MEHR_UI_FORMAT`   | string | text    | Output format: `text` or `json`              |
| `MEHR_UI_VERBOSE`  | bool   | false   | Enable verbose output                        |
| `MEHR_UI_PROGRESS` | string | spinner | Progress style: `spinner`, `dots`, or `none` |

## Provider Configuration

| Variable                            | Type   | Default | Description                      |
| ----------------------------------- | ------ | ------- | -------------------------------- |
| `MEHR_PROVIDERS_FILE_BASEPATH`      | string | .       | Base path for file provider      |
| `MEHR_PROVIDERS_DIRECTORY_BASEPATH` | string | .       | Base path for directory provider |

## Variable Naming Convention

Environment variables follow the pattern:

```
MEHR_<SECTION>_<KEY>
```

Examples:

- `MEHR_AGENT_TIMEOUT` → `agent.timeout`
- `MEHR_UI_FORMAT` → `ui.format`
- `MEHR_GIT_COMMITPREFIX` → `git.commitprefix`

## Setting Variables

### Shell Export

```bash
export MEHR_AGENT_TIMEOUT=600
```

### Shell Profile

Add to `~/.bashrc` or `~/.zshrc`:

```bash
export MEHR_AGENT_TIMEOUT=600
export MEHR_UI_COLOR=true
```

### .env Files

Create `.env` in your project:

```bash
# .env
MEHR_AGENT_DEFAULT=claude
MEHR_UI_VERBOSE=true
```

### Per-Command

```bash
MEHR_UI_VERBOSE=true mehr plan
```

## Examples

### Extended Timeout

For complex tasks that need more time:

```bash
export MEHR_AGENT_TIMEOUT=900  # 15 minutes
```

### CI/CD Mode

For automated pipelines:

```bash
export MEHR_UI_COLOR=false
export MEHR_UI_FORMAT=json
export MEHR_UI_PROGRESS=none
```

### Custom Branch Pattern

Use feature branches instead of task branches:

```bash
export MEHR_GIT_BRANCHPATTERN=feature/{task_id}
```

### Signed Commits

For repositories requiring GPG signatures:

```bash
export MEHR_GIT_SIGNCOMMITS=true
```

## Validation

Invalid values produce errors:

```bash
export MEHR_AGENT_DEFAULT=invalid
mehr start task.md
# Error: invalid agent "invalid", must be "claude"
```

Valid values:

| Variable             | Valid Values              |
| -------------------- | ------------------------- |
| `MEHR_AGENT_DEFAULT` | `claude`                  |
| `MEHR_UI_FORMAT`     | `text`, `json`            |
| `MEHR_UI_PROGRESS`   | `spinner`, `dots`, `none` |

## Troubleshooting

### Variable Not Applied

Check if it's set:

```bash
echo $MEHR_AGENT_TIMEOUT
```

Check priority:

```bash
# .env.local overrides .env
cat .env.local
```

## See Also

- [Configuration Files](configuration/files.md) - File-based configuration
- [Configuration Overview](configuration/overview.md) - Priority and defaults
