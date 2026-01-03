# mehr auto

Full automation mode that runs the entire workflow without user interaction.

## Synopsis

```bash
mehr auto <reference> [flags]
```

## Description

The `auto` command orchestrates a complete task lifecycle in one command:

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
| `--no-branch`      |       | Do not create a git branch           | `false`     |
| `--worktree`       | `-w`  | Create a separate git worktree       | `false`     |
| `--max-retries`    |       | Maximum quality check retry attempts | `3`         |
| `--no-quality`     |       | Skip quality checks entirely         | `false`     |
| `--no-push`        |       | Don't push after merge               | `false`     |
| `--no-delete`      |       | Don't delete task branch after merge | `false`     |
| `--no-squash`      |       | Use regular merge instead of squash  | `false`     |
| `--target`         | `-t`  | Target branch to merge into          | auto-detect |
| `--quality-target` |       | Make target for quality checks       | `quality`   |

## Examples

### Basic Usage

```bash
mehr auto task.md

mehr auto ./tasks/feature/
```

### Quality Control

```bash
mehr auto --max-retries 5 task.md

mehr auto --no-quality task.md

mehr auto --quality-target lint task.md
```

### Git Options

```bash
mehr auto --no-push task.md

mehr auto --no-delete task.md

mehr auto --target develop task.md

mehr auto --no-squash task.md

mehr auto --worktree task.md
```

## Quality Retry Loop

When quality checks fail, auto mode automatically:

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

In auto mode, if the AI agent asks a clarifying question during planning, the question is skipped and the agent proceeds with its best guess. This ensures fully non-interactive execution.

The skipped questions are logged for audit purposes.

## Output

During execution, auto mode displays progress:

```
Starting Auto mode for: task.md
Full automation: start -> plan -> implement -> quality -> finish

  [AUTO] Task registered
  [AUTO] Entering planning phase...
  [AUTO] Planning complete
  [AUTO] Entering implementation phase...
  [AUTO] Implementation complete
  [AUTO] Quality check attempt 1/3...
  [AUTO] Quality checks passed
  [AUTO] Finishing task...
  [AUTO] Task completed

AUTO complete!
  Quality attempts: 1
  Changes merged (not pushed)
```

On failure, it shows which phase failed:

```
AUTO failed at: quality
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
- When you want to review specifications before implementation

## Comparison with Manual Workflow

| Aspect           | Auto       | Manual       |
| ---------------- | ---------- | ------------ |
| User interaction | None       | At each step |
| Agent questions  | Skipped    | Answered     |
| Quality failures | Auto-retry | Manual fix   |
| Review specifications     | No         | Yes          |
| Control          | Less       | Full         |

## See Also

- [mehr start](start.md) - Start a task manually
- [mehr plan](plan.md) - Run planning phase
- [mehr implement](implement.md) - Run implementation phase
- [mehr finish](finish.md) - Complete and merge
