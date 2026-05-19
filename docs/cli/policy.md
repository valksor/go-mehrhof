# kvelmo policy

Workflow policy management.

## Usage

```bash
kvelmo policy <subcommand>
```

## Subcommands

| Subcommand | Description                                  |
| ---------- | -------------------------------------------- |
| `check`    | Check current task against workflow policies |

## Options

| Flag     | Description    |
| -------- | -------------- |
| `--json` | Output as JSON |

## Examples

```bash
# Check policy compliance
kvelmo policy check

# JSON output
kvelmo policy check --json
```

## What It Checks

Evaluates the current task state against configured workflow policies and reports any violations.

## Related

- [review](/cli/review.md) — Review process
- [approve](/cli/approve.md) — Approve transitions
