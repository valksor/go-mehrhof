# kvelmo hooks

List configured workflow hooks.

## Usage

```bash
kvelmo hooks [flags]
```

## Options

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON |

## What It Shows

Pre-transition hooks that run before workflow state changes. These are configured in project or global settings.

## Examples

```bash
# List all hooks
kvelmo hooks

# JSON output
kvelmo hooks --json
```

## Related

- [config](/cli/config.md) — Configuration
- [Workflow](/concepts/workflow.md) — Workflow concepts
