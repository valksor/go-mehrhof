# mehr status

Show detailed task state (specs, checkpoints, sessions).

## Synopsis

```bash
mehr status [flags]
```

**Aliases:** `st`

## Description

The `status` command displays information about the active task including:

- Task ID and state
- Source reference
- Specifications
- Checkpoints
- Current branch
- Worktree path (if applicable)

**Context-aware:** When run from within a worktree, automatically shows the task associated with that worktree.

## Flags

| Flag     | Short | Type | Default | Description                              |
| -------- | ----- | ---- | ------- | ---------------------------------------- |
| `--all`  |       | bool | false   | Show all tasks in workspace              |
| `--json` |       | bool | false   | Output as JSON for programmatic use      |

**Note:** The `--json` flag is useful for scripting and integration with other tools.

## Examples

### Active Task Status

```bash
mehr status
```

Output:

```
Active Task: a1b2c3d4
  Title:   Add user authentication
  State:   implementing - Generating code from specifications
  Source:  task.md
  WorkDir: .mehrhof/work/a1b2c3d4
  Started: 2024-01-15 10:30:00
  Branch:  task/a1b2c3d4

Specifications: 2
  ✓ specification-1: User login flow [done]
  ○ specification-2: Session management [implementing]
  Summary: 1 completed, 1 implementing

Checkpoints: 3
  - #1: Initial planning (abc12345)
  - #2: Login implementation (def67890)
  - #3: Added tests (ghi11223)

Available commands:
  mehr implement - Implement the specifications
  mehr note      - Add notes to the task
  mehr finish    - Complete and optionally merge
```

### Worktree Task Status

When running from within a worktree:

```bash
cd ../project-worktrees/a1b2c3d4
mehr status
```

Output:

```
Worktree Task: a1b2c3d4
  Title:    Add user authentication
  State:    implementing - Generating code from specifications
  Source:   task.md
  Worktree: /path/to/project-worktrees/a1b2c3d4
  Started:  2024-01-15 10:30:00
  Branch:   task/a1b2c3d4

Specifications: 2
  ...
```

### All Tasks

```bash
mehr status --all
```

Output:

```
Tasks in workspace:

  * a1b2c3d4 (idle)
    Source: task.md
    Branch: task/a1b2c3d4
    Specs: 2

    b5c6d7e8 (implementing)
    Source: ./features/auth/
    Branch: task/b5c6d7e8
    Specs: 3

    c9d0e1f2 (done)
    Source: bugfix.md
    Branch: (deleted)
    Specs: 1

* = active task
```

### JSON Output

For programmatic access, use `--json`:

```bash
mehr status --json
```

Output:

```json
{
  "task_id": "a1b2c3d4",
  "title": "Add user authentication",
  "state": "implementing",
  "state_description": "Generating code from specifications",
  "source": "task.md",
  "work_dir": ".mehrhof/work/a1b2c3d4",
  "branch": "task/a1b2c3d4",
  "started_at": "2024-01-15T10:30:00Z",
  "agent_name": "claude",
  "is_active": true,
  "specifications": [
    {
      "number": 1,
      "title": "User login flow",
      "status": "done",
      "created_at": "2024-01-15T10:30:00Z",
      "completed_at": "2024-01-15T11:00:00Z"
    }
  ],
  "specifications_summary": {
    "draft": 0,
    "ready": 0,
    "implementing": 1,
    "done": 1
  },
  "checkpoints": [
    {
      "number": 1,
      "message": "Initial planning",
      "id": "abc1234567890abcdef",
      "timestamp": "2024-01-15T10:45:00Z"
    }
  ],
  "total_tokens": 125000
}
```

## Status Fields

| Field          | Description                   |
| -------------- | ----------------------------- |
| Task           | Unique 8-character identifier |
| State          | Current workflow state        |
| Source         | Original task file/directory  |
| Branch         | Git branch name               |
| Specifications | List of SPEC files            |
| Checkpoints    | Undo/redo availability        |
| Notes          | Number of note entries        |
| Sessions       | Number of logged sessions     |

## States

| State        | Meaning                 |
| ------------ | ----------------------- |
| idle         | Ready for action        |
| planning     | Creating specifications |
| implementing | Generating code         |
| reviewing    | Running review          |
| done         | Completed               |
| failed       | Error occurred          |

## No Active Task

If no task is active:

```bash
mehr status
```

Output:

```
No active task.

Start a new task:
  mehr start task.md

Or switch to an existing task branch:
  git checkout task/<id>
```

---

# mehr continue

Resume workflow with optional auto-execution.

## Synopsis

```bash
mehr continue [flags]
```

## Description

The `continue` command is designed for resuming work on a task after a break. It shows the current status and suggests what to do next. With `--auto`, it automatically executes the next logical workflow step.

**Flags:**
| Flag     | Type | Default | Description                             |
| -------- | ---- | ------- | --------------------------------------- |
| `--auto` | bool | false   | Auto-execute the next logical workflow step |

## Examples

### After Starting

```bash
mehr continue
```

Output:

```
Task: a1b2c3d4
State: idle
Source: task.md

No specifications yet.

Suggested next action:
  mehr plan    # Create implementation specifications
```

### After Planning

```bash
mehr continue
```

Output:

```
Task: a1b2c3d4
State: idle
Specs: 2 ready

Specifications are ready for implementation.

Suggested next action:
  mehr implement    # Generate code from specifications
```

### After Implementation

```bash
mehr continue
```

Output:

```
Task: a1b2c3d4
State: idle
Changes: 5 files modified

Code has been generated.

Suggested actions:
  git diff         # Review changes
  mehr review      # Run code review
  mehr finish      # Complete and merge
```

## Use Cases

### Resuming Work

After stepping away:

```bash
cd project
git checkout task/a1b2c3d4
mehr continue
```

### Quick Status Check

```bash
mehr continue
```

More contextual than `mehr status` with action suggestions.

### Command Comparison

| Command         | Purpose                                                  |
| --------------- | -------------------------------------------------------- |
| `mehr status`   | Detailed state inspection (specs, checkpoints, sessions)  |
| `mehr guide`    | Quick, lightweight suggestions (no execution capability)  |
| `mehr continue` | Status display with optional auto-execution capability    |

## See Also

- [start](cli/start.md) - Begin a new task
- [Workflow](../concepts/workflow.md) - Understanding states
