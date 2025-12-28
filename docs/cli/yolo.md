# mehr yolo

Full automation mode that runs the entire workflow without user interaction.

## Synopsis

```bash
mehr yolo <reference> [flags]
```

## Description

The `yolo` command orchestrates a complete task lifecycle in one command:

1. **Start** - Register task and create git branch
2. **Plan** - Generate implementation specifications (agent questions are skipped)
3. **Implement** - Execute the specifications
4. **Quality** - Run quality checks with automatic retry loop
5. **Finish** - Merge changes to target branch

This is ideal for well-defined tasks where you trust the AI to handle the entire process autonomously.

## Arguments

| Argument    | Description                                              |
| ----------- | -------------------------------------------------------- |
| `reference` | Task source: file path, directory, or provider reference |

## Flags

| Flag               | Short | Description                          | Default     |
| ------------------ | ----- | ------------------------------------ | ----------- |
| `--agent`          | `-a`  | Agent to use                         | auto-detect |
| `--branch`         | `-b`  | Create git branch for the task       | `true`      |
| `--no-branch`      |       | Do not create a git branch           |             |
| `--worktree`       | `-w`  | Create a separate git worktree       | `false`     |
| `--max-retries`    |       | Maximum quality check retry attempts | `3`         |
| `--skip-quality`   |       | Skip quality checks entirely         | `false`     |
| `--no-push`        |       | Don't push after merge               | `false`     |
| `--no-delete`      |       | Don't delete task branch after merge | `false`     |
| `--no-squash`      |       | Use regular merge instead of squash  | `false`     |
| `--target`         | `-t`  | Target branch to merge into          | auto-detect |
| `--quality-target` |       | Make target for quality checks       | `quality`   |

## Examples

### Basic Usage

```bash
# Full automation from a markdown file
mehr yolo task.md

# Full automation from a directory
mehr yolo ./tasks/feature/
```

### Quality Control

```bash
# Allow more quality retries (default: 3)
mehr yolo --max-retries 5 task.md

# Skip quality checks entirely
mehr yolo --skip-quality task.md

# Use custom make target for quality
mehr yolo --quality-target lint task.md
```

### Git Options

```bash
# Don't push after merge
mehr yolo --no-push task.md

# Keep the task branch after merge
mehr yolo --no-delete task.md

# Merge to specific branch
mehr yolo --target develop task.md

# Use regular merge instead of squash
mehr yolo --no-squash task.md

# Use git worktree
mehr yolo --worktree task.md
```

## Quality Retry Loop

When quality checks fail, yolo mode automatically:

1. Captures the quality check output (error messages)
2. Appends it to the task notes as feedback
3. Re-runs implementation phase (agent sees what failed)
4. Re-runs quality checks
5. Repeats until quality passes or `--max-retries` is exceeded

This creates a self-correcting feedback loop where the AI can fix its own mistakes.

```
┌─────────────────┐
│ Run make quality│
└────────┬────────┘
         │
    ┌────┴────┐
    │ Passed? │
    └────┬────┘
    Yes  │  No
    ─────┼─────
         │         ┌─────────────────┐
         ▼         │ attempt < max?  │
    Continue       └────────┬────────┘
    to Finish           Yes │  No
                       ─────┼─────
                            │
                    ┌───────┴────────┐
                    │ Add errors to  │
                    │ notes, re-impl │
                    └───────┬────────┘
                            │
                    Loop back to quality
```

## Agent Question Handling

In yolo mode, if the AI agent asks a clarifying question during planning, the question is skipped and the agent proceeds with its best guess. This ensures fully non-interactive execution.

The skipped questions are logged for audit purposes.

## Output

During execution, yolo mode displays progress:

```
Starting YOLO mode for: task.md
Full automation: start -> plan -> implement -> quality -> finish

  [YOLO] Task registered
  [YOLO] Entering planning phase...
  [YOLO] Planning complete
  [YOLO] Entering implementation phase...
  [YOLO] Implementation complete
  [YOLO] Quality check attempt 1/3...
  [YOLO] Quality checks passed
  [YOLO] Finishing task...
  [YOLO] Task completed

YOLO complete!
  Quality attempts: 1
  Changes merged (not pushed)
```

On failure, it shows which phase failed:

```
YOLO failed at: quality
  Planning:       done
  Implementation: done
  Quality:        3 attempt(s), passed=false
  Finish:         pending
```

## When to Use

**Good for:**

- Well-defined, self-contained tasks
- Batch processing multiple tasks
- CI/CD automation
- Tasks with clear acceptance criteria

**Not recommended for:**

- Complex tasks requiring clarification
- Tasks with ambiguous requirements
- When you want to review specs before implementation

## Comparison with Manual Workflow

| Aspect           | Yolo       | Manual       |
| ---------------- | ---------- | ------------ |
| User interaction | None       | At each step |
| Agent questions  | Skipped    | Answered     |
| Quality failures | Auto-retry | Manual fix   |
| Review specs     | No         | Yes          |
| Control          | Less       | Full         |

## See Also

- [mehr start](start.md) - Start a task manually
- [mehr plan](plan.md) - Run planning phase
- [mehr implement](implement.md) - Run implementation phase
- [mehr finish](finish.md) - Complete and merge
