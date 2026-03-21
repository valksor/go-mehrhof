# kvelmo list

List all tasks in the workspace.

## Usage

```bash
kvelmo list
```

## Options

| Flag | Description |
|------|-------------|
| `--history` | Search archived task history for the current project |
| `--search` | Filter history by keyword (matches title, branch, source) |
| `--tag` | Filter history by tag |
| `--since` | Show tasks completed after this date (RFC3339 or YYYY-MM-DD) |
| `--until` | Show tasks completed before this date (RFC3339 or YYYY-MM-DD) |
| `--state` | Filter by final state (e.g., `finished`, `abandoned`) |
| `--limit` | Maximum number of results (0 = unlimited) |
| `--file` | Filter by file path touched (substring match) |

## Subcommands

### `list history`

Show completed/archived task history for the current project.

| Flag | Short | Description |
|------|-------|-------------|
| `--json` | | Output as JSON |
| `--search` | `-s` | Filter by keyword (uses task.search RPC) |
| `--tag` | | Filter by tag |
| `--since` | | Show tasks completed after this date (RFC3339 or YYYY-MM-DD) |
| `--until` | | Show tasks completed before this date (RFC3339 or YYYY-MM-DD) |
| `--state` | | Filter by final state (e.g., `finished`, `abandoned`) |
| `--limit` | | Maximum number of results (0 = unlimited) |
| `--file` | | Filter by file path touched (substring match) |

### `list search <query>`

Search archived tasks by keyword.

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON |
| `--tag` | Filter by tag |
| `--since` | Show tasks completed after this date (RFC3339 or YYYY-MM-DD) |
| `--until` | Show tasks completed before this date (RFC3339 or YYYY-MM-DD) |
| `--state` | Filter by final state (e.g., `finished`, `abandoned`) |
| `--limit` | Maximum number of results (0 = unlimited) |
| `--file` | Filter by file path touched (substring match) |

## Output

```
Tasks:
  #1 [implemented] Add user auth (github:owner/repo#123)
  #2 [planned] Fix login bug (file:task.md)
  #3 [submitted] Update docs (github:owner/repo#456)
```

## Examples

```bash
# List all active tasks
kvelmo list

# JSON output
kvelmo list --json

# Browse task history
kvelmo list history

# Search history for a keyword
kvelmo list search "auth"

# Filter history by date and state
kvelmo list history --since 2026-01-01 --state finished

# Filter by file touched
kvelmo list history --file "pkg/auth"
```

## Related

- [status](/cli/status.md) — Current task details
- [projects](/cli/projects.md) — Project registry
