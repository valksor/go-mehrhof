# mehr continue

Resume work on task.

## Synopsis

```bash
mehr continue [flags]
```

**Aliases:** `cont`, `c`

## Description

The `continue` command helps you resume work on a task after a break. It:

1. Displays the current task status
2. Shows task metadata (title, branch, specifications, checkpoints)
3. Suggests the most appropriate next action based on the current state
4. Shows available undo/redo options
5. With `--auto`, executes the next logical step automatically

This is particularly useful when you return to a project and need to remember where you left off.

## Flags

| Flag     | Type | Default | Description                             |
| -------- | ---- | ------- | --------------------------------------- |
| `--auto` | bool | false   | Auto-execute the next logical workflow step |

## Examples

### Resume Work on a Task

```bash
mehr continue
```

Output when task is in planning state:

```
Task: a1b2c3d4
Title: Add user authentication
State: planning
Branch: feature/AUTH-001--add-user-auth
Specifications: 2
Checkpoints: 1

Suggested next actions:
  mehr implement  # Start implementation
  mehr note       # Add notes

Other options:
  mehr finish     # Complete and merge changes
  mehr abandon    # Abandon task without merging
```

### After Implementation

```bash
mehr continue
```

Output when task has checkpoints:

```
Task: a1b2c3d4
Title: Add user authentication
State: implementing
Branch: feature/AUTH-001--add-user-auth
Specifications: 2
Checkpoints: 5

Suggested next actions:
  mehr implement  # Continue implementation
  mehr note       # Add notes
  mehr undo       # Revert last change
  mehr finish     # Complete and merge

  mehr undo       # Revert to previous checkpoint (4 available)

Other options:
  mehr finish     # Complete and merge changes
  mehr abandon    # Abandon task without merging
```

### No Active Task

```bash
mehr continue
```

Output when no task is active:

```
No active task found.

To start a new task:
  mehr start <file.md>       # From markdown file
  mehr start <directory/>    # From directory with README.md
```

### On Orphaned Task Branch

If you're on a task branch but the task was deleted:

```bash
mehr continue
```

Output:

```
On task branch: task/a1b2c3d4
But no active task found with ID: a1b2c3d4

The task may have been completed or deleted.
To start a new task, run: mehr start <reference>
```

## State-Based Suggestions

The suggested actions depend on the current task state:

| State          | Primary Suggestions                                |
| -------------- | -------------------------------------------------- |
| `idle`         | `plan` (if no specifications), `implement` (if specifications exist) |
| `planning`     | `implement`, `note`                                |
| `implementing` | `implement`, `note`, `undo`, `finish`              |
| `reviewing`    | `finish`, `implement`                              |
| `done`         | `start` (new task)                                 |

## Choosing the Right Command

| Command         | When to Use                                                    |
| --------------- | -------------------------------------------------------------- |
| `mehr guide`    | "What should I do next?" (fastest, minimal output)             |
| `mehr status`   | "Show full task details" (full inspection, all details)        |
| `mehr continue` | "Resume work on task" (`--auto` runs next step)                |

Use `guide` for quick suggestions. Use `status` for full details. Use `continue` when resuming work with optional auto-execution.

## See Also

- [status](cli/status.md) - Detailed task status
- [plan](cli/plan.md) - Create specifications
- [implement](cli/implement.md) - Implement specifications
- [undo](cli/undo.md) / [redo](cli/redo.md) - Checkpoint management
