# kvelmo group

Cross-repo task group management. Link tasks across multiple repositories for synchronized lifecycle operations.

## Usage

```bash
kvelmo group <subcommand>
```

## Subcommands

| Subcommand | Description |
|------------|-------------|
| `create <label>` | Create a new task group |
| `add <group-id> <task-id>` | Add a task to a group |
| `list` | List all task groups |
| `status <group-id>` | Show group status and member tasks |
| `submit <group-id>` | Mark group as submitted |
| `remove <group-id>` | Remove a task group |

## Examples

```bash
# Create a group for a cross-repo feature
kvelmo group create "auth-migration"

# Add tasks — run from inside each project's worktree directory
# (the current directory is recorded as the task's project path)
cd ~/projects/api && kvelmo group add grp_abc123 task_001
cd ~/projects/web && kvelmo group add grp_abc123 task_002 --state implementing

# Check group status
kvelmo group status grp_abc123

# Mark group as submitted (after running `kvelmo submit` in each project)
kvelmo group submit grp_abc123

# Clean up
kvelmo group remove grp_abc123
```

## Flags

| Flag | Subcommand | Description |
|------|------------|-------------|
| `--state <state>` | `add` | Initial state to record for the task (default: `loaded`) |

### Valid States

Common states for the `--state` flag:

| State | Meaning |
|-------|---------|
| `loaded` | Task loaded from provider (default) |
| `planned` | Specification complete |
| `implementing` | Agent writing code |
| `implemented` | Implementation complete |
| `reviewing` | Under human review |
| `submitted` | PR created |

See [State Machine](/concepts/state-machine.md) for the full list.

## How It Works

Task groups coordinate tasks that span multiple repositories. The group tracks each member task's project directory and current state.

Submission workflow:
1. Run `kvelmo submit` in each project worktree to create individual PRs
2. Run `kvelmo group submit <group-id>` to mark the entire group as submitted

Note: `group submit` only records the group status — it does not create PRs.

## Web UI

Task groups can also be managed from the web UI via the Task Groups panel accessible from the global dashboard.

## Related

- [batch](/cli/batch.md) — Run actions across all active projects
- [submit](/cli/submit.md) — Create pull request for a task
