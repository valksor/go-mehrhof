# mehr config

Manage and validate Mehrhof configuration files.

## Commands

### mehr config init

Create a new `.mehrhof/config.yaml` file with sensible defaults.

```bash
mehr config init [flags]
```

**Flags:**

| Flag        | Short | Description                                                    |
| ----------- | ----- | -------------------------------------------------------------- |
| `--force`   | `-f`  | Overwrite existing config without prompting (requires confirmation) |
| `--project` |       | Project type for intelligent defaults: `go`, `node`, `python`, `php` |

**Behavior:**

- Detects project type automatically from common files (`go.mod`, `package.json`, `pyproject.toml`, `composer.json`)
- Creates `.mehrhof/config.yaml` with project-appropriate defaults
- Safe by default: warns if config exists and exits (unless `--force` is used)
- Generates helpful comments showing all available options

**Examples:**

```bash
mehr config init

mehr config init --project go

mehr config init --force
```

**Generated Config Structure:**

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

# User-defined agent aliases
agents:
  # Example: Custom agent with environment variables
  # glm:
  #   extends: claude
  #   description: "Claude with custom API"
  #   env:
  #     ANTHROPIC_API_KEY: "${CUSTOM_API_KEY}"

# Plugins
plugins:
  enabled: []
```

**What to do next:**

```bash
mehr config validate

vim .mehrhof/config.yaml

mehr start task.md
```

### mehr config validate

Validate workspace (`.mehrhof/config.yaml`) and app (`.env`) configuration files.

```bash
mehr config validate [flags]
```

**Flags:**

| Flag               | Description                                                |
| ------------------ | ---------------------------------------------------------- |
| `--workspace-only` | Only validate workspace config (`.mehrhof/config.yaml`)    |
| `--app-only`       | Only validate app config (`.env` files)                    |
| `--strict`         | Treat warnings as errors (exit code 1 if warnings present) |
| `--format`         | Output format: `text` (default), `json`                    |

**Exit Codes:**

| Code | Meaning                             |
| ---- | ----------------------------------- |
| 0    | Configuration is valid              |
| 1    | One or more validation errors found |

## Examples

### Validate All Configuration

```bash
mehr config validate
```

**Output (valid config):**

```
Validating configuration...

Configuration is VALID
```

**Output (with issues):**

```
Validating configuration...

.mehrhof/config.yaml:
  ERROR [AGENT_ALIAS_CIRCULAR] agents.glm-fast: Circular dependency detected: glm-fast -> glm -> glm-fast
    Suggestion: Remove circular reference in 'extends' field

  WARNING [ENV_VAR_UNSET] agents.glm.env.ANTHROPIC_API_KEY: Environment variable ${GLM_API_KEY} is not set
    Suggestion: Set GLM_API_KEY environment variable

