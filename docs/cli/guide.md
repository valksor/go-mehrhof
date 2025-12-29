# mehr guide

Show context-aware next actions based on the current task state.

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
  mehr implement              # Implement the specifications
  mehr plan                  # Create more specifications
  mehr chat                  # Discuss the plan
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
  mehr chat "your answer"    # Respond to the question
  mehr chat                   # Enter interactive mode
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
  mehr start <reference>    # Start a new task
```

## State-Based Suggestions

The `guide` command provides different suggestions based on the current workflow state:

| State          | Suggestions                                                      |
| -------------- | ---------------------------------------------------------------- |
| `idle`         | `plan`, `chat` (no specs) or `implement`, `finish` (specs exist) |
| `planning`     | `status`, `chat`                                                |
| `implementing` | `status`, `chat`, `undo`, `finish`                              |
| `reviewing`    | `status`, `finish`, `implement`                                 |
| `done`         | Start new task                                                  |
| `waiting`      | `chat` (respond to question)                                    |
| `dialogue`     | `chat` (continue conversation)                                  |
| `failed`       | `status`, `chat`, start new task                                |

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

## Comparison with Other Commands

| Command         | Purpose                                                        |
| --------------- | -------------------------------------------------------------- |
| `mehr guide`    | Quick, lightweight next-action suggestions                     |
| `mehr status`   | Detailed state inspection (specs, checkpoints, sessions)        |
| `mehr continue` | Status display with optional auto-execution (--auto flag)      |

## See Also

- [status](status.md) - Detailed task status
- [chat](chat.md) - Discuss with the agent
- [Workflow](../concepts/workflow.md) - Understanding states
