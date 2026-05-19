# kvelmo delete

Delete a terminal task from the registry.

## Usage

```bash
kvelmo delete
```

## Options

| Flag              | Description                |
| ----------------- | -------------------------- |
| `--delete-branch` | Also delete the git branch |

## Prerequisites

- Task must be in terminal state (`submitted` or `failed`)

## What Happens

1. Task is removed from registry
2. Branch may be kept or deleted
3. Local files remain

## When to Use

- Task is complete and you want to clean up
- Task failed and is no longer needed

## Related

- [abandon](/cli/abandon.md) — Delete active task
- [list](/cli/list.md) — See all tasks
