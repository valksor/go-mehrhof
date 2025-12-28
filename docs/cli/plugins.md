# mehr plugins

Manage provider, agent, and workflow plugins.

## Synopsis

```bash
mehr plugins <subcommand> [flags]
```

## Description

The `plugins` command group manages the plugin system. Plugins extend mehr with custom integrations without requiring recompilation:

- **Providers**: Custom task sources (Jira, YouTrack, Linear) — *Stable*
- **Agents**: Custom AI backends (Codex, Junie, custom models) — *Stable*
- **Workflows**: Custom phases, guards, and effects — *Experimental*

> **Note:** Workflow plugins have scaffolding in place but are not yet fully integrated into the state machine. Use provider and agent plugins for production workloads.

## Subcommands

| Command    | Description                             |
| ---------- | --------------------------------------- |
| `list`     | List discovered plugins                 |
| `install`  | Install a plugin from git URL or path   |
| `remove`   | Remove an installed plugin              |
| `validate` | Validate plugin manifest and connection |
| `info`     | Show detailed plugin information        |

---

## mehr plugins list

List all discovered plugins from global and project directories.

```bash
mehr plugins list
```

**Output columns:**

| Column      | Description                            |
| ----------- | -------------------------------------- |
| NAME        | Plugin identifier                      |
| TYPE        | `provider`, `agent`, or `workflow`     |
| SCOPE       | `global` or `project`                  |
| ENABLED     | Whether enabled in config (`yes`/`no`) |
| DESCRIPTION | Human-readable description (truncated) |

**Example:**

```
NAME       TYPE      SCOPE    ENABLED  DESCRIPTION
jira       provider  global   yes      Jira integration provider
youtrack   provider  project  no       YouTrack integration
codex      agent     global   yes      OpenAI Codex agent
```

---

## mehr plugins install

Install a plugin from a git repository or local path.

```bash
mehr plugins install <source> [--global]
```

**Arguments:**

| Argument | Description                      |
| -------- | -------------------------------- |
| `source` | Git URL or local filesystem path |

**Flags:**

| Flag       | Description                                                     |
| ---------- | --------------------------------------------------------------- |
| `--global` | Install to `~/.mehrhof/plugins/` (default: `.mehrhof/plugins/`) |

**Examples:**

```bash
# Install from git
mehr plugins install https://github.com/user/mehrhof-jira

# Install from local path
mehr plugins install ./my-plugin

# Install globally
mehr plugins install https://github.com/user/mehrhof-jira --global
```

**Notes:**

- Git URLs are cloned with `--depth 1`
- The `mehrhof-` prefix is automatically stripped from plugin names
- After installation, enable the plugin in `config.yaml`

---

## mehr plugins remove

Remove an installed plugin by name.

```bash
mehr plugins remove <name> [--global]
```

**Arguments:**

| Argument | Description       |
| -------- | ----------------- |
| `name`   | Plugin identifier |

**Flags:**

| Flag       | Description                                     |
| ---------- | ----------------------------------------------- |
| `--global` | Remove from global directory (default: project) |

**Example:**

```bash
mehr plugins remove jira
mehr plugins remove jira --global
```

**Important:** Also remove the plugin from `plugins.enabled` in your config.yaml.

---

## mehr plugins validate

Validate a plugin's manifest and test that it can be loaded and initialized.

```bash
mehr plugins validate [name]
```

**Arguments:**

| Argument | Description                   |
| -------- | ----------------------------- |
| `name`   | Plugin to validate (optional) |

If no name is provided, all discovered plugins are validated.

**Validation checks:**

1. Manifest structure (`plugin.yaml`)
2. Executable exists
3. Process starts successfully
4. Initialization completes

**Example:**

```bash
# Validate specific plugin
mehr plugins validate jira

# Validate all plugins
mehr plugins validate
```

**Output:**

```
Validating 'jira'...
  OK
Validating 'youtrack'...
  ERROR: Executable not found: ./youtrack-provider

Some plugins failed validation
```

---

## mehr plugins info

Show detailed information about a specific plugin.

```bash
mehr plugins info <name>
```

**Example:**

```bash
mehr plugins info jira
```

**Output:**

```
Name:        jira
Type:        provider
Version:     1
Protocol:    1
Description: Jira integration provider
Scope:       global
Directory:   /Users/me/.mehrhof/plugins/jira
Author:      ACME Corp
Homepage:    https://github.com/acme/mehrhof-jira

Provider Configuration:
  Schemes:      jira
  Priority:     50
  Capabilities: read, list, comment, update_status

Expected Environment Variables:
  JIRA_URL (required)
    Jira server URL
  JIRA_TOKEN (required)
    API token for authentication
```

---

## Plugin Discovery

Plugins are discovered from two locations:

| Scope   | Path                  | Priority |
| ------- | --------------------- | -------- |
| Project | `.mehrhof/plugins/`   | Higher   |
| Global  | `~/.mehrhof/plugins/` | Lower    |

Project plugins with the same name override global plugins.

---

## Enabling Plugins

Discovered plugins are not loaded by default. Enable them explicitly in `.mehrhof/config.yaml`:

```yaml
plugins:
  enabled:
    - jira
    - youtrack
    - codex
  config:
    jira:
      url: "https://company.atlassian.net"
      project: "PROJ"
    youtrack:
      url: "https://youtrack.company.com"
```

---

## See Also

- [Plugins Concept](../concepts/plugins.md) - Understanding the plugin system
- [Configuration Files](../configuration/files.md) - Config file reference
