# Implementation Phase

The implementation phase executes the specification and makes code changes.

## Starting Implementation

When your task is in the `planned` state:

1. Review the specification one more time
2. Click **Implement** in the actions panel
3. Watch the agent work in the Output panel

## What Happens During Implementation

The AI agent:

1. **Reads** the specification
2. **Modifies** code files as planned
3. **Creates** new files if needed
4. **Runs** tests if specified
5. **Creates** a checkpoint when done

## Watching Progress

The Output panel shows:

- Files being read and modified
- Code changes being made
- Tool calls (file writes, terminal commands)
- Errors and warnings

## Monitoring Changes

Use the **Changes** panel in the sidebar to see:

- Files modified
- Diff view of changes
- Lines added/removed

## If Something Goes Wrong

If the implementation isn't right:

1. Click **Undo** to revert to the planned state
2. You can:
   - Modify the specification
   - Add more context to the task
   - Re-plan with new information
3. Click **Implement** again

**Tip:** Use undo liberally. It's safe and doesn't lose work.

## Optional: Simplify

After implementation, you can optionally run:

1. Click **Simplify** to clean up code
2. The agent reviews the changes for clarity
3. Refactors for readability

## Optional: Optimize

Or run optimization:

1. Click **Optimize** to improve code quality
2. The agent reviews for performance
3. Suggests and applies optimizations

## State Transition

| Before    | After         |
| --------- | ------------- |
| `planned` | `implemented` |

A checkpoint is created after successful implementation.

## Long-Running Implementations

For large changes:

- The Output panel streams progress
- You can monitor in real-time
- The **Workers** panel shows job status

Prefer the command line? See [kvelmo implement](/cli/implement.md).
