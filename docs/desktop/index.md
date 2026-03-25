# Desktop App

The desktop app is a native shell around the same local orchestration workflow used by the Web UI, CLI, and TUI.

It is not a separate backend or a cut-down edition. It is another interface over the same task state, checkpoints, and agent-driven workflow.

## What It Is Good For

Use the desktop app when you want:

- a local graphical shell without living in the terminal
- easier project selection and workflow control
- access to the core orchestration flow in a native application wrapper

## How It Fits With the Other Interfaces

The interfaces are complementary:

- **Web UI** is the primary browser-based experience
- **CLI** is strongest for scripting, automation, and shell-native operations
- **TUI** is strongest for terminal-first live control
- **Desktop app** is strongest when you want a native app shell for the same workflow

You can move between them because the underlying state is shared locally.

## Workflow Coverage

The desktop app participates in the same lifecycle:

```text
start -> plan -> implement -> review -> submit -> finish
```

Optional refinement and recovery flows still apply, including checkpoint-based undo/redo and reset paths.

## Getting Started

See [Desktop Getting Started](/desktop/getting-started.md).
