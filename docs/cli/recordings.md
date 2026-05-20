# recordings

Manage agent interaction recordings.

## Usage

```bash
kvelmo recordings <subcommand>
```

## Description

View and manage recordings of agent interactions. Recordings are JSONL files that capture all communication between kvelmo and AI agents, useful for debugging and auditing.

## Global Options

| Flag    | Description                                                  |
| ------- | ------------------------------------------------------------ |
| `--dir` | Recordings directory (default: ~/.valksor/kvelmo/recordings) |

## Subcommands

### list

List all recordings.

```bash
kvelmo recordings list
```

| Flag      | Description                                    |
| --------- | ---------------------------------------------- |
| `--job`   | Filter by job ID                               |
| `--since` | Show recordings since duration (e.g., 24h, 7d) |
| `--json`  | Output as JSON                                 |

### view

View a recording file.

```bash
kvelmo recordings view <file>
```

### replay

Replay a recording with filtering.

```bash
kvelmo recordings replay <file>
```

| Flag       | Description                            |
| ---------- | -------------------------------------- |
| `--filter` | Filter by event type (comma-separated) |

### clean

Remove old recordings.

```bash
kvelmo recordings clean
```

| Flag           | Description                                          |
| -------------- | ---------------------------------------------------- |
| `--older-than` | Remove recordings older than duration (default: 30d) |

## Examples

```bash
# List recent recordings
kvelmo recordings list --since 24h

# View a specific recording
kvelmo recordings view 2026-03-12-abc123.jsonl

# Replay with filter
kvelmo recordings replay 2026-03-12-abc123.jsonl --filter "tool_call,tool_result"

# Clean old recordings
kvelmo recordings clean --older-than 7d
```

## Related

- [logs](/cli/logs.md) — View activity log
- [diagnose](/cli/diagnose.md) — System diagnostics
