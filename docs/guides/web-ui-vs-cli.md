# Web UI vs CLI vs TUI

kvelmo has multiple interfaces because different parts of the workflow benefit from different control surfaces.

The important point is this: they are not separate products. They are separate views into the same local orchestration system.

## Short Version

- start with the **Web UI** if you want the clearest day-to-day experience
- use the **CLI** when you want explicit control, automation, or shell-native operations
- use the **TUI** when you want a full-screen terminal workflow without opening a browser

## Quick Comparison

| Aspect              | Web UI                            | CLI                                          | TUI                            |
| ------------------- | --------------------------------- | -------------------------------------------- | ------------------------------ |
| Best for            | primary day-to-day orchestration  | scripting, automation, system-facing control | terminal-first live steering   |
| Interaction style   | dashboards, panels, visual review | commands, pipes, shell integration           | full-screen terminal dashboard |
| Browser required    | yes                               | no                                           | no                             |
| Scriptable          | limited                           | yes                                          | no                             |
| Visual review       | strongest                         | lower                                        | limited                        |
| Raw control surface | medium                            | strongest                                    | medium                         |
| Remote/headless use | possible with browser access      | strongest                                    | strong                         |

## When to Use the Web UI

Choose the Web UI when you want:

- the main product experience
- project dashboards and task views
- visual review of changes and context
- easy access to related surfaces such as logs, activity, memory, recordings, code graph, policy, CI, and browser tools

The Web UI is the best default because it exposes the orchestration system most clearly.

## When to Use the CLI

Choose the CLI when you want:

- automation and scripting
- provider-heavy workflows
- shell integration and raw command control
- system-facing operations like daemon lifecycle, RPC, and one-shot pipelines

The CLI is also the best reference surface for understanding the full command model.

## When to Use the TUI

Choose the TUI when you want:

- a full-screen terminal interface
- live output and workflow control without a browser
- a terminal-first environment where switching contexts is expensive

Start it with:

```bash
kvelmo tui
```

## Mixing Interfaces

Mixing interfaces is normal.

Example:

```bash
# Start the local server
kvelmo serve --open

# Check state from the terminal
kvelmo status

# Watch progress in CLI or TUI
kvelmo watch
kvelmo tui
```

You can start in the browser, inspect in CLI, monitor in TUI, and come back to the browser for review and submit.

## Common Patterns

### Browser-First

Use this when you want the main product path:

1. create the task in the Web UI
2. plan and implement in the browser
3. review visually
4. submit from the same interface

### CLI-First

Use this when you want shell-native control:

```bash
kvelmo start --from github:owner/repo#123
kvelmo plan
kvelmo implement
kvelmo review
kvelmo submit
```

### Hybrid

Use this when you want speed in terminal plus visual review:

```bash
kvelmo start "Refactor the settings panel"
kvelmo plan
kvelmo implement
```

Then use the Web UI for diff-heavy review and submission.

## Deliberate Parity Boundaries

Not every feature should appear identically in every interface.

That is deliberate:

- the Web UI is strongest for visual review and multi-panel context
- the CLI is strongest for commands, automation, and raw tooling
- the TUI is strongest for compact terminal control

Choose the interface that matches the job instead of expecting every surface to look the same.
