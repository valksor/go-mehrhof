# fork

Manage conversation forks for parallel approaches. Create multiple forks from the current checkpoint to try different implementation strategies, compare them side-by-side, then select the winner.

## Usage

```bash
kvelmo fork <subcommand>
```

## Subcommands

| Subcommand         | Description                                   |
| ------------------ | --------------------------------------------- |
| `create <label>`   | Create a new fork from the current checkpoint |
| `list`             | List active forks                             |
| `compare`          | Compare all active forks                      |
| `select <fork-id>` | Select the winning fork and merge it back     |

## Examples

```bash
# Create two forks to try different approaches
kvelmo fork create "approach-a-iterative"
kvelmo fork create "approach-b-recursive"

# List active forks
kvelmo fork list

# Compare forks side-by-side (files changed, lines added/removed)
kvelmo fork compare

# Select the winning fork
kvelmo fork select abc123
```

## Flags

| Flag     | Subcommand        | Description              |
| -------- | ----------------- | ------------------------ |
| `--json` | `list`, `compare` | Output raw JSON response |

## How It Works

Each fork creates a separate git worktree branching from the current checkpoint. Agents work independently in each fork. When you select a winner, the fork's changes are merged back into the main worktree.

## Web UI

Forks can also be managed from the web UI via the Fork tab, which provides a visual comparison of fork diffs and statistics.

## Related

- [checkpoints](/cli/checkpoints.md) — Navigate checkpoints
- [undo](/cli/undo.md) — Revert to previous checkpoint
