# redo

Restore a checkpoint that was undone.

## Usage

```bash
kvelmo redo
```

## Examples

```bash
# Redo after undo
kvelmo redo
```

## What Happens

1. Checkpoint is restored from redo stack
2. Working directory is updated
3. State is updated accordingly

## When to Use

- Undid too far
- Want to compare implementations
- Restore work you undid

## Prerequisites

Must have previously used `kvelmo undo`.

Also in Web UI: [Dashboard](/web-ui/dashboard.md).

## Related

- [undo](/cli/undo.md) — Undo first
- [Checkpoints](/concepts/checkpoints.md) — How checkpoints work
