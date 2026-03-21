# Logs

View live output and chat history for the current task.

## Opening

Open the Logs panel from the sidebar or use the logs button in the project view. It loads as a modal overlay.

## Tabs

### Live Output

Real-time output from the current session. Lines are color-coded:

| Color | Meaning |
|-------|---------|
| Red | Lines containing "error" or "failed" |
| Yellow | Lines containing "warning" |
| Green | Lines containing "completed" or "success" |

Click **Clear** to reset the output buffer. Output auto-scrolls to the latest line.

### Chat History

Full chat history for the current task, loaded via `chat.history`. Each message shows:

| Field | Description |
|-------|-------------|
| Role | `user`, `assistant`, or `system` badge |
| Timestamp | Time the message was sent |
| Job ID | Associated job identifier (if applicable) |
| Content | Message text |

## Related

- [kvelmo logs](/cli/logs.md) — CLI equivalent
- [Chat](/web-ui/chat.md) — Interactive chat interface
