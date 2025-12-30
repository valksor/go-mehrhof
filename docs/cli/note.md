# mehr note

Add notes to the current task.

## Synopsis

```bash
mehr note [message]
mehr answer [message]  # Alias for answering agent questions
```

## Description

The `note` command saves notes to the task. Notes are appended to `notes.md` and included in subsequent planning and implementation prompts.

Unlike `plan` or `implement`, the `note` command does **not** run the AI agent. It simply saves your input as a note for future reference.

## Arguments

| Argument  | Description              |
| --------- | ------------------------ |
| `message` | Optional note to add     |

## Examples

### Interactive Mode

```bash
mehr note
```

Opens an interactive prompt for adding multiple notes:

```
Task: abc12345 (state: idle)

Entering interactive mode. Type 'exit' or 'quit' to leave.

> The API should use REST, not GraphQL
Note saved.

> Add error handling for edge cases
Note saved.

> exit
```

### Single Message

```bash
mehr note "Use PostgreSQL for the database, not SQLite"
```

Adds a single note without entering interactive mode.

### Answering Agent Questions

When the agent asks a question during planning, use the `answer` alias (which is `note` under a different name):

```bash
mehr plan
# Agent asks: "Should we use PostgreSQL or MySQL?"
mehr answer "Use PostgreSQL - we already have it in production"
mehr plan  # Continues with your answer
```

### Multiple Notes Before Planning

```bash
mehr start file:task.md
mehr note "Focus on security - this will handle payments"
mehr note "Use JWT for authentication"
mehr note "Add rate limiting to prevent abuse"
mehr plan
```

All notes are included when the agent runs during planning.

## What Happens

1. **Note Saving**
   - Input appended to `notes.md`
   - Timestamp and current state added

2. **No Agent Interaction**
   - The AI agent is NOT called
   - No tokens are consumed
   - Note is saved directly to disk

3. **Future Impact**
   - Notes included in `plan` prompts
   - Notes included in `implement` prompts
   - Notes included in `review` prompts

## Notes File

Notes are saved to `.mehrhof/work/<id>/notes.md`:

```markdown
# Notes

## 2025-01-15 10:45:00 [idle]

Use PostgreSQL for the database, not SQLite.

## 2025-01-15 11:00:00 [idle]

Focus on security - this will handle payments.

## 2025-01-15 11:30:00 [waiting]

**Q:** Should we use PostgreSQL or MySQL?
**A:** Use PostgreSQL - we already have it in production.
```

## Interactive Commands

In interactive mode:

| Command | Action      |
| ------- | ----------- |
| `exit`  | End session |
| `quit`  | End session |
| Ctrl+D  | End session |

## Use Cases

### Add Requirements

```bash
mehr note "The search should support fuzzy matching with typo tolerance"
```

### Provide Context

```bash
mehr note "We're using the existing UserRepository for data access"
```

### Request Changes

```bash
mehr note "Remove the caching - we'll add it later"
```

### Answer Questions

```bash
mehr answer "Use dependency injection instead of global state"
```

## Workflow

Typical workflow for adding context before planning:

```bash
# Start a task
mehr start file:task.md

# Add multiple notes
mehr note "Focus on performance"
mehr note "Add comprehensive error handling"
mehr note "Include unit tests for all new code"

# Run planning (agent sees all notes)
mehr plan
```

## See Also

- [plan](plan.md) - Create specifications (agent runs here)
- [implement](implement.md) - Generate code (agent runs here)
- [Workflow](../concepts/workflow.md) - Understanding states
