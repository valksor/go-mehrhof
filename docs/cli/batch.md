# kvelmo batch

Run an action across all active projects.

## Usage

```bash
kvelmo batch <action> [flags]
```

## Actions

| Action | Description |
|--------|-------------|
| `submit` | Submit all matching tasks (creates PRs) |
| `abort` | Abort all matching tasks |
| `reset` | Reset all matching tasks |
| `stop` | Stop all matching tasks |

## Options

| Flag | Description |
|------|-------------|
| `--state` | Filter by task state (e.g., `reviewing`, `failed`) |
| `--tag` | Filter by task tag |
| `--match` | Filter by project path substring |

## Examples

```bash
# Submit all reviewed tasks
kvelmo batch submit --state reviewing

# Stop all active tasks
kvelmo batch stop

# Abort all failed tasks
kvelmo batch abort --state failed

# Stop tasks with a specific tag
kvelmo batch stop --tag backend

# Submit tasks matching a path
kvelmo batch submit --match myproject
```

## Related

- [queue](/cli/queue.md) — Task queue management
- [tag](/cli/tag.md) — Task tagging
