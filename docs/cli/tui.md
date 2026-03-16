# kvelmo tui

Open the interactive terminal UI.

## Usage

```bash
kvelmo tui [--layout stacked|dashboard]
```

## Description

`kvelmo tui` launches a full-screen terminal interface that brings the kvelmo dashboard into your terminal. It is a browser-free alternative designed for SSH sessions, headless servers, and developers who prefer staying in the terminal.

On startup it auto-detects the current project from the working directory, connects to the worktree socket, and begins streaming live agent output. You can chat with the agent, trigger workflow transitions, and navigate between worktrees — all without leaving the terminal.

## Options

| Flag       | Default   | Description                                    |
|------------|-----------|------------------------------------------------|
| `--layout` | `stacked` | UI layout: `stacked` (default) or `dashboard` |

## Configuration

Set a default layout in `~/.valksor/kvelmo/kvelmo.yaml`:

```yaml
tui:
  layout: stacked  # or dashboard
```

The `--layout` flag overrides this setting for the current session.

## Keyboard Shortcuts

| Key            | Action                        |
|----------------|-------------------------------|
| `Tab`          | Switch to next worktree       |
| `Shift+Tab`    | Switch to previous worktree   |
| `Enter`        | Send chat message             |
| `p`            | Trigger plan                  |
| `i`            | Trigger implement             |
| `s`            | Stop current job              |
| `Ctrl+A`       | Abort task                    |
| `Ctrl+U`       | Undo checkpoint               |
| `Ctrl+R`       | Redo checkpoint               |
| `q` / `Ctrl+C` | Quit                          |
| `?`            | Toggle keybinding help        |

## Layouts

### stacked (default)

Three vertically stacked panes: task status on top, live agent output in the middle, and a chat input area at the bottom.

```
┌─────────────────────────────┐
│  Status: implementing       │
├─────────────────────────────┤
│                             │
│  Agent output (scrollable)  │
│                             │
├─────────────────────────────┤
│  > chat input               │
└─────────────────────────────┘
```

### dashboard

Adds a workers pane alongside the output pane, giving a wider view of all active workers and their current jobs.

```
┌─────────────────────────────┐
│  Status: implementing       │
├──────────────────┬──────────┤
│                  │ Workers  │
│  Agent output    │──────────│
│                  │ w1  busy │
│                  │ w2  idle │
├──────────────────┴──────────┤
│  > chat input               │
└─────────────────────────────┘
```

## Examples

```bash
# Open TUI in the current project directory
kvelmo tui

# Open with dashboard layout to see worker status
kvelmo tui --layout dashboard

# Open from a specific worktree path
cd /path/to/project && kvelmo tui
```

## Related

- [watch](/cli/watch.md) — Stream output without an interactive UI
- [chat](/cli/chat.md) — Interactive agent conversation (CLI only)
- [status](/cli/status.md) — Check current task state
- [Web UI Getting Started](/web-ui/getting-started.md) — Browser-based alternative
