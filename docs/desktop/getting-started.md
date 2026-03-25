# Getting Started with the Desktop App

The desktop app gives you a native shell around the same local orchestration model used by the Web UI and CLI.

## Install and Launch

1. Download the application from the project releases page
2. Install it for your platform
3. Launch the app and choose the project you want to work on

## What You Do in the App

The common path is the same as elsewhere:

1. create or load a task
2. plan it
3. implement it
4. review it
5. submit it
6. finish after merge

## What to Expect

The desktop app is best treated as a native wrapper around the local kvelmo system:

- tasks are still local
- checkpoints still govern recovery
- agent execution still happens through the same orchestration model
- other interfaces can inspect or continue the same task state

## When to Use Something Else

Switch to another interface when it fits better:

- use the **Web UI** for richer dashboard and panel-heavy review flows
- use the **CLI** for automation, provider operations, and shell-native control
- use the **TUI** for full-screen terminal steering

## Related

- [Desktop Overview](/desktop/index.md)
- [Quickstart](/quickstart.md)
- [Web UI Getting Started](/web-ui/getting-started.md)
