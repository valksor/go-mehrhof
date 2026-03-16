# CLI Reference

The kvelmo CLI provides a text-based interface for power users and automation.

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/valksor/kvelmo/master/install.sh | bash
```

## Global Flags

All commands support these global flags:

| Flag           | Description              |
|----------------|--------------------------|
| `--help`, `-h` | Show help for a command  |
| `--version`    | Show version information |

## Command Categories

### Workflow Commands

Commands that drive the task lifecycle:

| Command                        | Description                            |
|--------------------------------|----------------------------------------|
| [start](/cli/start.md)         | Load task from provider, create branch |
| [plan](/cli/plan.md)           | Generate implementation specification  |
| [implement](/cli/implement.md) | Execute the specification              |
| [simplify](/cli/simplify.md)   | Optional: simplify code for clarity    |
| [optimize](/cli/optimize.md)   | Optional: optimize code quality        |
| [review](/cli/review.md)       | Human review + security scan           |
| [submit](/cli/submit.md)       | Create PR and submit to provider       |
| [refresh](/cli/refresh.md)     | Check PR status and update state       |
| [finish](/cli/finish.md)       | Clean up after PR merge                |

### Navigation Commands

Commands for moving through your work history:

| Command                    | Description                             |
|----------------------------|-----------------------------------------|
| [undo](/cli/undo.md)       | Revert to previous checkpoint           |
| [redo](/cli/redo.md)       | Restore next checkpoint                 |
| [reset](/cli/reset.md)     | Recover from failed state               |
| [abort](/cli/abort.md)     | Abort current task                      |
| [abandon](/cli/abandon.md) | Full cleanup (stop + branch + work dir) |

### Information Commands

Commands for viewing status and information:

| Command                            | Description                    |
|------------------------------------|--------------------------------|
| [status](/cli/status.md)           | Show current task state        |
| [watch](/cli/watch.md)             | Stream live task output        |
| [tui](/cli/tui.md)                 | Interactive terminal UI        |
| [logs](/cli/logs.md)               | Show agent activity log        |
| [diff](/cli/diff.md)               | Show what the agent changed    |
| [show](/cli/show.md)               | Display task artifacts (specs) |
| [stats](/cli/stats.md)             | Show task analytics            |
| [list](/cli/list.md)               | List all tasks in workspace    |
| [checkpoints](/cli/checkpoints.md) | List git checkpoints           |
| [jobs](/cli/jobs.md)               | List worker jobs               |

### Management Commands

Commands for managing the kvelmo system:

| Command                      | Description                      |
|------------------------------|----------------------------------|
| [serve](/cli/serve.md)       | Start global socket + web server |
| [config](/cli/config.md)     | Configuration management         |
| [projects](/cli/projects.md) | Project registry management      |
| [workers](/cli/workers.md)   | Worker pool status               |
| [delete](/cli/delete.md)     | Delete terminal task             |
| [update](/cli/update.md)     | Re-fetch task from provider      |
| [stop](/cli/stop.md)         | Stop running job                 |
| [diagnose](/cli/diagnose.md) | Check system requirements        |
| [cleanup](/cli/cleanup.md)   | Remove stale socket files        |
| [shutdown](/cli/shutdown.md) | Shutdown worktree socket server  |

### Authentication Commands

Commands for authenticating with providers:

| Command                       | Description              |
|-------------------------------|--------------------------|
| [github login](/cli/login.md) | Authenticate with GitHub |
| [gitlab login](/cli/login.md) | Authenticate with GitLab |
| [linear login](/cli/login.md) | Authenticate with Linear |
| [wrike login](/cli/login.md)  | Authenticate with Wrike  |

### Utility Commands

Additional utility commands:

| Command                            | Description                      |
|------------------------------------|----------------------------------|
| [chat](/cli/chat.md)               | Interactive chat with agent      |
| [explain](/cli/explain.md)         | Ask agent to explain last action |
| [pipe](/cli/pipe.md)               | Run one-shot prompt (no server)  |
| [memory](/cli/memory.md)           | Memory management                |
| [files](/cli/files.md)             | File browser                     |
| [screenshots](/cli/screenshots.md) | Screenshot management            |
| [recordings](/cli/recordings.md)   | Agent interaction recordings     |
| [browse](/cli/browse.md)           | Open URLs in browser             |
| [browser](/cli/browser.md)         | Browser automation               |
| [git](/cli/git.md)                 | Git operations                   |
| [remote](/cli/remote.md)           | Remote provider ops (approve, merge) |
| [quality](/cli/quality.md)         | Quality gate controls            |
| [completion](/cli/completion.md)   | Shell completion setup           |

## Quick Example

```bash
# Start a task from a file
kvelmo start --from file:task.md

# Run the workflow
kvelmo plan
kvelmo implement
kvelmo review
kvelmo submit

# Check status at any time
kvelmo status

# Undo if something goes wrong
kvelmo undo
```

## Web UI

Prefer a visual interface? See [Web UI Guide](/web-ui/getting-started.md).
