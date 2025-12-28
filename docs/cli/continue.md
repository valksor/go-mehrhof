# mehr continue

Show status and suggested next actions for the current task.

## Synopsis

```bash
mehr continue
```

## Description

The `continue` command helps you resume work on a task after a break. It:

1. Displays the current task status
2. Shows task metadata (title, branch, specs, checkpoints)
3. Suggests the most appropriate next action based on the current state
4. Shows available undo/redo options

This is particularly useful when you return to a project and need to remember where you left off.

## Flags

This command has no specific flags. Global flags (`--verbose`, `--no-color`) are available.

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
  mehr talk       # Discuss the plan

Other options:
  mehr finish     # Complete and merge changes
  mehr delete     # Abandon task without merging
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
  mehr talk       # Discuss issues
  mehr undo       # Revert last change
  mehr finish     # Complete and merge

  mehr undo       # Revert to previous checkpoint (4 available)

Other options:
  mehr finish     # Complete and merge changes
  mehr delete     # Abandon task without merging
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
| `idle`         | `plan` (if no specs), `implement` (if specs exist) |
| `planning`     | `implement`, `talk`                                |
| `implementing` | `implement`, `talk`, `undo`, `finish`              |
| `reviewing`    | `finish`, `implement`                              |
| `done`         | `start` (new task)                                 |

## Difference from `mehr status`

| Command         | Purpose                                              |
| --------------- | ---------------------------------------------------- |
| `mehr continue` | Quick overview with actionable suggestions           |
| `mehr status`   | Detailed status with file listings and full metadata |

Use `continue` when you need guidance on what to do next. Use `status` when you need detailed information about the task state.

## See Also

- [status](cli/status.md) - Detailed task status
- [plan](cli/plan.md) - Create specifications
- [implement](cli/implement.md) - Implement specifications
- [undo/redo](cli/undo-redo.md) - Checkpoint management
