# Workflow

kvelmo manages development tasks through a structured workflow that stays visible across the Web UI, CLI, desktop app, TUI, and RPC layer.

## Philosophy

The core idea is simple:

**The agent does the mechanical work. You control transitions, review, and submission.**

That principle shows up in the system design:

- phase transitions are explicit
- checkpoints make recovery normal, not exceptional
- interfaces stay synchronized through sockets and shared state
- review, approval, and policy sit around the agent instead of behind it

## The Common Path

The most common flow is:

```text
start -> plan -> implement -> review -> submit -> finish
```

That is the path most users will recognize in the Web UI and the path most docs should teach first.

## The Full Lifecycle

The actual lifecycle is broader:

```text
start -> plan -> implement -> simplify? -> optimize? -> review -> submit -> finish
```

The state machine also includes recovery and coordination states such as `waiting`, `paused`, and `failed`.

## 1. Start

Load a task and prepare the project workspace.

Typical sources include:

- local files
- GitHub
- GitLab
- Linear
- Wrike
- Jira
- Azure DevOps
- plain text input

What happens:

- task context is loaded
- task state moves to `loaded`
- git/worktree preparation happens
- the task becomes visible through every active interface

## 2. Plan

Generate a specification before code changes begin.

What happens:

- the agent inspects the task and codebase
- a reviewable plan is produced
- task state moves through planning into `planned`
- a checkpoint is recorded

Why it matters:

- it separates understanding from execution
- it gives humans a clean approval point before code churn starts

## 3. Implement

Execute the approved direction in the project worktree.

What happens:

- code changes are applied
- output and progress are streamed
- task state moves into `implementing` and then `implemented`
- another checkpoint is recorded

## 4. Simplify

Optionally run a cleanup pass focused on clarity.

Use this when the first implementation works but is more complex or verbose than necessary.

## 5. Optimize

Optionally run a follow-up pass focused on quality improvements.

Use this when you want to improve output quality, reduce complexity, or clean up rough edges after functional completion.

## 6. Review

Review is the human checkpoint before submission.

Depending on your setup, review may include:

- file diffs
- logs and live output
- findings and policy gates
- CI and security signals
- checklisting and approvals

This is where kvelmo stops being "agent runner" and behaves more like workflow control.

## 7. Submit

Create the pull request and update provider-facing task state.

At this point the work is ready to leave the machine and enter the team workflow.

## 8. Finish

Clean up after merge or after the task reaches its natural endpoint.

This usually means closing the loop on task state and repository state.

## Recovery

Recovery is part of the workflow, not an afterthought.

Common commands:

```bash
kvelmo undo
kvelmo redo
kvelmo reset
kvelmo retry
kvelmo stop
kvelmo abort
```

Use them to navigate checkpoints, recover from failed states, or interrupt long-running operations.

## Interface Perspective

### Web UI

Best for:

- dashboards
- visual task control
- live status and review context
- panel-based exploration of related features

### CLI

Best for:

- scripts and automation
- explicit command composition
- provider operations and shell-native tasks
- system-facing control paths

### Desktop App

Best for:

- local native shell around the same workflow

### TUI

Best for:

- full-screen terminal control without a browser
- live output monitoring in terminal-first environments

## Parity Boundaries

The workflow is shared across interfaces, but not every control is presented identically everywhere.

That is deliberate.

Examples of interface-specific emphasis:

- the Web UI is strongest for dashboards, panels, and review surfaces
- the CLI is strongest for raw command control, shell integration, and RPC
- the TUI is strongest for compact terminal-based steering

## Technical Details

For the formal state model, see [State Machine](/concepts/state-machine.md).
