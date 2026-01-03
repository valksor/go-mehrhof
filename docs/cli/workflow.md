# mehr workflow

Display the workflow state machine diagram.

## Synopsis

```bash
mehr workflow [flags]
```

## Description

The `workflow` command displays a visual representation of the Mehrhof state machine. This helps you understand how tasks flow from start to completion, including all valid states and transitions.

The diagram shows:
- All workflow states (idle, planning, implementing, reviewing, etc.)
- Valid transitions between states
- Commands that trigger each transition
- Available actions from each state

**Use when:** You want to understand the workflow or see what commands are available from your current state.

## Flags

This command has no flags.

## Examples

### Display the Workflow

```bash
mehr workflow
```

Shows the complete state machine diagram with:

```
╔══════════════════════════════════════════════════════════════════════════════╗
║                           MEHRHOF WORKFLOW STATE MACHINE                         ║
╚══════════════════════════════════════════════════════════════════════════════╝

[ASCII art diagram showing states and transitions]

COMMANDS BY STATE:

  From idle (central hub):
    mehr start <ref>     Begin new task
    mehr plan             Create specifications
    mehr implement        Generate code from specifications
    mehr review           Run code review (optional)
    mehr finish           Complete task (creates PR or merges)
    mehr undo             Revert to previous checkpoint
    mehr redo             Restore forward checkpoint
    mehr status           Show current state
    mehr guide            Get context-aware suggestions
...
```

### Check Current State Instead

For your current task state, use `status`:

```bash
mehr status
```

### Get Context-Aware Suggestions

For suggested next actions, use `guide`:

```bash
mehr guide
```

## States Overview

| State         | Description                       | Next Steps                             |
| ------------- | --------------------------------- | -------------------------------------- |
| **idle**      | Central hub, ready for action     | plan, implement, review, finish, etc. |
| **planning**  | AI creating specifications        | Wait for completion or answer          |
| **implementing** | AI generating code            | Wait for completion or answer          |
| **reviewing** | Code review in progress           | Wait for completion                    |
| **waiting**   | Agent needs your input            | Answer with `mehr note` or `mehr answer` |
| **done**      | Task completed                    | Start a new task                       |
| **failed**    | Error occurred                    | Restart task                           |

## Choosing the Right Command

| Command      | Purpose                                          |
| ------------ | ------------------------------------------------ |
| `workflow`   | Visual diagram of all states and transitions     |
| `status`     | Current task state and details                   |
| `guide`      | Context-aware suggestions for next actions       |
| `continue`   | Resume workflow with auto-execution option       |

## Related Concepts

For a deeper understanding of the workflow state machine, guards, and lifecycle, see [Workflow Concepts](../concepts/workflow.md).

## Key Transitions

| From         | To           | Command         | Condition                              |
| ------------ | ------------ | --------------- | -------------------------------------- |
| idle         | planning     | `mehr plan`     | Task has valid source                  |
| planning     | idle         | (completion)    | Specifications created                 |
| idle         | implementing | `mehr implement` | Specifications exist                   |
| implementing | idle         | (completion)    | Code generated                         |
| idle         | reviewing    | `mehr review`   | Code changes exist                     |
| idle         | done         | `mehr finish`   | Quality checks pass                    |

## See Also

- [Workflow Concepts](../concepts/workflow.md) - Detailed state machine explanation
- [status](status.md) - Show current task state
- [guide](guide.md) - Get context-aware next actions
- [continue](continue.md) - Resume workflow with suggestions
