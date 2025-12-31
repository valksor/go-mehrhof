# Task Issues

Problems with task management and workflow.

## "No active task"

**Cause:** No task started or not on task branch.

**Solution:**

```bash
# Start a new task
mehr start file:task.md

# Or switch to existing task branch
git branch -a | grep task/
git checkout task/abc12345
```

## "Task already exists"

**Cause:** Task with same source already registered.

**Solution:**

```bash
# Delete existing task
mehr abandon --yes

# Start fresh
mehr start file:task.md
```

## "Cannot find task"

**Cause:** Work directory missing or corrupted.

**Solution:**

```bash
# Check if work directory exists
ls .mehrhof/work/

# If missing, start fresh
mehr start file:task.md
```

## "No specifications generated"

**Cause:** Agent couldn't understand requirements.

**Solution:**

```bash
# Add more detail to task file
vim task.md

# Be specific about requirements
# Include examples

# Try again
mehr plan
```

## "Specs are incomplete"

**Cause:** Requirements too vague.

**Solution:**

```bash
# Add clarification
mehr note "Include error handling for all edge cases"

# Regenerate
mehr plan
```

## "No SPEC files found"

**Cause:** Planning phase not run.

**Solution:**

```bash
# Run planning first
mehr plan

# Then implement
mehr implement
```

## "Implementation timeout"

**Cause:** Task too complex or slow API.

**Solution:**

Increase timeout in `.mehrhof/config.yaml`:

```yaml
agent:
  timeout: 900 # 15 minutes
```

Or break into smaller specs:

```bash
mehr note "Focus on just the API handler first"
mehr plan
```

## "Generated code has errors"

**Cause:** Agent misunderstood requirements.

**Solution:**

```bash
# Undo
mehr undo

# Provide more context
mehr note "Use the existing error handling pattern in internal/errors/"

# Try again
mehr implement
```

## "Cannot undo: no checkpoints"

**Cause:** At initial state or no checkpoints created.

**Solution:**

```bash
# Check checkpoint status
mehr status

# If truly at initial state, nothing to undo
# Start fresh if needed
mehr abandon
mehr start file:task.md
```

## "Cannot redo: nothing to redo"

**Cause:** Redo stack empty (new changes cleared it).

**Solution:**

```bash
# Check git reflog
git reflog

# Find the commit you want
git show abc1234

# Restore specific files
git checkout abc1234 -- path/to/file
```
