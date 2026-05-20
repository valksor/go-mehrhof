# tag

Manage task tags.

## Usage

```bash
kvelmo tag <subcommand>
```

## Subcommands

| Subcommand           | Description                        |
| -------------------- | ---------------------------------- |
| `add <tag> [tag...]` | Add tags to the current task       |
| `remove <tag>`       | Remove a tag from the current task |
| `list`               | List tags on the current task      |

## Examples

```bash
# Add tags
kvelmo tag add backend urgent

# Remove a tag
kvelmo tag remove urgent

# List all tags
kvelmo tag list
```

## Use With Batch

Tags can be used as filters with `kvelmo batch`:

```bash
kvelmo batch submit --tag backend
```

## Related

- [batch](/cli/batch.md) — Batch operations with tag filters
- [queue](/cli/queue.md) — Task queue
