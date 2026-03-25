# Quickstart

This guide gets you into kvelmo with the Web UI first, then shows where CLI, desktop, and TUI fit.

## What You'll Do

By the end of this guide, you will have:

1. Installed kvelmo locally
2. Started the local server
3. Created a task in the Web UI
4. Walked through plan, implement, review, and submit
5. Seen how the other interfaces connect to the same workflow

## Prerequisites

**Git**

kvelmo uses Git for branches, checkpoints, diffs, and recovery.

**An agent runtime**

kvelmo orchestrates your local agent setup. Claude CLI is the default path, and Codex plus custom/API-backed agents are also supported.

Check what you already have:

```bash
claude --version
codex --version
```

You only need one working agent path, not every option.

## Install kvelmo

```bash
curl -fsSL https://raw.githubusercontent.com/valksor/kvelmo/master/install.sh | bash
```

Verify the install and local environment:

```bash
kvelmo version
kvelmo diagnose
```

For platform-specific setup, see [Installation Guide](/INSTALL.md).

## Start With the Web UI

The Web UI is the main experience and the best place to understand the product.

From the project you want to work in:

```bash
cd /path/to/your/project
kvelmo serve --open
```

If your browser does not open automatically, go to `http://localhost:6337`.

## Create Your First Task

In the Web UI:

1. Open or register the project you want to work on
2. Click the control for creating a new task
3. Enter a short title and a concrete description
4. Start the task

A task can also come from providers such as GitHub, GitLab, Linear, Wrike, Jira, Azure DevOps, or a local file. The Web UI and CLI both operate on the same underlying task state.

## Run the Workflow

### 1. Plan

Generate a specification before code changes begin.

Review the plan carefully. This is the first major control point.

### 2. Implement

Run the implementation phase after the plan looks right.

Watch live output in the UI while the agent works through the task.

### 3. Simplify and Optimize

Depending on the workflow and settings, you may also run optional cleanup and quality passes.

Use these when you want a second pass focused on clarity or polish rather than first-pass completion.

### 4. Review

Inspect file changes, output, findings, logs, and any supporting context before approving the result.

### 5. Submit

Create the pull request when the task is ready.

After merge, use `finish` to clean up task state and branch state.

## Other Interfaces

### CLI

Use the CLI for direct control, scripts, automation, or provider-heavy workflows.

```bash
kvelmo start --from file:task.md
kvelmo plan
kvelmo implement
kvelmo review
kvelmo submit
```

### Desktop App

Use the desktop app if you want a native shell around the same local workflow.

### TUI

Use the terminal UI when you want a full-screen interface without a browser.

```bash
kvelmo tui
```

## Shared State Across Interfaces

The interfaces are not separate products. They are separate views into the same local orchestration system.

That means you can:

- start a task in the Web UI
- inspect or steer it in CLI
- watch it in TUI
- return to the browser for review and submission

Some capabilities are intentionally interface-specific. For example, shell integration and raw RPC are CLI-oriented, while dashboards and panel-heavy exploration are strongest in the Web UI.

## Useful Commands

```bash
kvelmo status    # snapshot of current task state
kvelmo watch     # stream live progress
kvelmo undo      # step back to previous checkpoint
kvelmo redo      # step forward to next checkpoint
kvelmo reset     # recover from stuck state
kvelmo review    # enter review before submission
kvelmo finish    # clean up after merge
```

## What's Next?

- [Web UI Guide](/web-ui/getting-started.md)
- [CLI Reference](/cli/index.md)
- [Workflow Concepts](/concepts/workflow.md)
- [Providers](/providers/index.md)
- [Configuration](/configuration/index.md)
