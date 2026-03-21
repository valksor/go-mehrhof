# kvelmo optimize

Optional: Optimize code quality after implementation.

## Usage

```bash
kvelmo optimize
```

## Options

| Flag | Short | Description |
|------|-------|-------------|
| `--wait` | `-w` | Wait for job to complete, streaming output |

## Prerequisites

- Task must be in `implemented` state

## What Happens

1. Agent reviews the implemented code
2. Optimizes for performance and quality
3. Creates a checkpoint
4. Returns to `implemented` state

## Examples

```bash
kvelmo implement
kvelmo optimize  # Optional optimization
kvelmo review
```

## When to Use

- Code is correct but slow
- Want performance improvements
- Need better algorithms

Also in Web UI: [Implementation Phase](/web-ui/implementing.md).

## Related

- [implement](/cli/implement.md) — Implement first
- [simplify](/cli/simplify.md) — Alternative: simplify
