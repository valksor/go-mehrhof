# agent

Agent management.

## Usage

```bash
kvelmo agent <subcommand>
```

## Subcommands

| Subcommand | Description                         |
| ---------- | ----------------------------------- |
| `status`   | Check agent availability and health |

## Options

| Flag     | Description    |
| -------- | -------------- |
| `--json` | Output as JSON |

## Examples

```bash
# Check agent health
kvelmo agent status

# JSON output
kvelmo agent status --json
```

## Output

```
Agent: available
Checks:
  ✓ claude binary found
  ✓ API key configured
```

## Related

- [workers](/cli/workers.md) — Worker pool management
- [Agents](/agents/index.md) — Agent documentation
