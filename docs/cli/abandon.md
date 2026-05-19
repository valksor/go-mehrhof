# kvelmo abandon

Full cleanup: stop task, delete branch, clean work directory.

## Usage

```bash
kvelmo abandon
```

## Options

| Flag            | Description                          |
| --------------- | ------------------------------------ |
| `--keep-branch` | Keep the git branch after abandoning |

## What Happens

1. Current task stops
2. Task branch is deleted
3. `.kvelmo/` files are cleaned
4. State returns to `none`

## Warning

This is destructive. Work on this task will be lost.

## When to Use

- Want to start completely fresh
- Task is no longer needed
- Major mistake, want full reset

## Recovery

Work may still be in git history:

```bash
git reflog
git checkout <commit-sha>
```

## Related

- [abort](/cli/abort.md) — Stop without cleanup
- [reset](/cli/reset.md) — Recover state only
