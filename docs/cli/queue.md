# kvelmo queue

Task queue management.

## Usage

```bash
kvelmo queue <subcommand>
```

## Subcommands

| Subcommand                | Description                   |
| ------------------------- | ----------------------------- |
| `add <source>`            | Add a task to the queue       |
| `remove <id>`             | Remove a task from the queue  |
| `list`                    | List queued tasks             |
| `reorder <id> <position>` | Move a task to a new position |

## Options

| Flag      | Subcommand | Description                        |
| --------- | ---------- | ---------------------------------- |
| `--title` | `add`      | Optional title for the queued task |
| `--json`  | `list`     | Output as JSON                     |

## Examples

```bash
# Add a GitHub issue to the queue
kvelmo queue add github:owner/repo#123

# Add with a title
kvelmo queue add github:owner/repo#456 --title "Fix login bug"

# List queued tasks
kvelmo queue list

# Move task to position 1
kvelmo queue reorder task-abc 1

# Remove a task
kvelmo queue remove task-abc
```

## Related

- [batch](/cli/batch.md) — Batch operations
- [start](/cli/start.md) — Start a task
