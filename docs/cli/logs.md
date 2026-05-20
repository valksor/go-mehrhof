# logs

Show agent activity log for the current task.

## Usage

```bash
kvelmo logs
```

## Description

Displays timestamped agent activity for the active task. Shows the same history as `chat history` but formatted as an activity log with prominent timestamps.

## Options

| Flag            | Description                                  |
| --------------- | -------------------------------------------- |
| `-n`, `--limit` | Number of messages to show (default: 50)     |
| `--full`        | Show full message content without truncation |
| `--json`        | Output raw JSON                              |

## Examples

```bash
# Show recent activity
kvelmo logs

# Show last 10 messages
kvelmo logs -n 10

# Show full messages without truncation
kvelmo logs --full

# JSON output
kvelmo logs --json
```

## Output

```
Activity log (42 messages, showing 42)
============================================================
10:15:32  USER    Add authentication middleware
10:15:45  AGENT   I'll analyze the codebase first to understand...
10:16:02  AGENT   Found the existing auth patterns in internal/auth...
```

## Related

- [watch](/cli/watch.md) — Stream live output
- [chat](/cli/chat.md) — Interactive agent conversation
