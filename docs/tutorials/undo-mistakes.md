# Tutorial: Recovering from Mistakes

Learn how to use checkpoints to safely experiment and recover from errors.

## Understanding Checkpoints

Mehrhof creates checkpoints at key moments:

- After `mehr plan` (specs created)
- After `mehr implement` (code generated)
- After `mehr talk` (if files changed)

You can navigate between checkpoints with `mehr undo` and `mehr redo`.

## Scenario 1: Bad Implementation

### The Problem

You implemented a feature, but the code has issues:

```bash
mehr start task.md
mehr plan
mehr implement
```

After review, the generated code:

- Uses the wrong design pattern
- Has bugs in the logic
- Doesn't match your style

### The Solution

Simply undo and provide better guidance:

```bash
# Revert the implementation
mehr undo

# Add context about what went wrong
mehr talk "Use the repository pattern like in internal/users/. The current approach has tight coupling."

# Try again
mehr implement
```

### Check Your Options

See how many checkpoints you have:

```bash
mehr status
```

```
Checkpoints:
  Undo: 2 available
  Redo: 0 available
```

## Scenario 2: Comparing Approaches

### Exploring Options

Sometimes you want to compare different implementations:

```bash
mehr start task.md
mehr plan
mehr implement         # Approach A
```

Not sure if it's the best approach?

```bash
# Save this state and try another
mehr undo

mehr talk "Try a functional approach instead of OOP"
mehr implement         # Approach B
```

### Comparing

Now compare both:

```bash
# Currently at Approach B
git diff HEAD~1        # See current changes

# Go back to Approach A
mehr undo
mehr redo              # Back to A
git diff HEAD~1        # Compare

# Decide: keep B
mehr undo              # Back to before A
mehr redo              # Skip to B
```

### Warning

Making new changes after undo clears the redo stack:

```bash
mehr undo              # At checkpoint 2
mehr implement         # NEW checkpoint 3
mehr redo              # Error: nothing to redo (old checkpoint 3 is gone)
```

## Scenario 3: Multiple Undo Steps

### Deep Recovery

Sometimes you need to go back multiple steps:

```bash
mehr plan              # Checkpoint 1
mehr implement         # Checkpoint 2
mehr talk "add tests"
mehr implement         # Checkpoint 3
mehr talk "fix bug"
mehr implement         # Checkpoint 4 (current)
```

To get back to checkpoint 2:

```bash
mehr undo              # Checkpoint 3
mehr undo              # Checkpoint 2
```

Check status:

```bash
mehr status
```

```
Checkpoints:
  Undo: 1 available
  Redo: 2 available
```

## Scenario 4: Recovering Accidentally Undone Work

### The Problem

You undid something you actually wanted:

```bash
mehr implement         # Good code!
mehr undo              # Oops, didn't mean to
```

### The Solution

```bash
mehr redo              # Restored!
```

### If You Made Changes

If you accidentally made changes after undo:

```bash
mehr implement         # Good code
mehr undo              # Mistake
mehr talk "something"   # This clears redo!
mehr redo              # Error: nothing to redo
```

Use git reflog to recover:

```bash
# Find the lost commit
git reflog

# Output:
# abc1234 HEAD@{0}: reset: moving to checkpoint-2
# def5678 HEAD@{1}: commit: [task] implement
# ...

# Recover specific files
git checkout def5678 -- path/to/file.go

# Or see what was in that commit
git show def5678
```

## Scenario 5: Partial Recovery

### The Problem

The implementation changed 10 files, but only 2 are wrong:

```bash
mehr implement
# 8 files good, 2 files bad
```

### Option A: Fix Manually

If changes are small, just edit the files:

```bash
# Edit the problematic files manually
vim internal/api/handler.go

# Git handles the rest
git add .
git commit -m "manual: fix handler logic"
```

### Option B: Selective Checkout

Keep most changes, restore specific files:

```bash
# First, note the current commit
git rev-parse HEAD  # abc1234

# Undo
mehr undo

# Selectively restore good files from the undone commit
git checkout abc1234 -- internal/api/good1.go
git checkout abc1234 -- internal/api/good2.go
# ... etc

# Now re-implement just the problematic parts
mehr talk "Only fix the handler in handler.go"
mehr implement
```

## Best Practices

### 1. Review Before Acting

Always review after implementation:

```bash
mehr implement
git diff              # Review changes
# Then decide: keep, undo, or iterate
```

### 2. Use Status Frequently

```bash
mehr status
```

Know how many checkpoints you have before major operations.

### 3. Don't Fear Undo

Undo is safe and fast. Use it liberally:

```bash
mehr implement
# Hmm, not sure...
mehr undo
# Try different guidance
mehr talk "..."
mehr implement
# Still not right
mehr undo
# ...
```

### 4. Document Your Context

When iterating, your notes help recovery:

```bash
mehr talk "Attempting approach A: singleton pattern"
mehr implement
mehr undo
mehr talk "Approach A didn't work because X. Trying approach B: dependency injection"
mehr implement
```

### 5. Know When to Start Fresh

If you're many undos deep with contradictory notes:

```bash
mehr delete --yes
# Rewrite task with lessons learned
mehr start improved-task.md
```

## Quick Reference

| Situation          | Solution                                                |
| ------------------ | ------------------------------------------------------- |
| Bad implementation | `mehr undo` → `mehr talk` → `mehr implement`            |
| Want to compare    | `mehr undo` → try alternative → `mehr undo`/`mehr redo` |
| Accidental undo    | `mehr redo`                                             |
| Lost redo          | `git reflog` → `git checkout <hash> -- file`            |
| Partially bad      | Manual edit or selective checkout                       |
| Too messy          | `mehr delete` → start fresh                             |

## Next Steps

- [Checkpoints Concept](../concepts/checkpoints.md) - How checkpoints work
- [Iterative Development](tutorials/iterative-development.md) - Refine through iteration
- [undo/redo commands](../cli/undo-redo.md) - Command reference
