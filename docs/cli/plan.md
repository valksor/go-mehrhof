# mehr plan

Create implementation specifications for the active task.

## Synopsis

```bash
mehr plan [flags]
```

## Description

The `plan` command runs the planning phase where the AI agent:

1. Reads the task source content
2. Reviews any existing notes
3. Analyzes requirements
4. Generates specification files with implementation details

specification files are saved to `.mehrhof/work/<id>/specifications/`.

## Flags

| Flag               | Short | Type   | Default | Description                         |
| ------------------ | ----- | ------ | ------- | ----------------------------------- |
| `--new`            | `-n`  | bool   | false   | Start standalone planning (no task) |
| `--verbose`        | `-v`  | bool   | false   | Show agent output in real-time      |
| `--agent-planning` |       | string |         | Override agent for planning step    |
| `--full-context`   |       | bool   | false   | Include full exploration context    |

## Examples

### Plan Active Task

```bash
mehr plan
```

Output:

```
Planning task a1b2c3d4...
Created: specification-1.md
Created: specification-2.md
Planning complete. 2 specifications created.
```

### Verbose Output

```bash
mehr plan --verbose
```

Shows the agent's reasoning and progress in real-time.

### Standalone Planning

```bash
mehr plan --new
```

Start an interactive planning session without a formal task:

```
Enter topic: How should I implement user authentication?
...
Plan saved to: .mehrhof/planned/xyz789/
```

### Standalone with Seed

```bash
mehr plan --new "Design a REST API for products"
```

Skip the topic prompt by providing it directly.

### Override Planning Agent

```bash
mehr plan --agent-planning claude-opus
```

Use a specific agent for this planning session. See [Per-Step Agents](../concepts/agents.md#per-step-agent-configuration).

## What Happens

### For Active Tasks

1. **Context Gathering**
   - Reads source from `.mehrhof/work/<id>/`
   - Includes notes from `notes.md`
   - Reviews any existing specs

2. **Agent Execution**
   - Prompts agent with context
   - Agent analyzes requirements
   - Agent generates structured specs

3. **Output**
   - specification files written to `specifications/` directory
   - Session logged to `sessions/`
   - Checkpoint created for undo support

### For Standalone Planning

1. **Input**
   - Interactive or seed-provided topic
   - No task context required

2. **Output**
   - Plan saved to `.mehrhof/planned/<id>/`
   - Includes `PLAN_HISTORY.md` with conversation

## SPEC File Format

Generated specs follow this format:

```markdown
---
title: Implementation Title
status: draft
created_at: 2025-01-15T10:30:00Z
---

# Component Name

## Overview

Brief description of what this spec covers.

## Implementation Details

1. Step one
2. Step two

## Files to Modify

- `path/to/file.go` - Description
- `path/to/other.go` - Description
```

## Multiple Specs

The agent may create multiple specification files for complex tasks:

```
specifications/
├── specification-1.md    # Core functionality
├── specification-2.md    # API endpoints
└── specification-3.md    # Tests
```

## Iterating on Plans

You can run `mehr plan` multiple times:

- Additional specs are appended
- Use `mehr note` to add requirements first
- Delete unwanted specs manually

## After Planning

Review the specifications:

```bash
cat .mehrhof/work/*/specifications/specification-1.md
```

If satisfied, implement:

```bash
mehr implement
```

If changes needed, add notes:

```bash
mehr note "Simplify the authentication flow"
```

## See Also

- [implement](cli/implement.md) - Implement specifications
- [note](cli/note.md) - Add notes or clarifications
- [SPEC Format](../reference/spec-format.md) - Specification details
