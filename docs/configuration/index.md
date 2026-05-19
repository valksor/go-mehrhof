# Configuration

Configuration controls how kvelmo behaves across agents, providers, workflow policy, workers, storage, and interface defaults.

The core principle is simple: kvelmo runs locally, so configuration is also local and layered.

## Configuration Sources

kvelmo combines settings from multiple places (listed in order of increasing priority):

1. built-in defaults
2. global settings
3. project settings
4. environment files
5. process environment variables

Project-level configuration overrides global configuration.

## Main Files

| Scope               | Path                            |
| ------------------- | ------------------------------- |
| Global settings     | `~/.valksor/kvelmo/kvelmo.yaml` |
| Project settings    | `.valksor/kvelmo.yaml`          |
| Global environment  | `~/.valksor/kvelmo/.env`        |
| Project environment | `.valksor/.env`                 |

Use YAML for settings files. Environment files are useful for tokens and other sensitive values.

## What You Usually Configure

Common configuration areas include:

- default and allowed agents
- provider credentials and behavior
- worker pool behavior
- workflow defaults and skip phases
- policy and review controls
- browser, memory, and other subsystem settings

## Start With the CLI

The simplest way to inspect configuration is:

```bash
kvelmo config show
```

Set values with:

```bash
kvelmo config set <key> <value>
```

## Common Patterns

### Set a Global Default Agent

```bash
kvelmo config set agent.default claude
```

### Set a Project-Specific Override

```bash
kvelmo config set workflow.auto_advance true --scope project
```

### Add Provider Credentials

Use provider login commands or environment files instead of hardcoding secrets into YAML when possible.

## How to Think About Scope

Use **global** settings for things you want across all projects:

- preferred agent
- general workflow defaults
- default provider credentials

Use **project** settings for things that belong to one repository:

- project-specific workflow tweaks
- local provider overrides
- repository-specific behavior

## Security Guidance

- keep secrets in environment files or environment variables when possible
- do not commit `.env` files (add them to `.gitignore`)
- treat project settings as repository-local behavior, not as a secret store

## Related

- [Settings Reference](/configuration/settings.md)
- [Environment Variables](/configuration/environment.md)
- [Providers Overview](/providers/index.md)
