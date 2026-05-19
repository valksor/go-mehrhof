# kvelmo watch

Stream live task output to the terminal.

## Usage

```bash
kvelmo watch
```

## Description

Connects to the worktree socket and subscribes to the event stream, displaying agent output, state changes, and errors in real time.

Press Ctrl+C to stop watching without affecting the running task.

## Options

| Flag     | Description                     |
| -------- | ------------------------------- |
| `--json` | Output raw JSON events (NDJSON) |

## Examples

```bash
# Watch formatted output
kvelmo watch

# Watch raw JSON events
kvelmo watch --json
```

## Event Types

| Event           | Description            |
| --------------- | ---------------------- |
| `job_output`    | Agent output text      |
| `state_changed` | Task state transition  |
| `job_failed`    | Job failure with error |
| `error`         | General error          |
| `heartbeat`     | Keepalive (ignored)    |

## Related

- [status](/cli/status.md) — Check current state
- [logs](/cli/logs.md) — View activity history
