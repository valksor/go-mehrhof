# mehr abandon

Abandon the current task without merging.

## Synopsis

```bash
mehr abandon [flags]
```

## Description

The `abandon` command discards the current task completely. It removes:

- The task branch (if created)
- The work directory
- The active task reference

Use this when you want to discard work without merging.

## Flags

| Flag            | Short | Type | Default | Description              |
| --------------- | ----- | ---- | ------- | ------------------------ |
| `--yes`         | `-y`  | bool | false   | Skip confirmation prompt |
| `--keep-branch` |       | bool | false   | Keep the git branch      |
| `--keep-work`   |       | bool | false   | Keep the work directory  |

## Examples

### Abandon with Confirmation

```bash
mehr abandon
```

Output:

```
Task: a1b2c3d4
Branch: task/a1b2c3d4
Work: .mehrhof/work/a1b2c3d4/

This will delete:
  - Git branch task/a1b2c3d4
  - Work directory and all contents
  - 2 specification files
  - 4 session logs

Are you sure? [y/N] y

Abandoning task...
  Switched to: main
  Branch deleted: task/a1b2c3d4
  Work directory removed
Task abandoned.
```

### Skip Confirmation

```bash
mehr abandon --yes
```

Skip the confirmation prompt.

### Keep Branch

```bash
mehr abandon --keep-branch
```

Only remove the work directory. Branch remains for manual inspection.

### Keep Work Directory

```bash
mehr abandon --keep-work
```

Only delete the branch. Work directory remains for reference.

## What Gets Deleted

### By Default

| Item             | Location                             | Deleted |
| ---------------- | ------------------------------------ | ------- |
| Git branch       | `task/<id>`                          | Yes     |
| Work directory   | `.mehrhof/work/<id>/`                | Yes     |
| Specifications   | `.mehrhof/work/<id>/specifications/` | Yes     |
| Session logs     | `.mehrhof/work/<id>/sessions/`       | Yes     |
| Notes            | `.mehrhof/work/<id>/notes.md`        | Yes     |
| Active reference | `.mehrhof/.active_task`              | Cleared |

**Note:** The default behavior for work directory deletion can be configured in `config.yaml`:

```yaml
workflow:
  delete_work_on_abandon: false   # Keep work dirs on abandon (default: true)
```

**Precedence:** CLI flag (`--keep-work`) > config (`delete_work_on_abandon`) > default (`true`)

### With --keep-branch

Branch preserved, everything else deleted.

### With --keep-work

Branch deleted, work directory preserved.

## When to Abandon

- The approach didn't work out
- Requirements changed significantly
- Task is no longer needed
- Want to start fresh

## Starting Fresh

After abandoning, start a new task:

```bash
mehr abandon --yes
mehr start task.md  # Fresh start
```

## Recovering Abandoned Work

### If Branch Still Exists

```bash
git checkout task/a1b2c3d4
```

### If Branch Was Deleted

```bash
# Find deleted branch in reflog
git reflog

# Recover branch
git checkout -b task/a1b2c3d4 <commit-hash>
```

### Work Directory

Once deleted, work directory contents cannot be recovered through Mehrhof. Use filesystem recovery tools if needed.

## Error Handling

### No Active Task

```
Error: No active task to abandon
Start a task first with 'mehr start'
```

### Uncommitted Changes

```
Warning: Working directory has uncommitted changes
These will be lost. Continue? [y/N]
```

### Protected Branch

```
Error: Cannot delete branch 'main'
This appears to be a protected branch.
```

## See Also

- [finish](cli/finish.md) - Complete with merge
- [start](cli/start.md) - Begin a new task
- [undo-redo](cli/undo-redo.md) - Revert instead of abandon
