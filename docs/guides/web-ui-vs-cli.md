# Web UI vs CLI vs TUI

kvelmo offers three interfaces. Choose the one that fits your workflow.

## Quick Comparison

| Aspect           | Web UI              | CLI        | TUI (`kvelmo tui`)      |
|------------------|---------------------|------------|-------------------------|
| Best for         | Non-technical users | Developers | SSH / headless servers  |
| Learning curve   | Lower               | Higher     | Medium                  |
| Visual feedback  | Yes                 | Limited    | Yes (terminal)          |
| Scripting        | No                  | Yes        | No                      |
| Keyboard-first   | No                  | Yes        | Yes                     |
| Real-time output | Yes                 | Yes        | Yes                     |
| Browser required | Yes                 | No         | No                      |

## When to Use Web UI

**Choose the Web UI when:**

- You prefer visual interfaces
- You're new to kvelmo
- You want click-through workflows
- You need to monitor multiple tasks
- You want easy access to file changes and diffs

**Web UI strengths:**

- Dashboard shows everything at a glance
- Sidebar panels for files, changes, checkpoints
- Click-based workflow
- No commands to memorize

## When to Use CLI

**Choose the CLI when:**

- You're comfortable with terminals
- You want to script workflows
- You need to integrate with other tools
- You prefer keyboard-driven workflows
- You're working in a headless environment

**CLI strengths:**

- Fast once you know the commands
- Scriptable and automatable
- Works over SSH
- Composable with other tools

## When to Use the TUI

**Choose the TUI (`kvelmo tui`) when:**

- You are working over SSH or in a headless environment where opening a browser is inconvenient
- You want the dashboard experience (live output, chat, worker status) without leaving the terminal
- You need keyboard-driven workflow transitions

**TUI strengths:**

- No browser required — works anywhere a terminal does
- Full-screen layout with live agent output streaming
- Interactive chat and one-key workflow triggers
- Supports multiple worktree tabs with `Tab`/`Shift+Tab`

See [kvelmo tui](/cli/tui.md) for full reference.

## Mixing Both

The interfaces share the same underlying engine. You can mix them:

```bash
# Start task via CLI
kvelmo start --from file:task.md

# Monitor in Web UI
# Open http://localhost:6337

# Continue via CLI
kvelmo plan
kvelmo implement

# Review changes in Web UI
# Use the Changes panel

# Submit via CLI
kvelmo submit
```

## Common Workflows

### Web UI Workflow

1. Open http://localhost:6337
2. Click **New Task**
3. Enter details, click **Start**
4. Click **Plan**, review specification
5. Click **Implement**, watch progress
6. Review in Changes panel
7. Click **Submit**

### CLI Workflow

```bash
kvelmo start --from file:task.md
kvelmo plan
kvelmo implement
kvelmo review
kvelmo submit
```

### Hybrid Workflow

```bash
# Start and plan via CLI (faster)
kvelmo start --from github:owner/repo#123
kvelmo plan

# Review specification in Web UI
# Open http://localhost:6337

# Implement and review via CLI
kvelmo implement
kvelmo review

# Submit via CLI
kvelmo submit
```

## Feature Availability

| Feature      | Web UI | CLI             | TUI             |
|--------------|--------|-----------------|-----------------|
| Start task   | Yes    | Yes             | No              |
| Plan         | Yes    | Yes             | Yes (key: `p`)  |
| Implement    | Yes    | Yes             | Yes (key: `i`)  |
| Review       | Yes    | Yes             | No              |
| Submit       | Yes    | Yes             | No              |
| Undo/Redo    | Yes    | Yes             | No              |
| File browser | Yes    | `ls` / `cat`    | No              |
| Diff viewer  | Yes    | `git diff`      | No              |
| Settings     | Yes    | `kvelmo config` | No              |
| Live output  | Yes    | `kvelmo watch`  | Yes             |
| Agent chat   | Yes    | `kvelmo chat`   | Yes             |
| Worker view  | Yes    | `kvelmo workers`| Dashboard layout|

## Recommendations

**New users:** Start with the Web UI to understand the workflow, then graduate to CLI for speed.

**Experienced users:** Use CLI for routine tasks, Web UI for complex reviews.

**Teams:** Use Web UI for collaboration, CLI for individual work.
