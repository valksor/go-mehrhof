# kvelmo strategy

Manage agent reasoning strategies.

## Subcommands

### `strategy list`

List all available agent reasoning strategies.

```bash
kvelmo strategy list
kvelmo strategy list --json
```

**Output:**

```
Available strategies:
  • direct
  • iterative
```

## Flags

| Flag     | Description    |
| -------- | -------------- |
| `--json` | Output as JSON |

## Related

- [Strategies](/agents/strategies.md) — Strategy configuration guide
- [agent](/cli/agent.md) — Agent status and health
- [config](/cli/config.md) — Configure per-phase strategy overrides
