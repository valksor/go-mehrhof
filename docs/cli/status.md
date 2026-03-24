# kvelmo status

Show current task state and information.

## Usage

```bash
kvelmo status
```

## Options

| Flag | Short | Description |
|------|-------|-------------|
| `--all` | `-a` | Show status of all active projects |
| `--blocked` | | Show only tasks needing attention (failed, waiting for prompt) |
| `--failed` | | Show only failed tasks |
| `--full` | | Show extended status including checkpoints |
| `--verbose` | `-v` | Show socket paths |
| `--json` | | Output as JSON |
| `--timeout` | `-t` | Connection timeout (default 5s) |

## Examples

```bash
# Show current project status
kvelmo status

# Show all active projects
kvelmo status --all

# Show only tasks needing attention
kvelmo status --blocked

# Show only failed tasks
kvelmo status --failed

# JSON output
kvelmo status --json

# Extended output with checkpoints
kvelmo status --full
```

## Output

```
Task: Add user authentication
State: implemented
Branch: feature/add-user-auth
Provider: github:valksor/kvelmo#123

Checkpoints:
  - plan: abc1234
  - implement: def5678
```

## States

| State          | Description                |
|----------------|----------------------------|
| `none`         | No active task             |
| `loaded`       | Task loaded                |
| `planning`     | Planning in progress       |
| `planned`      | Ready to implement         |
| `implementing` | Implementation in progress |
| `implemented`  | Ready to review            |
| `reviewing`    | Review in progress         |
| `submitted`    | Task complete              |

Also in Web UI: [Dashboard](/web-ui/dashboard.md).

## Related

- [State Machine](/concepts/state-machine.md) — All states
- [list](/cli/list.md) — List all tasks
