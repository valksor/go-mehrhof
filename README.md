# kvelmo

[![valksor](https://badgen.net/static/org/valksor/green)](https://github.com/valksor)
[![BSD-3-Clause](https://img.shields.io/badge/BSD--3--Clause-green?style=flat)](https://github.com/valksor/kvelmo/blob/master/LICENSE)
[![GitHub Release](https://img.shields.io/github/release/valksor/kvelmo.svg?style=flat)](https://github.com/valksor/kvelmo/releases/latest)
[![GitHub last commit](https://img.shields.io/github/last-commit/valksor/kvelmo.svg?style=flat)](https://github.com/valksor/kvelmo/commits/master)
[![Coverage Status](https://coveralls.io/repos/github/valksor/kvelmo/badge.svg?branch=master)](https://coveralls.io/github/valksor/kvelmo?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/valksor/kvelmo)](https://goreportcard.com/report/github.com/valksor/kvelmo)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/valksor/kvelmo)

kvelmo is a socket-first orchestration system for AI-assisted development.

It runs locally, manages task state across a project, and coordinates agent-driven work from task intake through planning, implementation, review, and pull request submission. The Web UI is the main experience, while the CLI, desktop app, and TUI all operate on the same underlying workflow and project state.

kvelmo is not the agent. It is the system around the agent: worktree setup, checkpoints, approvals, sockets, governance, observability, and recovery.

## Why Teams Use It

- **Web-first workflow** for creating tasks, watching progress, reviewing changes, and managing project state visually
- **Human-gated delivery** so planning, implementation, review, and submission stay explicit and inspectable
- **Local orchestration** with no hosted control plane required for the core workflow
- **Shared workflow across interfaces** so you can start in the browser and finish in CLI or TUI without losing context
- **Recovery built in** through checkpoints, undo/redo, reset paths, and resumable task state
- **Operational depth** beyond the happy path: policy, CI, security, activity logs, exports, recordings, notifications, and task grouping

## How It Works

```text
start -> plan -> implement -> simplify? -> optimize? -> review -> submit -> finish
```

Each phase updates task state, records checkpoints, and exposes progress through sockets to every interface.

At a high level:

1. **Start** loads work from a file or provider and prepares the workspace
2. **Plan** produces a reviewable specification before code changes begin
3. **Implement** executes the approved direction in the project worktree
4. **Simplify** and **Optimize** are optional refinement passes
5. **Review** adds the human checkpoint before code leaves the machine
6. **Submit** creates the PR and updates provider-facing state
7. **Finish** cleans up after merge

## Choose Your Interface

### Web UI

The browser UI is the primary experience.

Start the local server:

```bash
kvelmo serve --open
```

Then open `http://localhost:6337` if it did not open automatically.

Use the Web UI when you want:

- project and task dashboards
- visual workflow controls
- live output, chat, logs, and review context
- access to panels for memory, screenshots, browser automation, jobs, code graph, recordings, policy, CI, and more

**[Web UI Guide](https://valksor.com/docs/kvelmo/nightly/#/web-ui/getting-started)**

### CLI

The CLI is the most direct interface for scripting, shell-native workflows, automation, and provider operations.

```bash
kvelmo start --from file:task.md
kvelmo plan
kvelmo implement
kvelmo review
kvelmo submit
```

Use it when you want composable commands, automation, explicit control, or access to features that are intentionally shell-oriented.

**[CLI Reference](https://valksor.com/docs/kvelmo/nightly/#/cli/index)**

### Desktop App

The desktop app packages the same workflow in a native shell for people who want a local graphical experience without living in the terminal.

**[Desktop App Guide](https://valksor.com/docs/kvelmo/nightly/#/desktop/index)**

### TUI

The TUI gives you a full-screen terminal dashboard with workflow control and live output, without switching to a browser.

```bash
kvelmo tui
```

## What kvelmo Is

- **An orchestrator for development work**, not a coding model by itself
- **A local control plane** for project tasks, agent execution, and interface synchronization
- **A workflow system** with explicit transitions, approvals, and state recovery
- **A multi-surface product** spanning Web UI, CLI, desktop, TUI, and RPC
- **A governance layer** for policy checks, approvals, auditability, and submission discipline

## What kvelmo Is Not

- **Not a one-click autonomous coder** that silently edits and ships code
- **Not a hosted SaaS dependency** for the core local workflow
- **Not only a README-to-PR shortcut**; it also handles the operational edges around agent work
- **Not limited to a single agent**; Claude, Codex, and custom/API-backed agents can participate

## Key Capabilities

| Capability | What It Covers |
|------------|----------------|
| **Task intake** | Files plus providers such as GitHub, GitLab, Linear, Wrike, Jira, and Azure DevOps |
| **Workflow orchestration** | Start, plan, implement, simplify, optimize, review, submit, finish |
| **Checkpointing** | Undo, redo, reset, abandon, and resumable task history |
| **Observability** | Status, watch, logs, activity, jobs, stats, recordings, event log |
| **Governance** | Approvals, checklisting, policy, CI checks, security scanning, audit trail |
| **Project intelligence** | Memory, screenshots, browser automation, code graph, discovery, recap, explain |
| **Operations** | Backups, exports, notifications, access tokens, hooks, worker pools, task groups |

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/valksor/kvelmo/master/install.sh | bash
```

After installation:

```bash
kvelmo version
kvelmo diagnose
```

If your preferred agent CLI is available locally, kvelmo can orchestrate it. Claude CLI is the default path, and Codex plus custom/API-backed agents are also supported.

## Common Commands

```bash
# Run the local server and Web UI
kvelmo serve --open

# Start a task from plain text or a provider
kvelmo start "Refactor the settings panel"
kvelmo start --from github:owner/repo#123

# Core workflow
kvelmo plan
kvelmo implement
kvelmo review
kvelmo submit

# Recovery and navigation
kvelmo status
kvelmo watch
kvelmo undo
kvelmo redo
kvelmo reset

# Alternate interfaces
kvelmo tui
```

## Documentation

| Guide | Description |
|-------|-------------|
| [Quickstart](https://valksor.com/docs/kvelmo/nightly/#/quickstart) | Start with the Web UI and learn the workflow |
| [Web UI Guide](https://valksor.com/docs/kvelmo/nightly/#/web-ui/getting-started) | Main browser experience |
| [CLI Reference](https://valksor.com/docs/kvelmo/nightly/#/cli/index) | Full command reference |
| [Desktop App](https://valksor.com/docs/kvelmo/nightly/#/desktop/index) | Native shell walkthrough |
| [Workflow Concepts](https://valksor.com/docs/kvelmo/nightly/#/concepts/workflow) | States, gates, and recovery |
| [Providers](https://valksor.com/docs/kvelmo/nightly/#/providers/index) | External task sources |
| [Configuration](https://valksor.com/docs/kvelmo/nightly/#/configuration/index) | Settings, tokens, and behavior |

## Development

```bash
make build
make quality
make test
```

See `CONTRIBUTING.md` for the full contributor workflow.

## Prototype

`prototype/` contains the historical reference implementation that informed the current architecture. It is preserved as read-only reference material and is not the active development target.

## License

[BSD 3-Clause License](LICENSE)