Summary: 1 error(s), 1 warning(s)
Configuration is INVALID
```

### JSON Output for CI

```bash
mehr config validate --format json
```

```json
{
  "valid": false,
  "errors": 1,
  "warnings": 1,
  "findings": [
    {
      "severity": "error",
      "code": "AGENT_ALIAS_CIRCULAR",
      "message": "Circular dependency detected: glm-fast -> glm -> glm-fast",
      "path": "agents.glm-fast",
      "file": ".mehrhof/config.yaml",
      "suggestion": "Remove circular reference in 'extends' field"
    },
    {
      "severity": "warning",
      "code": "ENV_VAR_UNSET",
      "message": "Environment variable ${GLM_API_KEY} is not set",
      "path": "agents.glm.env.ANTHROPIC_API_KEY",
      "file": ".mehrhof/config.yaml",
      "suggestion": "Set GLM_API_KEY environment variable"
    }
  ]
}
```

### Strict Mode

Treat warnings as errors (useful for CI pipelines):

```bash
mehr config validate --strict
```

### Validate Only Workspace Config

```bash
mehr config validate --workspace-only
```

### Validate Only App Config

```bash
mehr config validate --app-only
```

## Validations Performed

### Workspace Config (`.mehrhof/config.yaml`)

| Check                     | Error Code               | Description                          |
| ------------------------- | ------------------------ | ------------------------------------ |
| YAML syntax               | `YAML_SYNTAX`            | Invalid YAML structure               |
| Agent alias circular deps | `AGENT_ALIAS_CIRCULAR`   | Alias chain forms a loop             |
| Undefined agent reference | `AGENT_ALIAS_UNDEFINED`  | `extends` references unknown agent   |
| Missing extends field     | `AGENT_ALIAS_NO_EXTENDS` | Alias missing required `extends`     |
| Invalid git pattern       | `GIT_PATTERN_INVALID`    | Unknown placeholder in pattern       |
| Empty git pattern         | `GIT_PATTERN_EMPTY`      | Branch pattern is empty              |
| Invalid enum value        | `INVALID_ENUM`           | Value not in allowed set             |
| Out of range value        | `INVALID_RANGE`          | Numeric value outside bounds         |
| Unset env variable        | `ENV_VAR_UNSET`          | `${VAR}` reference not set (warning) |
| Plugin config mismatch    | `PLUGIN_NOT_FOUND`       | Config for disabled plugin (warning) |

### App Config (`.env` files)

| Check             | Error Code       | Description                          |
| ----------------- | ---------------- | ------------------------------------ |
| Load error        | `ENV_LOAD_ERROR` | Failed to parse .env file            |
| Invalid agent     | `INVALID_ENUM`   | Unknown `agent.default` value        |
| Invalid UI format | `INVALID_ENUM`   | Must be `text` or `json`             |
| Invalid progress  | `INVALID_ENUM`   | Must be `spinner`, `dots`, or `none` |
| Out of range      | `INVALID_RANGE`  | Timeout, retries, etc. out of bounds |

### Cross-Config Validation

| Check             | Error Code        | Description                                 |
| ----------------- | ----------------- | ------------------------------------------- |
| Agent consistency | `AGENT_MISMATCH`  | Workspace default agent unknown             |
| Config conflict   | `CONFIG_CONFLICT` | Different values in app vs workspace (info) |

## Valid Git Pattern Placeholders

The following placeholders are valid in `git.branch_pattern` and `git.commit_prefix`:

| Placeholder | Description                       |
| ----------- | --------------------------------- |
| `{key}`     | Task key/ID                       |
| `{task_id}` | Task ID                           |
| `{type}`    | Task type (feature, bugfix, etc.) |
| `{slug}`    | URL-safe task title               |
| `{title}`   | Task title                        |

**Example patterns:**

```yaml
git:
  branch_pattern: "{type}/{key}--{slug}" # feature/ABC123--add-login
  commit_prefix: "[{key}]" # [ABC123] Implement feature
```

## CI/CD Integration

### GitHub Actions

```yaml
- name: Validate mehr config
  run: mehr config validate --strict --format json
```

### Pre-commit Hook

```bash
#!/bin/sh
mehr config validate --strict || exit 1
```

## Troubleshooting

### "circular alias dependency detected"

Your agent aliases form a loop:

```yaml
agents:
  a:
    extends: b # Problem: a -> b -> a
  b:
    extends: a
```

Fix by breaking the cycle.

### "extends unknown agent"

The `extends` field references an agent that doesn't exist:

```yaml
agents:
  custom:
    extends: nonexistent # "nonexistent" is not a known agent
```

Fix by using a valid agent name (`claude` or another defined alias).

### "Environment variable not set"

A `${VAR}` reference in your config points to an unset variable:

```yaml
agents:
  glm:
    env:
      ANTHROPIC_API_KEY: "${GLM_API_KEY}" # GLM_API_KEY not set
```

Fix by setting the environment variable or using a literal value.

## See Also

- [Configuration Guide](../configuration/index.md) - How configuration works and file reference
- [mehr agents](agents.md) - Agent commands
- [AI Agents](../agents/index.md) - Agent configuration and aliases
