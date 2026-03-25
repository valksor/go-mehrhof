# Task Groups

Task Groups coordinate tasks that span multiple repositories. Access the Task Groups panel from the global dashboard sidebar.

## Creating a Group

Click "Create Group" and provide a label describing the cross-repo feature or change.

## Adding Tasks

Add tasks from different projects to the group. Each task tracks its project directory, task ID, and current lifecycle state.

## Monitoring Status

The group status view shows all member tasks with their current states. This helps you track progress across repositories before submitting.

## Submitting

The "Submit" action marks the group as submitted. Note: this does not create PRs — run `kvelmo submit` in each project worktree first, then mark the group as submitted to record the coordinated completion.

## CLI Equivalent

All group operations are also available via CLI:

```bash
kvelmo group create "feature-name"
kvelmo group add <group-id> <task-id>
kvelmo group list
kvelmo group status <group-id>
kvelmo group submit <group-id>
kvelmo group remove <group-id>
```

See [group CLI reference](/cli/group.md) for details.

## Related

- [Dashboard](/web-ui/dashboard.md) — Global project view
- [Queue](/web-ui/queue.md) — Task queue management
