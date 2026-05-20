# checkpoints

List git checkpoints for the current task.

## Usage

```bash
kvelmo checkpoints
```

## Output

```
Checkpoints:
  1. abc1234 - plan (2 minutes ago)
  2. def5678 - implement (1 minute ago)

Redo stack:
  (empty)
```

## What It Shows

- Checkpoint SHA
- Phase that created it
- Timestamp

## Examples

```bash
# List checkpoints
kvelmo checkpoints
```

## Related

- [undo](/cli/undo.md) — Revert to checkpoint
- [redo](/cli/redo.md) — Restore checkpoint
- [Checkpoints](/concepts/checkpoints.md) — How they work
