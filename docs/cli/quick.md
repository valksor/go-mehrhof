# kvelmo quick

Quick-fix workflow: load, implement, and submit in one step.

## Usage

```bash
kvelmo quick --from <source>
```

## Options

| Flag | Description |
|------|-------------|
| `--from` | Task source (required) |

## Examples

```bash
# Quick fix from GitHub issue
kvelmo quick --from github:owner/repo#123

# Quick fix from file
kvelmo quick --from file:task.md
```

## What It Does

Skips planning and review phases for trivial fixes where the full lifecycle is overkill. Loads the task, implements it, and submits a PR in one step.

## Related

- [start](/cli/start.md) — Full workflow start
- [implement](/cli/implement.md) — Implementation phase
- [submit](/cli/submit.md) — PR submission
