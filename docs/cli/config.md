# kvelmo config

Configuration management.

## Usage

```bash
kvelmo config <subcommand>
```

## Subcommands

| Command             | Description                                                |
| ------------------- | ---------------------------------------------------------- |
| `show`              | Show current configuration                                 |
| `init`              | Initialize default configuration                           |
| `get <key>`         | Get a specific configuration value                         |
| `set <key> <value>` | Set a configuration value                                  |
| `edit`              | Open configuration file in editor                          |
| `path`              | Show configuration file path                               |
| `validate`          | Validate configuration against schema                      |
| `check`             | Check for config drift between global and project settings |
| `diff`              | Show differences between global and project configuration  |

## Examples

```bash
# Show all settings
kvelmo config show

# Initialize defaults
kvelmo config init

# Get/set a value
kvelmo config get agent.default
kvelmo config set agent.default claude
kvelmo config set workers.max 8

# Validate and check drift
kvelmo config validate
kvelmo config check
kvelmo config check --json
```

## Configuration Files

| Scope   | Location                        |
| ------- | ------------------------------- |
| Global  | `~/.valksor/kvelmo/kvelmo.yaml` |
| Project | `.valksor/kvelmo.yaml`          |

## Common Settings

| Key             | Description                |
| --------------- | -------------------------- |
| `agent.default` | Default AI agent           |
| `workers.max`   | Maximum concurrent workers |
| `web.port`      | Web UI port                |

## Related

- [Configuration](/configuration/index.md) — Full configuration guide
- [Settings](/configuration/settings.md) — All settings
- [Environment](/configuration/environment.md) — Environment variables
