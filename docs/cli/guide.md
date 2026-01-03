# mehr guide

What should I do next?

## Synopsis

```bash
mehr guide
```

## Description

The `guide` command analyzes your current context (active task, state, specifications, pending questions) and suggests the most appropriate next action. This is useful when you're unsure what to do next or when returning to work after a break.

## When to Use

- **After a break** - Quickly see what you were working on and what to do next
- **Before running commands** - Get context-aware suggestions instead of guessing
- **When blocked** - See the current state and available options
- **In worktrees** - Auto-detects which task you're working on

## Examples

### No Active Task

```bash
$ mehr guide

No active task.

Suggested actions:
  mehr start <reference>   # Start a new task
  mehr status --all          # View all tasks in workspace
```

### Task in Planning State

```bash
$ mehr guide

Task: a1b2c3d4
Title: Add user authentication
State: planning

Specifications: 2

Suggested next actions:
  mehr implement
  mehr plan
  mehr note
```

### Pending Agent Question

```bash
$ mehr guide

Task: a1b2c3d4
Title: Add user authentication
State: waiting

Specifications: 1

⚠️  The AI has a question for you:
  Which authentication method would you prefer: JWT or session-based?
  Options:
    1. JWT
    2. Session-based
    3. OAuth2

Suggested action:
  mehr answer "your response" # Respond to the question
  mehr note
```

### Task Done

```bash
$ mehr guide

Task: a1b2c3d4
Title: Add user authentication
State: done

Specifications: 3

Suggested next actions:
  Task is complete!
  mehr start <reference>
```

## State-Based Suggestions

The `guide` command provides different suggestions based on the current workflow state:

| State          | Suggestions                                                      |
| -------------- | ---------------------------------------------------------------- |
| `idle`         | `plan`, `note` (no specifications) or `implement`, `finish` (specifications exist) |
| `planning`     | `status`, `note`                                                |
| `implementing` | `status`, `note`, `undo`, `finish`                              |
| `reviewing`    | `status`, `finish`, `implement`                                 |
| `done`         | Start new task                                                  |
| `waiting`      | `answer` (respond to question)                                  |
| `failed`       | `status`, `note`, start new task                                |

## Worktree Support

When running inside a git worktree, `guide` automatically detects which task is associated with that worktree:

```bash
$ cd ../project-worktrees/a1b2c3d4
$ mehr guide

Task: a1b2c3d4
Title: Add authentication
State: implementing
...
```

## Choosing the Right Command

| Command         | When to Use                                                    |
| --------------- | -------------------------------------------------------------- |
| `mehr guide`    | "What should I do next?" (fastest, minimal output)             |
| `mehr status`   | "Show full task details" (full inspection, all details)        |
| `mehr continue` | "Resume work on task" (`--auto` runs next step)                |

## See Also

- [status](status.md) - Detailed task status
- [note](note.md) - Add notes to the task
- [Workflow](../concepts/workflow.md) - Understanding states
