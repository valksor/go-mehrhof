# Configuration

kvelmo uses a layered configuration system with global, project, and environment settings.

## Configuration Files

| File    | Scope          | Location                        |
|---------|----------------|---------------------------------|
| Global  | All projects   | `~/.valksor/kvelmo/kvelmo.yaml` |
| Project | Single project | `.valksor/kvelmo.yaml`          |

## Priority Order

Settings are applied in this order (highest priority first):

1. Command-line flags
2. Environment variables
3. Project config
4. Global config
5. Defaults

## Quick Start

```bash
# Initialize default config
kvelmo config init

# Show current config
kvelmo config show

# Set a value
kvelmo config set default_agent claude
```

## Common Settings

| Setting         | Description                | Default     |
|-----------------|----------------------------|-------------|
| `default_agent` | AI agent to use            | Auto-detect |
| `max_workers`   | Maximum concurrent workers | 4           |
| `web_port`      | Web UI port                | 6337        |

## Configuration Topics

- [Settings Reference](/configuration/settings.md) — All settings
- [Environment Variables](/configuration/environment.md) — Environment overrides

## TUI Configuration

Settings for the terminal UI (`kvelmo tui`):

```yaml
tui:
  layout: stacked  # stacked (default) or dashboard
```

| Setting      | Description                             | Default   |
|--------------|-----------------------------------------|-----------|
| `tui.layout` | Default layout for the terminal UI      | `stacked` |

The `stacked` layout shows status, output, and chat vertically. The `dashboard` layout adds a workers pane alongside the output. See [kvelmo tui](/cli/tui.md) for details.

## Example Configuration

```json
{
  "default_agent": "claude",
  "max_workers": 8,
  "web_port": 6337,
  "git": {
    "auto_commit": true,
    "branch_pattern": "feature/{slug}"
  },
  "tui": {
    "layout": "stacked"
  },
  "providers": {
    "github": {
      "token": "ghp_xxxx"
    }
  }
}
```

## CLI Commands

```bash
# Initialize with defaults
kvelmo config init

# Show all settings
kvelmo config show

# Set a value
kvelmo config set <key> <value>
```

See [CLI: config](/cli/config.md) for full reference.
