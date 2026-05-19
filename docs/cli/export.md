# kvelmo export

Export task history and metrics.

## Usage

```bash
kvelmo export [flags]
```

## Options

| Flag        | Description                                                                                 |
| ----------- | ------------------------------------------------------------------------------------------- |
| `--format`  | Output format: `json` or `csv` (default: `json`)                                            |
| `--since`   | Time range (e.g., `7d`, `30d`)                                                              |
| `--include` | Data to include, comma-separated: `tasks`, `metrics`, `activity` (default: `tasks,metrics`) |

## Examples

```bash
# Export all data as JSON
kvelmo export

# Export last 7 days as CSV
kvelmo export --format csv --since 7d

# Export only metrics
kvelmo export --include metrics

# Export tasks and activity
kvelmo export --include tasks,activity --since 30d
```

## Related

- [report](/cli/report.md) — Compliance reports
- [stats](/cli/stats.md) — Real-time metrics
- [activity](/cli/activity.md) — Activity log
