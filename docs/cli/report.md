# kvelmo report

Generate compliance report.

## Usage

```bash
kvelmo report [flags]
```

## Options

| Flag | Description |
|------|-------------|
| `--format` | Output format: `md` or `json` (default: `md`) |
| `--since` | Time range (default: `30d`, e.g., `7d`, `90d`) |

## Examples

```bash
# Generate markdown report for last 30 days
kvelmo report

# JSON report for last week
kvelmo report --format json --since 7d

# Quarterly report
kvelmo report --since 90d
```

## What It Includes

- Task summaries
- AI usage metrics
- Activity statistics
- Completion rates

## Related

- [export](/cli/export.md) — Data export
- [stats](/cli/stats.md) — Real-time metrics
