# mehr chat

Add notes or discuss the task with the AI agent.

## Synopsis

```bash
mehr chat [message] [flags]
```

## Description

The `chat` command enters dialogue mode for:

- Adding context and clarifications
- Asking questions about requirements
- Refining understanding before implementation
- Discussing approach and trade-offs

Notes from chat sessions are saved and included in future planning and implementation prompts.

## Arguments

| Argument  | Description              |
| --------- | ------------------------ |
| `message` | Optional initial message |

## Flags

| Flag            | Short | Type   | Default | Description                       |
| --------------- | ----- | ------ | ------- | --------------------------------- |
| `--continue`    | `-c`  | bool   | false   | Continue previous session         |
| `--session`     | `-s`  | string |         | Specific session file to continue |
| `--agent-chat`  |       | string |         | Override agent for dialogue       |

## Examples

### Interactive Mode

```bash
mehr chat
```

Opens an interactive conversation:

```
You: What's the best approach for the authentication?
Agent: Based on the requirements, I recommend...
You: Let's use JWT instead of sessions
Agent: Understood. I'll update...
You: exit
Session saved.
```

### Single Message

```bash
mehr chat "Use PostgreSQL for the database, not SQLite"
```

Adds a note without entering interactive mode.

### Continue Previous Session

```bash
mehr chat --continue
```

Resume the most recent chat session with full context.

### Continue Specific Session

```bash
mehr chat --session 2025-01-15T10-30-00-chat.yaml
```

Resume a specific session by filename.

### Override Dialogue Agent

```bash
mehr chat --agent-chat claude-opus
```

Use a specific agent for this conversation. See [Per-Step Agents](../concepts/agents.md#per-step-agent-configuration).

## What Happens

1. **State Change**
   - Enters `dialogue` state
   - Returns to `idle` when done

2. **Conversation**
   - Messages sent to AI agent
   - Responses displayed
   - Context maintained throughout

3. **Note Saving**
   - Notes appended to `notes.md`
   - Session logged to `sessions/`

4. **Future Impact**
   - Notes included in `plan` prompts
   - Notes included in `implement` prompts

## When to Use Chat

### Before Planning

```bash
mehr start task.md
mehr chat "Focus on security - this will handle payments"
mehr plan
```

### After Planning

```bash
mehr plan
mehr chat "specification-2 is too complex. Can we simplify?"
mehr plan  # Creates refined specs
```

### During Implementation

```bash
mehr implement
# Not quite right...
mehr undo
mehr chat "Use dependency injection instead of global state"
mehr implement
```

## Notes File

Notes are saved to `.mehrhof/work/<id>/notes.md`:

```markdown
# Notes

## 2025-01-15 10:30:00 [planning]

Use PostgreSQL for the database.

## 2025-01-15 11:00:00 [implementing]

Focus on error handling for edge cases.
```

## Session Files

Sessions are logged to `.mehrhof/work/<id>/sessions/`:

```yaml
version: "1"
kind: Session
metadata:
  started_at: 2025-01-15T10:30:00Z
  type: chat
  agent: claude
exchanges:
  - role: user
    content: "Use PostgreSQL"
  - role: agent
    content: "Understood..."
```

## Interactive Commands

In interactive mode:

| Command | Action      |
| ------- | ----------- |
| `exit`  | End session |
| `quit`  | End session |
| Ctrl+D  | End session |

## Use Cases

### Clarify Requirements

```bash
mehr chat "The search should support fuzzy matching with typo tolerance"
```

### Ask Questions

```bash
mehr chat "What's the best way to handle rate limiting?"
```

### Provide Context

```bash
mehr chat "We're using the existing UserRepository for data access"
```

### Request Changes

```bash
mehr chat "Remove the caching - we'll add it later"
```

## See Also

- [plan](plan.md) - Create specifications
- [implement](implement.md) - Generate code
- [Workflow](../concepts/workflow.md) - Understanding states
