# Tutorial: Recovering from Mistakes

Learn how to use checkpoints to safely experiment and recover from errors.

## Understanding Checkpoints

Mehrhof creates checkpoints at key moments:

- After `mehr plan` (specifications created)
- After `mehr implement` (code generated)
- After `mehr note` (if files changed)

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
mehr undo

mehr note "Use the repository pattern like in internal/users/. The current approach has tight coupling."

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
mehr implement
```

Not sure if it's the best approach?

```bash
mehr undo

mehr note "Try a functional approach instead of OOP"
mehr implement
```

### Comparing

Now compare both:

```bash
git diff HEAD~1

mehr undo
mehr redo
git diff HEAD~1

mehr undo
mehr redo
```

### Warning

Making new changes after undo clears the redo stack:

```bash
mehr undo
mehr implement
mehr redo
```

## Scenario 3: Multiple Undo Steps

### Deep Recovery

Sometimes you need to go back multiple steps:

```bash
mehr plan
mehr implement
mehr note "add tests"
mehr implement
mehr note "fix bug"
mehr implement
```

To get back to checkpoint 2:

```bash
mehr undo
mehr undo
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
mehr implement
mehr undo
```

### The Solution

```bash
mehr redo
```

### If You Made Changes

If you accidentally made changes after undo:

```bash
mehr implement
mehr undo
mehr note "something"   # This clears redo!
mehr redo
```

Use git reflog to recover:

```bash
git reflog


git checkout def5678 -- path/to/file.go

git show def5678
```

## Scenario 5: Partial Recovery

### The Problem

The implementation changed 10 files, but only 2 are wrong:

```bash
mehr implement
```

### Option A: Fix Manually

If changes are small, just edit the files:

```bash
vim internal/api/handler.go

git add .
git commit -m "manual: fix handler logic"
```

### Option B: Selective Checkout

Keep most changes, restore specific files:

```bash
git rev-parse HEAD

mehr undo

git checkout abc1234 -- internal/api/good1.go
git checkout abc1234 -- internal/api/good2.go

mehr note "Only fix the handler in handler.go"
mehr implement
```

## Best Practices

### 1. Review Before Acting

Always review after implementation:

```bash
mehr implement
git diff
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
mehr undo
mehr note "..."
mehr implement
mehr undo
```

### 4. Document Your Context

When iterating, your notes help recovery:

```bash
mehr note "Attempting approach A: singleton pattern"
mehr implement
mehr undo
mehr note "Approach A didn't work because X. Trying approach B: dependency injection"
mehr implement
```

### 5. Know When to Start Fresh

If you're many undos deep with contradictory notes:

```bash
mehr abandon --yes
mehr start improved-task.md
```

## Quick Reference

| Situation          | Solution                                                |
| ------------------ | ------------------------------------------------------- |
| Bad implementation | `mehr undo` → `mehr note` → `mehr implement`            |
| Want to compare    | `mehr undo` → try alternative → `mehr undo`/`mehr redo` |
| Accidental undo    | `mehr redo`                                             |
| Lost redo          | `git reflog` → `git checkout <hash> -- file`            |
| Partially bad      | Manual edit or selective checkout                       |
| Too messy          | `mehr abandon` → start fresh                             |

## Next Steps

- [Checkpoints Concept](../concepts/checkpoints.md) - How checkpoints work
- [Iterative Workflow](iterative-workflow.md) - Refine through iteration
- [undo command](../cli/undo.md) - Undo command reference
- [redo command](../cli/redo.md) - Redo command reference
