# Plugins

Mehrhof supports a plugin system for extending functionality without recompilation. Plugins communicate via JSON-RPC 2.0 over stdin/stdout, making them language-agnostic.

## Plugin Types

| Type         | Purpose                  | Example Use Cases                | Status     |
| ------------ | ------------------------ | -------------------------------- | ---------- |
| **Provider** | Custom task sources      | Jira, YouTrack, Linear, Notion   | Stable     |
| **Agent**    | Custom AI backends       | Local LLMs, Codex, custom models | Stable     |
| **Workflow** | State machine extensions | Approval steps, notifications    | Stable     |

## Quick Start

### 1. Check Available Plugins

```bash
mehr plugins list
```

### 2. Enable a Plugin

Add to `.mehrhof/config.yaml`:

```yaml
plugins:
  enabled:
    - jira
    - slack-notify
```

### 3. Use the Plugin

```bash
# Provider plugin
mehr start jira:PROJ-123

# Agent plugin
mehr plan --agent my-local-llm
```

## Plugin Locations

Plugins are discovered from two directories:

```
~/.mehrhof/plugins/     # Global (available to all projects)
.mehrhof/plugins/       # Project-local (overrides global)
```

Project-local plugins take precedence when both exist with the same name.

## Managing Plugins

```bash
# List all discovered plugins
mehr plugins list

# Validate a specific plugin
mehr plugins validate jira

# Show plugin details
mehr plugins info jira
```

## Configuration

### Enabling Plugins

```yaml
# .mehrhof/config.yaml
plugins:
  enabled:
    - jira
    - youtrack
```

### Plugin-Specific Config

```yaml
plugins:
  enabled:
    - jira
  config:
    jira:
      url: "https://company.atlassian.net"
      project: "PROJ"
```

### Environment Variables

Plugins may require environment variables. Check the plugin's documentation or manifest:

```bash
export JIRA_URL="https://company.atlassian.net"
export JIRA_TOKEN="your-api-token"
```

## Provider Plugins

Provider plugins add custom task sources accessed via URL schemes:

```bash
mehr start jira:PROJ-123      # Jira provider
mehr start yt:TASK-456        # YouTrack provider
mehr start linear:ABC-789     # Linear provider
```

### Capabilities

| Capability      | Description          |
| --------------- | -------------------- |
| `read`          | Fetch task details   |
| `snapshot`      | Capture task content |
| `list`          | List/search tasks    |
| `comment`       | Add comments         |
| `update_status` | Change task status   |
| `create_pr`     | Create pull requests |

## Agent Plugins

Agent plugins provide custom AI backends:

```bash
mehr plan --agent ollama       # Use local LLM
mehr implement --agent codex   # Use OpenAI Codex
```

### Agent Aliases

Create shortcuts for agent configurations in `.mehrhof/config.yaml`:

```yaml
agents:
  fast:
    extends: ollama
    description: "Fast local model"
    env:
      OLLAMA_MODEL: "codellama:7b"

  accurate:
    extends: claude
    description: "More accurate but slower"
    env:
      ANTHROPIC_MODEL: "claude-3-opus"
```

Use with:

```bash
mehr plan --agent fast
mehr implement --agent accurate
```

## Workflow Plugins

Workflow plugins extend the state machine with custom phases, guards, and effects:

```
idle → planning → [custom phase] → implementing → reviewing → [approval] → done
```

### Features

- **Dynamic Phase Insertion**: Add custom phases using `after` or `before` to specify insertion points
- **Plugin Guards**: Control transitions with custom guard conditions evaluated via JSON-RPC
- **Critical Effects**: Mark effects as `critical: true` to block workflow on failure

### Example Use Cases

- Manager approval before deployment
- Security scan after implementation
- Slack notifications on phase changes
- Jira status updates on workflow transitions

### Effect Criticality

Effects can be marked as critical in the plugin manifest:

```yaml
effects:
  - name: "notifySlack"
    description: "Send notification"
    critical: false  # Log errors but continue

  - name: "updateJira"
    description: "Update ticket status"
    critical: true   # Workflow fails if this fails
```

## Troubleshooting

### Plugin Not Found

```bash
# Check if plugin is discovered
mehr plugins list

# Verify plugin directory exists
ls -la ~/.mehrhof/plugins/
ls -la .mehrhof/plugins/
```

### Plugin Not Loading

```bash
# Validate plugin configuration
mehr plugins validate my-plugin

# Run with verbose output
mehr --verbose start my:task
```

### Missing Environment Variables

Check the plugin manifest for required variables:

```bash
cat ~/.mehrhof/plugins/jira/plugin.yaml
```

## Developing Plugins

For plugin development documentation, see:

```
.mehrhof/plugins/docs/
├── README.md              # Getting started
├── provider-plugins.md    # Provider development
├── agent-plugins.md       # Agent development
├── workflow-plugins.md    # Workflow development
└── protocol.md            # JSON-RPC protocol reference
```

Or browse the example link provider:

```
.mehrhof/plugins/link/
├── plugin.yaml            # Manifest
├── link-provider.py       # Implementation
└── README.md              # Usage documentation
```

## See Also

- [mehr plugins](../cli/plugins.md) - CLI reference
- [AI Agents](agents.md) - Built-in agent system
- [Configuration](../configuration/files.md) - Config file reference
