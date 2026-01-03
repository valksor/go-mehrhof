# Checkpoints

Mehrhof automatically creates git checkpoints during planning and implementation, enabling safe experimentation with undo/redo support.

## What are Checkpoints?

Checkpoints are git commits that capture the state of your code at specific points in the workflow. They allow you to:

- Undo unwanted changes
- Redo reverted changes
- Experiment safely
- Review changes incrementally

## Automatic Checkpointing

Checkpoints are created automatically after:

- **Planning** - After SPEC files are generated
- **Implementation** - After code changes are applied
- **Chat sessions** - If files are modified

## Checkpoint Structure

Checkpoints use a stack-based system:

```
Undo Stack          Redo Stack
-----------         -----------
checkpoint-3        (empty after undo)
checkpoint-2
checkpoint-1
```

After an undo:

```
Undo Stack          Redo Stack
-----------         -----------
checkpoint-2        checkpoint-3
checkpoint-1
```

## Using Undo

Revert to the previous checkpoint:

```bash
mehr undo
```

This:

1. Saves current state to redo stack
2. Restores the previous checkpoint
3. Updates working directory

### When to Undo

- AI generated incorrect code
- Implementation doesn't meet requirements
- Want to try a different approach

## Using Redo

Restore an undone checkpoint:

```bash
mehr redo
```

This:

1. Restores the next checkpoint from redo stack
2. Pushes current state to undo stack

### Redo Behavior

The redo stack is cleared when:

- New changes are made after undo
- A new planning or implementation phase runs

## Checking Checkpoint Status

View available checkpoints:

```bash
mehr status
```

Output:

```
Task: a1b2c3d4
State: idle
Specifications: 2
Checkpoints:
  Undo: 3 available
  Redo: 1 available
```

## Checkpoint Workflow Example

```bash
# Start and plan
mehr start task.md
mehr plan
# Checkpoint 1 created

# First implementation attempt
mehr implement
# Checkpoint 2 created

# Not happy with result
mehr undo
# Back to checkpoint 1

# Add clarification
mehr note "Use a simpler approach without generics"

# Try again
mehr implement
# Checkpoint 3 created (redo stack cleared)

# This looks good!
mehr finish
```

## Git Integration

Checkpoints are implemented as git commits:

```bash
git log --oneline
# a1b2c3d [task] implement: code changes
# b2c3d4e [task] plan: specifications
# c3d4e5f [task] start: initial
```

Commit messages include:

- Task prefix `[task]`
- Phase name
- Brief description

## Manual Checkpoints

While Mehrhof creates checkpoints automatically, you can also make manual commits:

```bash
git add .
git commit -m "manual: added tests"
```

These integrate with the checkpoint system.

## Limitations

### Undo Limitations

- Only reverts to Mehrhof-created checkpoints
- Manual commits may not be undone
- Cannot undo past the initial task start

### Redo Limitations

- Redo stack cleared on new changes
- Cannot redo after manual commits

## Best Practices

1. **Review before implementing** - Check SPEC files before `mehr implement`

2. **Add notes** - Add context with `mehr note` before re-implementing

3. **Undo early** - If something's wrong, undo quickly before making more changes

4. **Check status** - Use `mehr status` to see checkpoint availability

5. **Don't fear experimentation** - Checkpoints make it safe to try different approaches

## Troubleshooting

### "Cannot undo: no checkpoints"

No previous checkpoints exist. This happens when:

- Just started a task
- Already at the earliest checkpoint

### "Cannot redo: nothing to redo"

The redo stack is empty. This happens when:

- No undo was performed
- New changes cleared the redo stack

### Recovering from Mistakes

If you accidentally made changes after undo:

```bash
# Check git reflog for the lost commit
git reflog

# Restore if needed
git checkout <commit-hash> -- <file>
```
