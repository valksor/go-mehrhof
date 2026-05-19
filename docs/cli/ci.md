# kvelmo ci

CI pipeline operations.

## Usage

```bash
kvelmo ci <subcommand>
```

## Subcommands

| Subcommand | Description                              |
| ---------- | ---------------------------------------- |
| `status`   | Show CI status for the current task's PR |

## Options

| Flag     | Description    |
| -------- | -------------- |
| `--json` | Output as JSON |

## Examples

```bash
# Check CI status
kvelmo ci status

# JSON output
kvelmo ci status --json
```

## Related

- [submit](/cli/submit.md) — Submit a PR
- [status](/cli/status.md) — Task status
