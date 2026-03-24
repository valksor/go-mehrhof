# kvelmo start

Start kvelmo sockets for the current directory.

## Usage

```bash
kvelmo start [description]
```

## Options

| Flag | Short | Description |
|------|-------|-------------|
| `--foreground` | | Run in foreground (for debugging) |
| `--verbose` | `-v` | Show socket paths |
| `--from` | | Task source (`file:path`, `github:owner/repo#123`, or URL) |
| `--text` | | Inline task description (creates task without external source) |
| `--auto` | | Auto-advance through plan, implement, and review |
| `--skip` | | Phases to skip during auto-advance (comma-separated, e.g. `--skip simplify,optimize`) |
| `--json` | | Output result as JSON |

A description can also be passed as a positional argument instead of using `--text`.

## Provider Formats

When using `--from`:

| Provider | Format                        | Example                     |
|----------|-------------------------------|-----------------------------|
| File     | `file:<path>`                 | `file:task.md`              |
| GitHub   | `github:<owner>/<repo>#<num>` | `github:valksor/kvelmo#123` |
| GitLab   | `gitlab:<project>#<num>`      | `gitlab:group/project#456`  |
| Wrike    | `wrike:<id>`                  | `wrike:abc123`              |

## Examples

```bash
# Start sockets
kvelmo start

# Show socket paths
kvelmo start --verbose

# Start and load a task from a file
kvelmo start --from file:task.md

# Start and load from GitHub issue
kvelmo start --from github:valksor/kvelmo#123

# Start with inline task description
kvelmo start --text "Fix login button alignment"

# Load from stdin
echo "Fix X" | kvelmo start --text -

# Auto-advance through plan, implement, review
kvelmo start --from file:task.md --auto

# Positional arg as inline text (shorthand for --text)
kvelmo start "Fix login button alignment"

# Auto-advance but skip simplify and optimize phases
kvelmo start --from file:task.md --auto --skip simplify,optimize
```

## What Happens

1. Global socket starts at `~/.valksor/kvelmo/global.sock` (if not already running)
2. Worktree socket starts for the current directory
3. If `--from` is provided, the task is loaded and state transitions to `loaded`

Also in Web UI: [Creating Tasks](/web-ui/creating-tasks.md).

## Related

- [plan](/cli/plan.md) — Next step after loading a task
- [Providers](/providers/index.md) — Task sources
