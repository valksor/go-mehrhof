# `--wait` flag reference

Block until the current job completes or fails.

## Description

The `wait` functionality is provided via the `--wait` / `-w` flag on workflow commands (`plan`, `implement`, `simplify`, `optimize`, `review`, `explain`). When enabled, the command connects to the worktree socket's event stream and blocks until the job completes or fails, streaming output in real time.

This page documents a shared flag reference, not a standalone `kvelmo wait` command.

Press Ctrl+C to detach without stopping the running job.

## Usage

```bash
kvelmo plan --wait
kvelmo implement -w
kvelmo simplify --wait
kvelmo optimize -w
```

## How It Works

1. Connects to the worktree socket via Unix domain socket
2. Sends a `stream.subscribe` JSON-RPC request
3. Streams events until `job_completed` or `job_failed` is received
4. Plays a terminal bell on completion or failure

## Event Types

| Event           | Behavior                       |
|-----------------|--------------------------------|
| `job_output`    | Printed to stdout              |
| `state_changed` | Printed as `[State] <message>` |
| `job_completed` | Terminal bell, exits success   |
| `job_failed`    | Terminal bell, exits with error|
| `error`         | Printed to stderr              |
| `heartbeat`     | Ignored (keepalive)            |

## Related

- [watch](/cli/watch.md) — Stream live output (standalone command)
- [status](/cli/status.md) — Check current task state
- [plan](/cli/plan.md) — Plan with `--wait` support
- [implement](/cli/implement.md) — Implement with `--wait` support
