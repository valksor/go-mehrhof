# File Changes

View files modified by the agent during the current task.

## Opening

The File Changes panel appears as a tab in the project workspace. It is automatically populated when a task is active and the agent has made code changes.

## Viewing Changes

The panel lists all modified files with their status:

| Status   | Description                   |
| -------- | ----------------------------- |
| Added    | New file created by the agent |
| Modified | Existing file changed         |
| Deleted  | File removed                  |
| Renamed  | File moved or renamed         |

Click any file to open a diff view showing the exact changes made.

## Diff Against Base

Click **Diff Against Base** to compare the current worktree state against the base branch. This shows the cumulative effect of all agent changes.

## Related

- [kvelmo diff](/cli/diff.md) — CLI equivalent
- `/diff` — Chat command equivalent
- [Reviewing](/web-ui/reviewing.md) — Review workflow for approving changes
