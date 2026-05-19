# Planning Phase

The planning phase generates a structured specification before any code changes.

## Starting Planning

When your task is in the `loaded` state:

1. Review the task title and description
2. Click **Plan** in the actions panel
3. Watch the agent work in the Output panel

## What Happens During Planning

The AI agent:

1. **Analyzes** the task requirements
2. **Explores** the codebase for context
3. **Identifies** files to modify
4. **Generates** a specification document

## Watching Progress

The Output panel shows real-time progress:

- Agent thoughts and reasoning
- File reads and searches
- Analysis steps
- Specification generation

## Reviewing the Specification

After planning completes:

1. Click **Specifications** in the sidebar
2. Open the generated specification
3. Review the implementation plan

### What to Look For

- Does the plan match your intent?
- Are the right files identified?
- Is the approach reasonable?
- Are there any misunderstandings?

## If the Plan is Wrong

If the specification doesn't match your intent:

1. Click **Undo** to revert
2. Modify the task description with more context
3. Click **Plan** again

**Tip:** Be specific in your description. The more context you provide, the better the specification.

## Adding Context

Before planning, you can add context:

- Edit the task description
- Add reference files or documentation
- Specify constraints or preferences

## Specification Location

Specifications are stored in:

```
.kvelmo/specifications/specification.md
```

Multiple planning attempts add numbered versions.

## State Transition

| Before   | After     |
| -------- | --------- |
| `loaded` | `planned` |

A checkpoint is created after successful planning.

Prefer the command line? See [kvelmo plan](/cli/plan.md).
