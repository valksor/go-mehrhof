# Recovery Guide

Recovery is part of normal kvelmo usage, not a rare failure path.

Because the workflow records checkpoints and keeps explicit task state, you usually recover by navigating state instead of improvising cleanup.

## First Response Checklist

When something looks wrong, start with:

```bash
kvelmo status
kvelmo watch
```

`status` shows a point-in-time snapshot; `watch` streams live updates. Together they tell you whether the task is progressing, waiting for input, paused, failed, or in a different state than expected.

## Common Recovery Commands

| Situation                            | Command          |
| ------------------------------------ | ---------------- |
| Implementation looks wrong           | `kvelmo undo`    |
| You undid too far                    | `kvelmo redo`    |
| Task is stuck or confused            | `kvelmo reset`   |
| A phase failed and should be retried | `kvelmo retry`   |
| Running work should stop             | `kvelmo stop`    |
| Running work should fail immediately | `kvelmo abort`   |
| You want to discard the task         | `kvelmo abandon` |

## Undo a Bad Result

If the current implementation or review state is not acceptable:

```bash
kvelmo undo
kvelmo status
```

Then either:

- change the task description or context and run forward again
- re-run the next phase
- inspect the task in the Web UI before proceeding

## Redo When Needed

If you undo and realize the previous state was actually correct:

```bash
kvelmo redo
```

## Reset a Stuck Task

Use `reset` when the task state is inconsistent or appears stuck.

```bash
kvelmo reset
```

This is the go-to command for general workflow-state problems. Use the situation-specific commands from the table above for targeted recovery.

## Retry a Failed Phase

If the task failed for a transient reason and the workflow should continue from the same point:

```bash
kvelmo retry
```

## Stop or Abort Running Work

Use:

- `stop` when you want to interrupt current execution cleanly
- `abort` when you want the task to move directly into a failed state

## Abandon a Task

If the task should be thrown away rather than recovered:

```bash
kvelmo abandon
```

Use this only when you want to discard the current path, not when you simply need to step back.

## Browser-Oriented Recovery

If you are working primarily in the Web UI, recovery still follows the same underlying model:

- inspect task state
- navigate checkpoints
- review logs and activity
- reset if the workflow state is unhealthy

The browser gives you more context, but the core recovery model is the same as the CLI.

## Typical Recovery Flow

```bash
kvelmo status
kvelmo undo
kvelmo watch
```

If that is not enough:

```bash
kvelmo reset
kvelmo retry
```

## When Recovery Is Not Enough

If the issue comes from environment setup rather than workflow state:

```bash
kvelmo diagnose
```

Also inspect:

- provider authentication
- configured agent path
- local server health
- repository/worktree state

## Related

- [Workflow Concepts](/concepts/workflow.md)
- [Web UI Getting Started](/web-ui/getting-started.md)
- [CLI Reference](/cli/index.md)
