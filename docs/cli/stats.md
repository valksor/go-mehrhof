# stats

Show task analytics for the current project.

## Usage

```bash
kvelmo stats
```

## Description

Displays completion statistics, success rate, and recent task history from the archive.

## Options

| Flag     | Description                               |
| -------- | ----------------------------------------- |
| `--json` | Output as JSON                            |
| `--all`  | Show stats across all registered projects |

## Examples

```bash
# Show stats for current project
kvelmo stats

# Show stats across all projects
kvelmo stats --all

# JSON output
kvelmo stats --json
```

## Output

```
Tasks: 15 total, 12 finished, 2 submitted, 1 failed
Success rate: 93.3%
Avg duration: 4m32s

Recent tasks:
  Add user authentication       finished    2026-03-12 14:30  (5m12s)
  Fix login redirect            finished    2026-03-12 10:15  (2m45s)
  Update API documentation      submitted   2026-03-11 16:45  (8m20s)
```

## Related

- [status](/cli/status.md) — Show current task state
- [list](/cli/list.md) — List all tasks
