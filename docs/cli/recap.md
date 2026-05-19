# kvelmo recap

Summarize current task state for resuming work.

## Usage

```bash
kvelmo recap [flags]
```

## Description

Shows a concise recap of the current task: where you are in the workflow, what changed, and what to do next. Useful when returning to a task after a break.

The recap includes:

- Current workflow state
- Task title, source, and branch
- Tags (if any)
- Last activity timestamp
- Checkpoint count and latest checkpoint
- Files changed (up to 10 shown)
- Phases completed
- Last error (if any)
- Suggested next action

## Options

| Flag     | Description              |
| -------- | ------------------------ |
| `--json` | Output raw JSON response |

## Examples

```bash
# Show task recap
kvelmo recap

# Machine-readable output
kvelmo recap --json
```

## Output

```
State: Implemented
Task:  Add user authentication
Source: github#42
Branch: kvelmo/add-user-auth
Last activity: 2m ago
Checkpoints: 3 (latest: a1b2c3d4 — implement complete)
Files changed: 5
  A src/auth/handler.go
  A src/auth/middleware.go
  M src/server/routes.go
  A src/auth/handler_test.go
  M go.mod
Phases completed: plan, implement

Next: kvelmo simplify
```

## Web UI

The RecapWidget on the dashboard provides the same information with real-time updates.

## Related

- [status](/cli/status.md) — Show current workflow state
- [show](/cli/show.md) — Display specification or plan content
- [checkpoints](/cli/checkpoints.md) — List git checkpoints
