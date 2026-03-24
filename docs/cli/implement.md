# kvelmo implement

Execute the specification and make code changes.

## Usage

```bash
kvelmo implement
```

## Options

| Flag | Short | Description |
|------|-------|-------------|
| `--dry-run` | | Simulate without executing agent |
| `--wait` | `-w` | Wait for job to complete, streaming output |
| `--json` | | Output result as JSON |

## Prerequisites

- Task must be in `loaded`, `planned`, `implemented`, or `submitted` state
- From `loaded`: skips planning and uses the task description as specification
- From `implemented` or `submitted`: re-runs implementation (re-entry)

## Examples

```bash
# Execute specification
kvelmo implement

# Force re-implement
kvelmo implement --force
```

## What Happens

1. Agent reads the specification
2. Agent modifies code files
3. Agent may run tests
4. A git checkpoint is created
5. State transitions to `implemented`

## Watching Progress

The agent's work streams in real-time:
- Files being read and modified
- Code changes being made
- Tool calls (writes, terminal commands)

## Optional Post-Processing

After implementation, you can optionally run:

```bash
# Simplify code for clarity
kvelmo simplify

# Optimize code quality
kvelmo optimize
```

## If Implementation Fails

Use undo to revert:
```bash
kvelmo undo
```

Then adjust the specification or task description.

Also in Web UI: [Implementation Phase](/web-ui/implementing.md).

## Related

- [plan](/cli/plan.md) — Plan before implementing
- [simplify](/cli/simplify.md) — Optional cleanup
- [optimize](/cli/optimize.md) — Optional optimization
- [review](/cli/review.md) — Review the implementation
