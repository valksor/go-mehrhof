# Valksor Mehrhof - AI-Powered Task Automation CLI

[![valksor](https://badgen.net/static/org/valksor/green)](https://github.com/valksor)
[![BSD-3-Clause](https://img.shields.io/badge/BSD--3--Clause-green?style=flat)](https://github.com/valksor/go-mehrhof/blob/master/LICENSE)
[![Coverage Status](https://coveralls.io/repos/github/valksor/go-mehrhof/badge.svg?branch=refs/tags/nightly)](https://coveralls.io/github/valksor/go-mehrhof?branch=refs/tags/nightly)

---

**⚠️ EXPERIMENTAL INTEGRATIONS**

Mehrhof's core workflow engine is stable, but **provider and agent integrations with third-party services are experimental**.

Due to the large number of external APIs (GitHub, GitLab, Jira, Notion, Claude, Gemini, etc.), integrations may:
- Break without notice due to third-party API changes
- Have edge cases not covered by automated tests
- Require manual validation for production use

We are gradually testing and hardening integrations. Report issues at [github.com/valksor/go-mehrhof/issues](https://github.com/valksor/go-mehrhof/issues).

---

## Why Mehrhof?

Mehrhof is a command-line tool that orchestrates AI agents to perform **planning, implementation, and code review** workflows for software development tasks.

**Key benefits:**
- **Parallel tasks** - Run multiple AI tasks simultaneously using git worktrees for isolated development
- **Workflow engine** - Reliable plan → implement → review → finish cycle with checkpointing and undo/redo
- **Provider integrations** - Connect to 15+ task sources (GitHub issues, Jira, Linear, Notion, etc.)
- **State tracking** - Task state persists across sessions; resume anytime with `mehr continue`
- **Auto mode** - Fully automated workflow: `mehr auto file:task.md` handles everything
- **Self-updating** - Auto-update from GitHub releases, no manual reinstall

## Quick Start

### Prerequisites

- **Go 1.25+** (for building from source)
- **Git** (for version control operations)
- **Claude CLI** - Mehrhof wraps Claude CLI for AI operations (see [quickstart](https://valksor.github.io/go-mehrhof/#/quickstart))

### Installation

#### Option 1: Pre-built Binary (Recommended)

Download the latest release for your platform:

| Platform | Architecture | Binary Name |
|----------|--------------|-------------|
| Linux | AMD64 | `mehr-linux-amd64` |
| Linux | ARM64 | `mehr-linux-arm64` |
| macOS | AMD64 (Intel) | `mehr-darwin-amd64` |
| macOS | ARM64 (Apple Silicon) | `mehr-darwin-arm64` |

```bash
# Download and install (example for macOS ARM64)
curl -L https://github.com/valksor/go-mehrhof/releases/latest/download/mehr-darwin-arm64 -o mehr
chmod +x mehr
sudo mv mehr /usr/local/bin/

# Verify
mehr version
```

#### Option 2: Nightly Build (Pre-release)

Get the latest commit build (always available, use with caution):

```bash
# Download latest nightly (example for macOS ARM64)
curl -L https://github.com/valksor/go-mehrhof/releases/download/nightly/mehr-darwin-arm64 -o mehr
chmod +x mehr
sudo mv mehr /usr/local/bin/
```

**Note:** Nightly builds are pre-release and may contain untested changes. Prefer stable releases when available.

#### Option 3: Build from Source

```bash
git clone https://github.com/valksor/go-mehrhof.git
cd go-mehrhof
make install

# Verify
mehr version
```

### First Task

```bash
# 1. Initialize workspace
mehr init

# 2. Create a task file
cat > task.md << 'EOF'
---
title: Add health check endpoint
---
Create a /health endpoint that returns HTTP 200 with JSON status.
EOF

# 3. Start the task (creates git branch)
mehr start task.md

# 4. Generate specifications
mehr plan

# 5. Implement
mehr implement

# 6. Review and finish
mehr finish
```

**Workflow Overview**: `init` → `start` → `plan` → `implement` → `review` → `finish`

## How It Works

```
┌─────────────────────────────────────────────────────────────┐
│                                                             │
│  mehr init  →  mehr start  →  mehr plan  →                  │
│                                                             │
│  →  mehr implement  →  mehr review  →  mehr finish          │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

1. **Initialize** (`mehr init`) - Set up workspace (one-time)
2. **Start** (`mehr start`) - Begin a task; creates git branch automatically
3. **Plan** (`mehr plan`) - AI generates implementation specifications
4. **Implement** (`mehr implement`) - AI executes the specifications
5. **Review** (`mehr review`) - Run automated code review
6. **Finish** (`mehr finish`) - Merge changes and clean up

**Recovery commands**:
- `mehr continue` - Resume workflow, optionally auto-execute (`--auto`)
- `mehr undo` / `mehr redo` - Revert to previous checkpoint
- `mehr abandon` - Abandon task without merging

## Essential Commands

| Command | Description |
|---------|-------------|
| `mehr init` | Initialize workspace (creates `.mehrhof/` directory) |
| `mehr start <ref>` | Start task from file, directory, or provider |
| `mehr auto <ref>` | Full automation: plan → implement → review → finish |
| `mehr plan` | Generate AI implementation specifications |
| `mehr implement` | Execute the specifications |
| `mehr review` | Run automated code review |
| `mehr status` | Full task inspection: specs, checkpoints, sessions |
| `mehr continue` | Resume workflow, optionally auto-execute (aliases: `cont`, `c`) |
| `mehr finish` | Complete task and merge changes |
| `mehr list` | List all tasks in workspace |
| `mehr undo` / `mehr redo` | Navigate checkpoints |
| `mehr note <msg>` | Add notes for AI context |

**See [CLI Reference](https://valksor.github.io/go-mehrhof/#/cli/index) for all commands and flags.**

## Task Providers

Mehrhof supports 15+ task sources. Use provider schemes to load tasks:

| Provider | Scheme | Example | Docs |
|----------|--------|---------|------|
| File | `file:` | `file:task.md` | [file](https://valksor.github.io/go-mehrhof/#/providers/file) |
| Directory | `dir:` | `dir:./tasks/` | [directory](https://valksor.github.io/go-mehrhof/#/providers/directory) |
| GitHub | `github:` | `github:123` | [github](https://valksor.github.io/go-mehrhof/#/providers/github) |
| GitLab | `gitlab:` | `gitlab:123` | [gitlab](https://valksor.github.io/go-mehrhof/#/providers/gitlab) |
| Bitbucket | `bitbucket:` | `bitbucket:123` | [bitbucket](https://valksor.github.io/go-mehrhof/#/providers/bitbucket) |
| Jira | `jira:` | `jira:PROJ-123` | [jira](https://valksor.github.io/go-mehrhof/#/providers/jira) |
| Linear | `linear:` | `linear:ENG-123` | [linear](https://valksor.github.io/go-mehrhof/#/providers/linear) |
| Asana | `asana:` | `asana:1234...` | [asana](https://valksor.github.io/go-mehrhof/#/providers/asana) |
| ClickUp | `clickup:` | `clickup:abc123` | [clickup](https://valksor.github.io/go-mehrhof/#/providers/clickup) |
| Azure DevOps | `azdo:` | `azdo:123` | [azure-devops](https://valksor.github.io/go-mehrhof/#/providers/azure-devops) |
| Notion | `notion:` | `notion:<uuid>` | [notion](https://valksor.github.io/go-mehrhof/#/providers/notion) |
| Trello | `trello:` | `trello:<id>` | [trello](https://valksor.github.io/go-mehrhof/#/providers/trello) |
| Wrike | `wrike:` | `wrike:<id>` | [wrike](https://valksor.github.io/go-mehrhof/#/providers/wrike) |
| YouTrack | `youtrack:` | `youtrack:ABC-123` | [youtrack](https://valksor.github.io/go-mehrhof/#/providers/youtrack) |

**Default provider**: Configure in `.mehrhof/config.yaml` to use bare references:
```yaml
providers:
  default: file  # "mehr start task.md" works without "file:" prefix
```

## Parallel Tasks with Worktrees

Run multiple tasks simultaneously in isolated environments:

```bash
# Terminal 1
mehr start --worktree feature-a.md
cd ../project-worktrees/<task-id>
mehr plan && mehr implement

# Terminal 2 (from main repo)
mehr start --worktree feature-b.md
cd ../project-worktrees/<task-id>
mehr plan && mehr implement
```

Each worktree is an isolated git checkout. Mehrhof auto-detects which task you're working on based on your current directory.

## AI Agents

Mehrhof supports multiple AI backends:

| Agent | Description |
|-------|-------------|
| Claude | Default agent via Claude CLI (recommended) |
| Gemini | Google's Gemini models |
| Ollama | Local models (Llama, Mistral, etc.) |
| Aider | AI pair programming tool |
| Copilot | GitHub Copilot integration |
| OpenRouter | Multiple models via OpenRouter API |
| Codex | OpenAI Codex models |

**See [AI Agents documentation](https://valksor.github.io/go-mehrhof/#/agents/index) for configuration and custom aliases.**

## Configuration

Project-level configuration in `.mehrhof/config.yaml`:

```yaml
# Git integration
git:
  auto_commit: true
  commit_prefix: "[{key}]"
  branch_pattern: "{type}/{key}--{slug}"
  target_branch: "main"

# Agent configuration
agent:
  default: claude
  timeout: 300

# Default provider for bare references
providers:
  default: file

# Auto-update checks
update:
  enabled: true
  check_interval: 24  # hours
```

**See [Configuration Guide](https://valksor.github.io/go-mehrhof/#/configuration/index) for all options including agent aliases, per-step agents, and provider settings.**

## Documentation

- 📖 [Full Documentation](https://valksor.github.io/go-mehrhof)
- [Quickstart](https://valksor.github.io/go-mehrhof/#/quickstart) - Install and first task in 5 minutes
- [Guides](https://valksor.github.io/go-mehrhof/#/guides/first-task) - Step-by-step tutorials
- [Providers](https://valksor.github.io/go-mehrhof/#/providers/index) - Task source integrations
- [AI Agents](https://valksor.github.io/go-mehrhof/#/agents/index) - Agent configuration and aliases
- [CLI Reference](https://valksor.github.io/go-mehrhof/#/cli/index) - All commands and flags
- [Configuration](https://valksor.github.io/go-mehrhof/#/configuration/index) - Customize behavior
- [Concepts](https://valksor.github.io/go-mehrhof/#/concepts/workflow) - Workflow, storage, architecture
- [Troubleshooting](https://valksor.github.io/go-mehrhof/#/troubleshooting/index) - Common issues

## Development

```bash
make build        # Build binary to ./build/mehr
make install      # Install to $GOPATH/bin
make test         # Run tests with coverage
make coverage     # Generate coverage report
make lint         # Run golangci-lint
make fmt          # Format code
make tidy         # Tidy dependencies
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development guidelines.

## License

By contributing to Mehrhof, you agree that your contributions will be licensed under the [BSD 3-Clause License](LICENSE).

