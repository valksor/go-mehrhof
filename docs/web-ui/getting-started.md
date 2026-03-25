# Getting Started with Web UI

The Web UI is the primary way to use kvelmo.

It gives you a project dashboard, task controls, live output, review context, and access to the broader orchestration features that sit around agent-driven development work.

## Start the Server

From the project you want to work on:

```bash
kvelmo serve --open
```

If your browser does not open automatically, go to `http://localhost:6337`.

## What You See First

The Web UI is organized around two broad views:

- **Global view** for project-level navigation, worker visibility, and cross-project state
- **Project view** for the active task, workflow controls, live output, and supporting panels

## Create a Task

Use the task creation flow in the UI to start work from:

- a freeform description
- a local file
- a connected provider reference

Once created, the task enters the shared workflow used by every interface.

## Run the Workflow in the Browser

### Plan

Generate and inspect the specification before implementation begins.

### Implement

Run the implementation phase and follow live output as the agent works.

### Simplify and Optimize

Use optional refinement passes when the task needs a cleanup or quality pass after first completion.

### Review

Inspect changes and surrounding context before approving the result.

### Submit

Create the pull request when the task is ready.

## Where Features Show Up

The Web UI is broader than a single linear task view. Features may appear as:

- persistent tabs or panels
- project widgets
- drawers and dialogs
- contextual review surfaces
- global dashboards

Common areas include:

| Area | Typical capabilities |
|------|----------------------|
| **Project dashboard** | Task state, actions, progress, quick controls |
| **Panels and tabs** | Chat, output, files, browser, screenshots, jobs, review context |
| **Context widgets** | Activity, logs, memory, recordings, code graph, policy, CI |
| **Dialogs and drawers** | Settings, exports, reports, backups, access management |
| **Global views** | Projects, workers, cross-project status |

## What the Web UI Is Best At

Use the Web UI when you want:

- the clearest day-to-day task workflow
- visual review and comparison surfaces
- multiple context streams on screen at once
- easy movement between orchestration features without remembering commands

## How It Relates to CLI, Desktop, and TUI

The Web UI is not a separate product tier. It is one interface over the same local orchestration system.

That means you can:

- start a task in the browser
- inspect it with `kvelmo status`
- operate it in `kvelmo tui`
- return to the browser for review and submit

Some commands stay CLI-first by design, including shell integration, raw RPC access, daemon lifecycle management, and pipe-style workflows.

## Recovery and Control

The browser workflow still benefits from the same recovery model as the CLI:

- checkpoints support undo and redo
- reset paths exist for recovery
- logs, activity, and status views help diagnose stuck work
- submission remains explicit

## Useful Companion Commands

```bash
kvelmo status
kvelmo watch
kvelmo review
kvelmo undo
kvelmo redo
kvelmo reset
kvelmo tui
```

## Next Reading

- [Quickstart](/quickstart.md)
- [Workflow Concepts](/concepts/workflow.md)
- [CLI Reference](/cli/index.md)
- [Configuration](/configuration/index.md)
