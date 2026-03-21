# kvelmo simplify

Optional: Simplify code for clarity after implementation.

## Usage

```bash
kvelmo simplify
```

## Options

| Flag | Short | Description |
|------|-------|-------------|
| `--wait` | `-w` | Wait for job to complete, streaming output |

## Prerequisites

- Task must be in `implemented` state

## What Happens

1. Agent reviews the implemented code
2. Refactors for readability and clarity
3. Creates a checkpoint
4. Returns to `implemented` state

## Examples

```bash
kvelmo implement
kvelmo simplify  # Optional cleanup
kvelmo review
```

## When to Use

- Code is correct but hard to read
- Want cleaner variable names
- Need better structure

Also in Web UI: [Implementation Phase](/web-ui/implementing.md).

## Related

- [implement](/cli/implement.md) — Implement first
- [optimize](/cli/optimize.md) — Alternative: optimize
