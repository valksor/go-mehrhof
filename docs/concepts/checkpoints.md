# Checkpoints

kvelmo uses git checkpoints to enable undo and redo functionality. Every phase creates a checkpoint, allowing you to navigate through your work history.

## How It Works

When you complete a phase (plan, implement, etc.), kvelmo:

1. Creates a git commit with the current state
2. Tags the commit as a checkpoint
3. Stores the checkpoint SHA in the task metadata

This means your work is always recoverable through git.

## Creating Checkpoints

Checkpoints are created automatically after:

- `kvelmo plan` completes
- `kvelmo implement` completes
- `kvelmo simplify` completes
- `kvelmo optimize` completes

You can also create manual checkpoints with git commits.

## Viewing Checkpoints

```bash
# List all checkpoints for current task
kvelmo checkpoints
```

Output shows:

- Checkpoint SHA
- Timestamp
- Phase that created it
- Commit message

## Undo

Revert to the previous checkpoint:

```bash
kvelmo undo
```

This:

1. Moves the redo stack forward (so you can redo later)
2. Resets the working directory to the previous checkpoint
3. Updates the task state accordingly

**Safe to use:** `undo` doesn't lose work. You can always `redo` to restore.

## Redo

Restore a checkpoint you undid:

```bash
kvelmo redo
```

This:

1. Moves back through the redo stack
2. Restores the working directory to that checkpoint
3. Updates the task state accordingly

## Multiple Undo

You can undo multiple times:

```bash
kvelmo undo    # Go back one checkpoint
kvelmo undo    # Go back another
kvelmo undo    # And another
```

Each undo adds to the redo stack. Use `redo` to move forward again.

## Undo/Redo Stack

The checkpoint system maintains two stacks:

```
Checkpoints (undo stack):     Redo stack:
┌───────────────────────┐     ┌───────────────────────┐
│ implement checkpoint  │ ◄── │                       │
├───────────────────────┤     │                       │
│ plan checkpoint       │     │                       │
├───────────────────────┤     │                       │
│ start checkpoint      │     │                       │
└───────────────────────┘     └───────────────────────┘

After "kvelmo undo":
┌───────────────────────┐     ┌───────────────────────┐
│ plan checkpoint       │ ◄── │ implement checkpoint  │
├───────────────────────┤     │                       │
│ start checkpoint      │     │                       │
└───────────────────────┘     └───────────────────────┘

After "kvelmo redo":
┌───────────────────────┐     ┌───────────────────────┐
│ implement checkpoint  │ ◄── │                       │
├───────────────────────┤     │                       │
│ plan checkpoint       │     │                       │
├───────────────────────┤     │                       │
│ start checkpoint      │     │                       │
└───────────────────────┘     └───────────────────────┘
```

## Use Cases

### Try a Different Approach

```bash
kvelmo implement          # First attempt
# Not happy with the result
kvelmo undo               # Go back to planned state
kvelmo implement          # Try again with different context
```

### Compare Implementations

```bash
kvelmo implement          # Approach A
# Review the changes
kvelmo undo               # Go back
kvelmo implement          # Approach B
# Compare with Approach A
kvelmo undo               # Back to B
kvelmo redo               # See A again
```

### Recover from Mistakes

```bash
kvelmo implement          # Something went wrong
kvelmo undo               # No problem, revert
# Fix the specification or provide more context
kvelmo implement          # Try again
```

## Checkpoint Storage

Checkpoints are stored in the task's WorkUnit:

```json
{
  "checkpoints": ["sha1", "sha2", "sha3"],
  "redo_stack": ["sha4"]
}
```

The git commits themselves are in your repository's history.

## Limitations

- Checkpoints are per-task. Starting a new task creates a new checkpoint history.
- The redo stack is cleared when you create a new checkpoint (e.g., after a new implementation).
- Checkpoints don't include untracked files. Use `git stash` for temporary work.

## Related

- [Workflow](/concepts/workflow.md) — The phases that create checkpoints
- [State Machine](/concepts/state-machine.md) — States and transitions
