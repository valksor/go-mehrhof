# mehr undo / mehr redo

Revert or restore checkpoints for safe experimentation.

## mehr undo

Revert to the previous checkpoint.

### Synopsis

```bash
mehr undo
```

### Description

The `undo` command reverts the task to its previous checkpoint. This undoes the last set of changes by resetting to a previous git state.

### Examples

```bash
mehr undo
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

### What Happens

1. Current state pushed to redo stack
2. Previous checkpoint restored
3. Working directory updated
4. State returns to idle

### When to Undo

- AI generated incorrect code
- Implementation doesn't meet requirements
- Want to try a different approach
- Made mistakes during experimentation

---

## mehr redo

Restore the next checkpoint (after undo).

### Synopsis

```bash
mehr redo
```

### Description

The `redo` command restores changes that were previously undone. Only available after an undo.

### Examples

```bash
mehr redo
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

### What Happens

1. Next checkpoint restored from redo stack
2. Current state pushed to undo stack
3. Working directory updated

### Redo Stack Behavior

The redo stack is **cleared** when:

- New changes are made after undo
- A new planning phase runs
- A new implementation phase runs

---

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

Use `mehr status` to see checkpoint counts:

```bash
mehr status
```

```
Checkpoints:
  Undo: 3 available
  Redo: 1 available
```

## Workflow Examples

### Iterating on Implementation

```bash
mehr implement        # First attempt
# Review... not quite right
mehr undo
mehr talk "Use a simpler approach"
mehr implement        # Second attempt
# Better!
mehr finish
```

### Exploring Options

```bash
mehr implement        # Approach A
mehr undo
mehr talk "Try functional style"
mehr implement        # Approach B
# Compare approaches
mehr undo             # Back to A
mehr redo             # Forward to B
# Decide on B
mehr finish
```

### Recovering from Mistakes

```bash
mehr implement
# Accidentally ran something destructive
mehr undo            # Safe again
```

## Limitations

### Cannot Undo

- Past the initial task start
- Manual git commits (not checkpoints)
- If no checkpoints exist

### Cannot Redo

- If redo stack is empty
- After making new changes post-undo

## Error Messages

### "Cannot undo: no checkpoints"

```
Error: Cannot undo - no checkpoints available
The task is at its initial state.
```

### "Cannot redo: nothing to redo"

```
Error: Cannot redo - nothing to redo
Either no undo was performed, or new changes cleared the redo stack.
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

- [Checkpoints Concept](../concepts/checkpoints.md) - How checkpoints work
- [implement](cli/implement.md) - Generate code
- [plan](cli/plan.md) - Create specifications
