# CLI Reference

The CLI is the most direct interface to kvelmo's local orchestration system.

Use it when you want explicit control over task state, scripting and automation hooks, provider operations, shell-native tooling, or system-facing commands that do not belong in the browser.

The Web UI is the primary experience for most users, but the CLI is the clearest way to understand the product's full control surface.

## What the CLI Is Best At

- scripting and automation
- shell integration
- daemon lifecycle control
- provider operations
- raw inspection and recovery flows
- commands that intentionally stay outside the browser

## Core Workflow

The common path remains:

```bash
kvelmo start --from file:task.md
kvelmo plan
kvelmo implement
kvelmo review
kvelmo submit
kvelmo finish
```

Optional follow-up phases also exist:

```bash
kvelmo simplify
kvelmo optimize
```

Quick path:

```bash
kvelmo quick "Fix the broken validation message"
```

## Shared State With Other Interfaces

The CLI does not run a different workflow. It operates on the same local task state used by the Web UI, desktop app, and TUI.

That means you can:

- start a task in the browser
- inspect it in CLI
- continue it in TUI
- return to the browser for review

## Command Areas

### Workflow

The workflow commands move a task through the lifecycle:

| Command | Description |
|---------|-------------|
| [start](/cli/start.md) | Load or create a task |
| [plan](/cli/plan.md) | Generate the implementation specification |
| [implement](/cli/implement.md) | Execute the approved plan |
| [simplify](/cli/simplify.md) | Run a clarity-focused pass |
| [optimize](/cli/optimize.md) | Run a quality-focused pass |
| [review](/cli/review.md) | Enter the review checkpoint |
| [submit](/cli/submit.md) | Create the pull request |
| [finish](/cli/finish.md) | Clean up after merge or completion |
| [quick](/cli/quick.md) | Fast path that skips planning |

### Navigation and Recovery

Use these when you need visibility or control over task progression:

| Command | Description |
|---------|-------------|
| [status](/cli/status.md) | Inspect current task state |
| [watch](/cli/watch.md) | Stream progress |
| [undo](/cli/undo.md) | Go back to a checkpoint |
| [redo](/cli/redo.md) | Restore an undone checkpoint |
| [reset](/cli/reset.md) | Recover task state |
| [retry](/cli/retry.md) | Re-run a failed phase |
| [abort](/cli/abort.md) | Force a task into failed state |
| [stop](/cli/stop.md) | Interrupt running work |
| [abandon](/cli/abandon.md) | Discard the current task |

### Governance and Review

These commands sit around execution rather than replacing it:

| Command | Description |
|---------|-------------|
| [approve](/cli/approve.md) | Record approval decisions |
| [checklist](/cli/checklist.md) | Manage review checklist items |
| [audit](/cli/audit.md) | Inspect audit trail data |
| [policy](/cli/policy.md) | Evaluate workflow policy |
| [quality](/cli/quality.md) | Run quality gates |
| [ci](/cli/ci.md) | Inspect CI status |
| [security](/cli/security.md) | Run security scanning |

### Information and Context

These commands help you understand task and project state:

| Command | Description |
|---------|-------------|
| [show](/cli/show.md) | Display saved workflow artifacts |
| [diff](/cli/diff.md) | Inspect changed files |
| [checkpoints](/cli/checkpoints.md) | Inspect checkpoint history |
| [jobs](/cli/jobs.md) | Inspect running and queued jobs |
| [logs](/cli/logs.md) | Review logs |
| [activity](/cli/activity.md) | Inspect RPC activity |
| [eventlog](/cli/eventlog.md) | View lifecycle events |
| [recap](/cli/recap.md) | Summarize state for resume |
| [explain](/cli/explain.md) | Ask for explanation of recent behavior |
| [stats](/cli/stats.md) | View metrics and counters |

### Project Operations

These commands cover the broader operational surface:

| Command | Description |
|---------|-------------|
| [queue](/cli/queue.md) | Manage task queues |
| [batch](/cli/batch.md) | Run actions across projects |
| [catalog](/cli/catalog.md) | Work with task templates |
| [fork](/cli/fork.md) | Branch task alternatives |
| [group](/cli/group.md) | Coordinate task groups |
| [projects](/cli/projects.md) | Manage registered projects |
| [workers](/cli/workers.md) | Manage worker pools |
| [backup](/cli/backup.md) | Export state for recovery |
| [restore](/cli/restore.md) | Restore saved state |
| [export](/cli/export.md) | Export task data |
| [report](/cli/report.md) | Generate reports |
| [notify](/cli/notify.md) | Test notification delivery |
| [hooks](/cli/hooks.md) | Inspect workflow hooks |
| [access](/cli/access.md) | Manage access tokens |

### Interface and Utility Commands

| Command | Description |
|---------|-------------|
| [serve](/cli/serve.md) | Start sockets and web server |
| [shutdown](/cli/shutdown.md) | Stop the server |
| [tui](/cli/tui.md) | Open the terminal UI |
| [chat](/cli/chat.md) | Chat with the agent |
| [pipe](/cli/pipe.md) | Run a one-shot prompt without server mode |
| [rpc](/cli/rpc.md) | Call raw JSON-RPC methods |
| [browse](/cli/browse.md) | Open URLs in a browser |
| [browser](/cli/browser.md) | Control browser automation |
| [memory](/cli/memory.md) | Manage semantic memory |
| [screenshots](/cli/screenshots.md) | Manage screenshots |
| [codegraph](/cli/codegraph.md) | Inspect code graph data |
| [discover](/cli/discover.md) | Discover project commands |
| [prompt](/cli/prompt.md) | Shell prompt integration |
| [completion](/cli/completion.md) | Shell completion |

## Providers and Login

Provider-aware task sources and login flows are documented here:

- [Providers Overview](/providers/index.md)
- [Provider login subcommands](/cli/login.md)
- [test-provider](/cli/test-provider.md)

## Prefer the Browser?

If you want the same workflow with richer dashboards and visual review surfaces, start with [Web UI Getting Started](/web-ui/getting-started.md).
