# update

Re-fetch task from provider.

## Usage

```bash
kvelmo update
```

## What Happens

1. Task is fetched from provider again
2. Description is updated
3. Delta specification may be generated

## When to Use

- Task description changed in provider
- Want latest requirements
- Need to sync with upstream

## Prerequisites

- Task must have a provider source (not file)

## Related

- [start](/cli/start.md) — Start a task
- [Providers](/providers/index.md) — Task sources
