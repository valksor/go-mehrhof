# mehr redo

Restore the next checkpoint (after undo).

## Synopsis

```bash
mehr redo [-y|--yes]
```

## Description

The `redo` command restores changes that were previously undone. Only available after an undo.

By default, `mehr redo` shows a confirmation prompt before proceeding. Use `--yes` to skip it.

## Flags

| Flag | Description |
|------|-------------|
| `-y, --yes` | Skip confirmation prompt |

## Examples

```bash
mehr redo

mehr redo --yes
mehr redo -y
```

Output:

```
Restoring checkpoint...
Restored: checkpoint-3
Files changed:
  - src/api/handler.go (restored)
  - src/api/auth.go (created)
Redo complete.
```

## What Happens

1. Next checkpoint restored from redo stack
2. Current state pushed to undo stack
3. Working directory updated

## Redo Stack Behavior

The redo stack is **cleared** when:

- New changes are made after undo
- A new planning phase runs
- A new implementation phase runs

## Checkpoint Stacks

Mehrhof maintains two stacks:

```
Before:
Undo: [c1, c2, c3]    Redo: []

After 'mehr undo':
Undo: [c1, c2]        Redo: [c3]

After 'mehr redo':
Undo: [c1, c2, c3]    Redo: []

After 'mehr undo' then new implementation:
Undo: [c1, c2, c4]    Redo: []  (c3 lost!)
```

## Checking Availability

Use `mehr status` to see how many redos are available:

```bash
mehr status
```

```
Checkpoints:
  Undo: 3 available
  Redo: 1 available
```

## Limitations

### Cannot Redo When:

- Redo stack is empty (no undo was performed)
- New changes were made after undo (stack cleared)
- New planning or implementation phase ran

### Error Messages

```
Error: Cannot redo - nothing to redo
Either no undo was performed, or new changes cleared the redo stack.
```

## Workflow Examples

### Comparing Approaches

```bash
mehr implement
mehr undo
mehr note "Try functional style"
mehr implement
mehr undo
mehr redo
mehr finish
```

### Accidental Undo

```bash
mehr undo
mehr redo
```

## Redo Stack Clearing

The redo stack is cleared when you make new changes:

```bash
mehr implement
mehr undo
mehr implement
mehr redo
```

## See Also

- [undo](undo.md) - Revert to previous checkpoint
- [Checkpoints](../concepts/checkpoints.md) - How checkpoints work
- [implement](implement.md) - Generate code
