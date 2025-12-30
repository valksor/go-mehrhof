# mehr undo

Revert to the previous checkpoint.

## Synopsis

```bash
mehr undo [-y|--yes]
```

## Description

The `undo` command reverts the task to its previous checkpoint. This undoes the last set of changes by resetting to a previous git state.

By default, `mehr undo` shows a confirmation prompt before proceeding. Use `--yes` to skip it.

## Flags

| Flag | Description |
|------|-------------|
| `-y, --yes` | Skip confirmation prompt |

## Examples

```bash
# Undo with confirmation
mehr undo

# Undo without confirmation
mehr undo --yes
mehr undo -y
```

Output:

```
Reverting to checkpoint...
Restored: checkpoint-2
Files changed:
  - src/api/handler.go (reverted)
  - src/api/auth.go (removed)
Undo complete. Use 'mehr redo' to restore.
```

## What Happens

1. Current state pushed to redo stack
2. Previous checkpoint restored
3. Working directory updated
4. State returns to idle

## When to Undo

- AI generated incorrect code
- Implementation doesn't meet requirements
- Want to try a different approach
- Made mistakes during experimentation

## Checking Availability

Use `mehr status` to see how many undos are available:

```bash
mehr status
```

```
Checkpoints:
  Undo: 3 available
  Redo: 1 available
```

## Limitations

### Cannot Undo When:

- No checkpoints exist (task is at initial state)
- Manual git commits were made (not checkpoints)
- Undo stack is empty

### Error Messages

```
Error: Cannot undo - no checkpoints available
The task is at its initial state.
```

## Workflow Examples

### Iterating on Implementation

```bash
mehr implement        # First attempt
# Review... not quite right
mehr undo
mehr note "Use a simpler approach"
mehr implement        # Second attempt
# Better!
mehr finish
```

### Exploring Options

```bash
mehr implement        # Approach A
mehr undo             # Back to try something else
mehr note "Try functional style"
mehr implement        # Approach B
# Decide on B
mehr finish
```

### Recovering from Mistakes

```bash
mehr implement
# Accidentally ran something destructive
mehr undo            # Safe again
```

## Advanced Recovery

If you need to recover beyond the checkpoint system:

```bash
# View git history
git reflog

# Find the desired state
# abc1234 HEAD@{5}: commit: [task] implement

# Restore specific files
git checkout abc1234 -- path/to/file.go

# Or reset (use carefully)
git reset --hard abc1234
```

## See Also

- [redo](redo.md) - Restore after undo
- [Checkpoints](../concepts/checkpoints.md) - How checkpoints work
- [implement](implement.md) - Generate code
- [plan](plan.md) - Create specifications
