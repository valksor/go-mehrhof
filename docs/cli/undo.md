# undo

Revert to the previous checkpoint.

## Usage

```bash
kvelmo undo
```

## Examples

```bash
# Undo last step
kvelmo undo

# Undo multiple times
kvelmo undo
kvelmo undo
```

## What Happens

1. Working directory reverts to previous checkpoint
2. Checkpoint is moved to redo stack
3. State is updated accordingly

## When to Use

- Implementation doesn't look right
- Want to try a different approach
- Made a mistake

## Safe to Use

Undo doesn't lose work. You can always redo:

```bash
kvelmo redo
```

## Multiple Undo

Each undo goes back one checkpoint:

```
implemented → planned → loaded
```

Also in Web UI: [Dashboard](/web-ui/dashboard.md).

## Related

- [redo](/cli/redo.md) — Restore after undo
- [Checkpoints](/concepts/checkpoints.md) — How checkpoints work
