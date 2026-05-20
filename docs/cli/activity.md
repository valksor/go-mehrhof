# activity

View RPC activity log.

## Usage

```bash
kvelmo activity [flags]
```

## Options

| Flag            | Description                                                               |
| --------------- | ------------------------------------------------------------------------- |
| `--since`       | Time range (default: `1h`, e.g., `30m`, `24h`)                            |
| `--method`      | Filter by method pattern (pipe-separated, e.g., `start\|plan\|implement`) |
| `--errors-only` | Show only failed requests                                                 |
| `--limit`       | Maximum entries to return (default: `50`)                                 |
| `--json`        | Output as JSON                                                            |

## Examples

```bash
# View last hour of activity
kvelmo activity

# View errors from the last 24 hours
kvelmo activity --since 24h --errors-only

# Filter by method
kvelmo activity --method "plan|implement"

# JSON output for scripting
kvelmo activity --json
```

## Related

- [stats](/cli/stats.md) — Metrics overview
- [logs](/cli/logs.md) — Operation logs
