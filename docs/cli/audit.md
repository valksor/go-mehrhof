# audit

View a compliance-focused audit trail of all kvelmo actions.

## Usage

```bash
kvelmo audit [flags]
```

## Flags

| Flag      | Default | Description                     |
| --------- | ------- | ------------------------------- |
| `--since` | `7d`    | Time range (e.g., 24h, 7d, 30d) |
| `--json`  | `false` | Output as JSON                  |

## Examples

```bash
# View last 7 days of activity
kvelmo audit

# View last 30 days
kvelmo audit --since 30d

# Export as JSON for compliance tools
kvelmo audit --json > audit-report.json
```

## Output

```
Compliance Audit Report (since 7d)
═══════════════════════════════════════════════════════════

Active Tasks (2)
  task-abc123                              implementing
  task-def456                              reviewing

Activity Log (45 entries)
  2026-03-15 10:23:45  jsmith        task.start                      12ms
  2026-03-15 10:24:01  jsmith        task.plan                       85ms
  2026-03-15 10:30:15  jsmith        task.implement                 120ms
```

## What's Logged

Each entry includes:

- **Timestamp** — when the action occurred
- **User ID** — OS username of the person who invoked the action
- **Method** — the RPC method called
- **Duration** — how long the operation took
- **Error** — error message if the operation failed

## Prerequisites

- Activity logging must be enabled: `kvelmo config set storage.activity_log.enabled true`
- The global socket must be running: `kvelmo serve`

## Related

- [activity](/cli/activity.md) — Lower-level activity log queries
- [export](/cli/export.md) — Export data in JSON or CSV
