# FAQ

## General

### What is kvelmo?

kvelmo is a local orchestration system for AI-assisted development. It manages task state, worktree state, checkpoints, approvals, and interface synchronization across the Web UI, CLI, desktop app, TUI, and RPC endpoints.

### Is kvelmo an AI?

No. kvelmo coordinates AI agents. It is the workflow and control layer around them.

### Who is it for?

kvelmo is best understood as Web UI first, with strong support for CLI, desktop, and TUI workflows. It is useful for individual developers, operators, and teams who want structure around agent-driven code changes.

### Is kvelmo free?

Yes. kvelmo is BSD-3 licensed. You bring your own agent access where needed.

### Does kvelmo require API keys?

Not for the core product itself. Agent authentication depends on the agent path you choose. Some providers and API-backed agent integrations do require tokens.

## Installation

### How do I install kvelmo?

```bash
curl -fsSL https://raw.githubusercontent.com/valksor/kvelmo/master/install.sh | bash
```

Then verify your environment:

```bash
kvelmo version
kvelmo diagnose
```

### What are the requirements?

- Git
- A supported local environment such as macOS, Linux, or WSL2
- At least one working agent path such as Claude CLI, Codex, or a configured API-backed/custom agent

### Does kvelmo work on Windows?

Yes, through WSL2. See [Windows WSL Setup](/guides/windows-wsl.md).

## Interfaces

### Which interface should I start with?

Start with the Web UI unless you already know you want a terminal-first workflow.

```bash
kvelmo serve --open
```

Then open `http://localhost:6337` if needed.

### Does the CLI do more than the Web UI?

Some capabilities are intentionally CLI-oriented, such as shell integration, raw RPC, daemon lifecycle commands, and pipe-style workflows. The Web UI is still the primary place for day-to-day task orchestration and visual review.

### What is the TUI for?

The TUI is a full-screen terminal interface for live workflow control and output streaming.

```bash
kvelmo tui
```

### Is the desktop app different from the Web UI?

The desktop app is another local interface over the same core workflow. Think of it as a native shell, not a separate backend.

## Workflow

### What is the workflow?

The common path is:

1. `start`
2. `plan`
3. `implement`
4. `review`
5. `submit`
6. `finish`

Optional refinement phases such as `simplify` and `optimize` may also run depending on how you operate kvelmo.

### Can I skip planning?

Yes. `quick` starts a fast path that skips planning and auto-advances through the remaining phases.

```bash
kvelmo quick "Fix the broken settings copy"
```

### Can I undo changes?

Yes. kvelmo records checkpoints through the workflow.

```bash
kvelmo undo
kvelmo redo
```

### What happens if a task gets stuck?

Use the status and recovery tools first:

```bash
kvelmo status
kvelmo watch
kvelmo reset
```

### Can I resume later?

Yes. Task state is persisted locally. You can return through any interface that works for your environment.

## Configuration

### Where is the config file?

Global configuration lives at `~/.valksor/kvelmo/kvelmo.yaml`.

Project configuration lives at `.valksor/kvelmo.yaml`.

### Is configuration YAML or JSON?

Settings files are YAML. Some command output is printed as JSON for inspection, but the configuration files themselves are YAML.

### How do I change settings?

```bash
kvelmo config set <key> <value>
```

You can also inspect effective settings:

```bash
kvelmo config show
```

### How do provider tokens work?

Provider integrations such as GitHub, GitLab, Linear, Jira, Wrike, and Azure DevOps may require tokens or login flows. See [Configuration](/configuration/index.md) and [Providers](/providers/index.md).

## Agents

### Which agents are supported?

kvelmo can work with Claude, Codex, custom agents, and configured API-backed agents, depending on your local setup and settings.

### Can I use different agents for different phases?

Yes. The settings model supports phase-aware agent selection.

## Advanced Features

### Is kvelmo only for task execution?

No. It also includes operational and governance features:

- policy and approvals
- CI status and security scans
- activity logs and recordings
- exports and backups
- hooks and notifications
- memory and browser automation
- code graph tooling

### Can I manage more than one project?

Yes. The system supports global and per-worktree coordination, including project views, worker pools, and task grouping.

## Help

### Where can I report bugs?

[GitHub Issues](https://github.com/valksor/kvelmo/issues)

### Where can I ask questions?

[GitHub Discussions](https://github.com/valksor/kvelmo/discussions)

### Where is the documentation?

[Documentation](https://valksor.com/docs/kvelmo/nightly)
