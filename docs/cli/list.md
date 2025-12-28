# mehr list

List all tasks in the workspace.

## Synopsis

```bash
mehr list [flags]
```

## Description

The `list` command displays all tasks in the workspace with their worktree paths and states. This is particularly useful when running multiple parallel tasks across different terminals.

**Features:**

- Shows all tasks regardless of which directory you're in
- Displays worktree paths for parallel task management
- Indicates which task is active in the main repo (`*`)
- Indicates which worktree you're currently in (`→`)

## Flags

| Flag          | Short | Type | Default | Description                    |
| ------------- | ----- | ---- | ------- | ------------------------------ |
| `--worktrees` | `-w`  | bool | false   | Show only tasks with worktrees |

## Examples

### List All Tasks

```bash
mehr list
```

Output:

```
TASK ID     STATE           TITLE                    WORKTREE                         ACTIVE
a1b2c3d4    implementing    Add authentication       ../project-worktrees/a1b2c3d4    →
e5f6g7h8    planning        Fix database queries     ../project-worktrees/e5f6g7h8
c9d0e1f2    idle            Update config            -                                *
f3g4h5i6    done            Refactor logging         -

Legend: * = active task in main repo, → = current worktree
```

### List Only Worktree Tasks

```bash
mehr list --worktrees
```

Output:

```
TASK ID     STATE           TITLE                    WORKTREE                         ACTIVE
a1b2c3d4    implementing    Add authentication       ../project-worktrees/a1b2c3d4    →
e5f6g7h8    planning        Fix database queries     ../project-worktrees/e5f6g7h8

Legend: * = active task in main repo, → = current worktree
```

## Output Columns

| Column   | Description                                                 |
| -------- | ----------------------------------------------------------- |
| TASK ID  | Unique 8-character task identifier                          |
| STATE    | Current workflow state (idle, planning, implementing, etc.) |
| TITLE    | Task title from source file                                 |
| WORKTREE | Path to worktree, or `-` if none                            |
| ACTIVE   | `*` for active task in main repo, `→` for current worktree  |

## Use Cases

### Managing Parallel Tasks

When working on multiple features simultaneously:

```bash
# See what's running
mehr list

# Check specific task
cd ../project-worktrees/a1b2c3d4
mehr status
```

### Finding Your Worktrees

```bash
mehr list --worktrees
```

Shows only tasks with worktrees, making it easy to navigate between parallel tasks.

### From Any Location

The `list` command works from:

- Main repository
- Any worktree
- Any subdirectory within the project

It always shows all tasks in the workspace.

## Related Commands

- [status](cli/status.md) - Show detailed status of current task
- [start](cli/start.md) - Start a new task (with `--worktree` for parallel)
- [finish](cli/finish.md) - Complete a task and clean up worktree

## See Also

- [Parallel Tasks](../README.md#parallel-tasks) - Overview of parallel task workflow
