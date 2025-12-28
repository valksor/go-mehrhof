# Mehrhof - AI-Powered Task Automation CLI

[![valksor](https://badgen.net/static/org/valksor/green)](https://github.com/valksor)
[![BSD-3-Clause](https://img.shields.io/badge/BSD--3--Clause-green?style=flat)](https://github.com/valksor/go-mehrhof/blob/master/LICENSE)
[![Coverage Status](https://coveralls.io/repos/github/valksor/go-mehrhof/badge.svg?branch=master)](https://coveralls.io/github/valksor/go-mehrhof?branch=master)

A command-line tool for AI-assisted task automation. It orchestrates AI agents to perform planning, implementation, and code review workflows.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Parallel Tasks](#parallel-tasks)
- [Commands](#commands)
- [Configuration](#configuration)
- [Workflow](#workflow)
- [Providers](#providers)
- [Plugins](#plugins)
- [Architecture](#architecture)
- [Development](#development)
- [Troubleshooting](#troubleshooting)

## Prerequisites

Before installing Mehrhof, ensure you have the following:

- **Go 1.25+** - Required for building from source
- **Git** – Required for version control operations
- **Claude API Key** - Set `ANTHROPIC_API_KEY` environment variable for AI agent

Optional:

- **CodeRabbit CLI** - For automated code reviews (`npm install -g coderabbitai`)
- **GitHub Token** - For GitHub issue integration (`GITHUB_TOKEN` or `MEHR_GITHUB_TOKEN`)

## Installation

### From Source

```bash
# Clone the repository
git clone <repository-url>
cd go-mehrhof

# Build the binary
make build

# Install to $GOPATH/bin
make install
```

### Verify Installation

```bash
mehr version
```

## Quick Start

```bash
# 1. Initialize workspace (creates .mehrhof directory)
mehr init

# 2. Start working on a task from a markdown file
mehr start file:task.md

# 3. Create implementation specifications
mehr plan

# 4. Implement the specifications
mehr implement

# 5. Check current status
mehr status

# 6. Complete and merge changes
mehr finish
```

**Note:** The `file:` prefix is the provider scheme. You can configure a default provider in `.mehrhof/config.yaml` to omit it.

## Parallel Tasks

Run multiple tasks simultaneously in separate terminals using worktrees.

### Starting Parallel Tasks

```bash
# Terminal 1: Start first task with worktree
mehr start --worktree file:feature-a.md
# Output: Task registered: a1b2c3d4
#         Worktree: ../project-worktrees/a1b2c3d4

cd ../project-worktrees/a1b2c3d4
mehr plan && mehr implement

# Terminal 2: Start second task (from main repo)
mehr start --worktree file:feature-b.md
# Output: Task registered: e5f6g7h8
#         Worktree: ../project-worktrees/e5f6g7h8

cd ../project-worktrees/e5f6g7h8
mehr plan && mehr implement
```

### Listing All Tasks

```bash
mehr list
```

Output:

```
TASK ID     STATE           TITLE                    WORKTREE                         ACTIVE
a1b2c3d4    implementing    Add authentication       ../project-worktrees/a1b2c3d4    →
e5f6g7h8    planning        Fix database queries     ../project-worktrees/e5f6g7h8
c9d0e1f2    done            Update README            -

Legend: * = active task in main repo, → = current worktree
```

### How It Works

- **Worktrees** are isolated git checkouts, each on its own branch
- **Auto-detection**: Commands automatically detect which task you're working on based on your current directory
- **Shared storage**: All tasks share `.mehrhof/` in the main repo
- **File locking**: Prevents race conditions when multiple processes access shared data

### Worktree vs Branch-Only

| Feature          | `--worktree`   | `--branch` (default) |
| ---------------- | -------------- | -------------------- |
| Parallel tasks   | Yes            | No                   |
| Isolation        | Full directory | Same directory       |
| Branch switching | Not needed     | Required             |
| Disk usage       | Higher         | Lower                |

## Commands

### `mehr init`

Initialize the task workspace by creating the `.mehrhof` directory and updating `.gitignore`.

```bash
mehr init
```

This creates:

- `.mehrhof/` directory for task storage
- `.mehrhof/config.yaml` with default configuration
- Updates `.gitignore` to exclude task files

---

### `mehr start <reference>`

Start working on a task from a file, directory, or external provider.

```bash
mehr start file:task.md              # From a markdown file (scheme required)
mehr start dir:./tasks/              # From a directory with README.md
mehr start --worktree file:task.md   # With separate worktree (for parallel tasks)
mehr start --branch=false file:task.md  # Without creating a git branch
```

**Provider Schemes:**
You must specify a provider scheme prefix (e.g., `file:`, `dir:`). Alternatively, configure a default provider in `.mehrhof/config.yaml`:

```yaml
providers:
  default: file # Bare references like "task.md" will use file: provider
```

**Flags:**
| Flag | Description | Default |
|------|-------------|---------|
| `-a, --agent <name>` | Agent to use | auto-detect |
| `-b, --branch` | Create git branch for the task (use `--branch=false` to disable) | `true` |
| `-w, --worktree` | Create separate git worktree (enables parallel tasks) | `false` |
| `-k, --key <key>` | External key for branch/commit naming (e.g., `FEATURE-123`) | auto-detect |
| `--commit-prefix <template>` | Commit prefix template (e.g., `[{key}]`) | `[{key}]` |
| `--branch-pattern <template>` | Branch pattern template (e.g., `{type}/{key}--{slug}`) | `{type}/{key}--{slug}` |

**Naming Variables:**
| Variable | Description | Example |
|----------|-------------|---------|
| `{key}` | External key from filename/frontmatter/CLI | `FEATURE-123` |
| `{task_id}` | Internal task ID | `a1b2c3d4` |
| `{type}` | Task type from filename prefix | `feature`, `fix`, `task` |
| `{slug}` | URL-safe slugified title | `add-user-auth` |

**Key Resolution Priority:** CLI `--key` flag > YAML frontmatter `key:` > filename extraction > task ID fallback

---

### `mehr continue`

Show status and suggested next actions for the current task. Useful when returning to work after a break.

```bash
mehr continue
```

This will:

- Display current task status
- Suggest the most appropriate next action based on state
- Show available undo/redo options

---

### `mehr plan`

Create implementation specifications for the active task.

```bash
mehr plan                    # Create specs for active task
mehr plan --verbose          # Show agent output
mehr plan --new              # Start standalone planning (no task required)
mehr plan --new "build a CLI"  # Standalone planning with seed topic
```

This runs the planning phase to analyze the task and create structured `specification-N.md` files in the work directory. You can run this multiple times to create additional specifications.

**Flags:**
| Flag | Description | Default |
|------|-------------|---------|
| `-n, --new` | Start standalone planning without a task | `false` |
| `-s, --seed <topic>` | Initial topic for standalone planning | - |

**Standalone Planning:**
With `--new`, you can start an interactive planning session without an active task. This is useful for exploring requirements before creating a formal task. Plans are saved to `.mehrhof/planned/` directory.

---

### `mehr implement`

Implement the specifications for the active task.

```bash
mehr implement               # Implement the specs
mehr implement --dry-run     # Preview without making changes
mehr implement --verbose     # Show agent output
```

**Flags:**
| Flag | Description | Default |
|------|-------------|---------|
| `-n, --dry-run` | Don't apply file changes (preview only) | `false` |

Requires at least one SPEC file (run `mehr plan` first).

---

### `mehr talk [message]`

Enter interactive conversation mode with the AI agent.

```bash
mehr talk                        # Enter interactive mode
mehr talk "Help me debug this"   # Start with a message
mehr talk --continue             # Continue previous session
```

**Flags:**
| Flag | Description | Default |
|------|-------------|---------|
| `-c, --continue` | Continue previous session | `false` |
| `-s, --session <file>` | Specific session file to continue | - |

---

### `mehr status`

Show the status of the current task.

```bash
mehr status              # Show current task status
mehr status --all        # Show all tasks (same as mehr list)
```

In a worktree, automatically shows the task associated with that worktree.

**Flags:**
| Flag | Description | Default |
|------|-------------|---------|
| `-a, --all` | Show all tasks | `false` |

---

### `mehr list`

List all tasks in the workspace with their worktree paths and states.

```bash
mehr list                # List all tasks
mehr list --worktrees    # Show only tasks with worktrees
```

**Flags:**
| Flag | Description | Default |
|------|-------------|---------|
| `-w, --worktrees` | Show only tasks with worktrees | `false` |

---

### `mehr agents list`

List all available AI agents, including built-in agents and user-defined aliases.

```bash
mehr agents list
```

Output:

```
NAME      TYPE      EXTENDS  AVAILABLE  DESCRIPTION
claude    built-in  -        yes        -
glm       alias     claude   yes        Claude with GLM API key
glm-fast  alias     glm      yes        GLM with lower token limit
```

See [Agent Aliases](#agent-aliases) for configuration details.

---

### `mehr config validate`

Validate workspace configuration (`.mehrhof/config.yaml`).

```bash
mehr config validate                    # Validate workspace config
mehr config validate --strict           # Treat warnings as errors
mehr config validate --format json      # JSON output for CI
```

**Flags:**
| Flag | Description | Default |
|------|-------------|---------|
| `--strict` | Treat warnings as errors | `false` |
| `--format` | Output format: `text`, `json` | `text` |

**Validations:**

- YAML syntax validity
- Agent alias circular dependencies
- Undefined agent references
- Git pattern template validity
- Plugin configuration

---

### `mehr review`

Run automated code review on current changes.

> **Note:** This command is a thin wrapper that delegates to external code review tools. It does not contain built-in review logic—it simply executes the specified tool as a subprocess and captures its output.

```bash
mehr review                     # Run CodeRabbit review
mehr review --tool coderabbit   # Explicitly specify tool
mehr review --output review.txt # Save to specific file
```

**Flags:**
| Flag | Description | Default |
|------|-------------|---------|
| `--tool <name>` | Review tool to use (must be installed separately) | `coderabbit` |
| `-o, --output <file>` | Output file name | `REVIEW-N.txt` |

**Supported Tools:**

- **CodeRabbit** (default): Install with `npm install -g coderabbitai`
- Any CLI tool that accepts a `review` subcommand can be used via `--tool`

**Review Status:**

- `COMPLETE` - Review passed with no issues
- `ISSUES` - Review found issues that need attention
- `ERROR` - Review tool failed to run

---

### `mehr undo`

Revert to the previous checkpoint.

```bash
mehr undo
```

---

### `mehr redo`

Restore the next checkpoint (after undo).

```bash
mehr redo
```

---

### `mehr finish`

Complete the task and merge changes to the target branch, or create a pull request.

```bash
mehr finish                      # Complete and merge (with confirmation)
mehr finish --yes                # Skip confirmation prompt
mehr finish --no-push            # Merge but don't push
mehr finish --no-delete          # Keep task branch after merge
mehr finish --no-squash          # Regular merge instead of squash
mehr finish --target develop     # Merge to specific branch
mehr finish --pr                 # Create PR instead of merging
mehr finish --pr --draft         # Create PR as draft
mehr finish --pr --pr-title "Fix bug"  # Custom PR title
```

**Flags:**
| Flag | Description | Default |
|------|-------------|---------|
| `-y, --yes` | Skip confirmation prompt | `false` |
| `--no-push` | Don't push after merge | `false` |
| `--no-delete` | Don't delete task branch | `false` |
| `--no-squash` | Use regular merge instead of squash | `false` |
| `-t, --target <branch>` | Target branch to merge into | from config |
| `--pr` | Create pull request instead of merging locally | `false` |
| `--draft` | Create PR as draft (requires `--pr`) | `false` |
| `--pr-title <title>` | Custom PR title (requires `--pr`) | auto-generated |
| `--pr-body <body>` | Custom PR body (requires `--pr`) | auto-generated |

---

### `mehr delete`

Delete the current task without merging changes.

```bash
mehr delete                 # Delete with confirmation
mehr delete --yes           # Delete without confirmation
mehr delete -y              # Same as --yes
mehr delete --keep-branch   # Delete task but keep the git branch
mehr delete --keep-work     # Delete branch but keep the work directory
```

**Flags:**
| Flag | Description | Default |
|------|-------------|---------|
| `-y, --yes` | Skip confirmation prompt | `false` |
| `--keep-branch` | Keep the git branch | `false` |
| `--keep-work` | Keep the work directory | `false` |

Use this when you want to abandon a task completely.

---

### `mehr yolo <reference>` (alias: `mehr auto`)

Full automation mode: runs the entire workflow without user interaction.

```bash
mehr yolo file:task.md               # Full cycle from file
mehr auto file:task.md               # Same as yolo (alias)
mehr yolo dir:./tasks/               # Full cycle from directory
mehr yolo --max-retries 5 file:task.md  # Allow more quality retries
mehr yolo --skip-quality file:task.md   # Skip quality checks entirely
```

**Workflow:**

1. Register task (creates git branch)
2. Run planning (creates specs, skips agent questions)
3. Implement the specifications
4. Run quality checks (with retry loop if failed)
5. Merge to target branch

**Flags:**
| Flag | Description | Default |
|------|-------------|---------|
| `-a, --agent <name>` | Agent to use | auto-detect |
| `-b, --branch` | Create git branch for the task (use `--branch=false` to disable) | `true` |
| `-w, --worktree` | Create a separate git worktree | `false` |
| `--max-retries <n>` | Maximum quality check retry attempts | `3` |
| `--skip-quality` | Skip quality checks entirely | `false` |
| `--no-push` | Don't push after merge | `false` |
| `--no-delete` | Don't delete task branch after merge | `false` |
| `--no-squash` | Use regular merge instead of squash | `false` |
| `-t, --target <branch>` | Target branch to merge into | auto-detect |
| `--quality-target <target>` | Make target for quality checks | `quality` |

**Quality Retry Loop:**
If quality checks fail, yolo mode automatically re-runs implementation with the quality errors as feedback, up to `--max-retries` times.

---

### `mehr version`

Print version information.

```bash
mehr version
```

---

### Global Flags

These flags are available for all commands:

| Flag            | Description                           | Default |
| --------------- | ------------------------------------- | ------- |
| `-v, --verbose` | Enable verbose output (debug logging) | `false` |
| `--no-color`    | Disable color output                  | `false` |

## Configuration

### Workspace Configuration

Project-level configuration is stored in `.mehrhof/config.yaml`:

```yaml
# Task workspace configuration
git:
  auto_commit: true
  commit_prefix: "[{key}]" # Template: {key}, {task_id}, {type}, {slug}
  branch_pattern: "{type}/{key}--{slug}" # e.g., feature/FEATURE-123--add-auth
  sign_commits: false

agent:
  default: claude
  timeout: 300 # seconds
  max_retries: 3

providers:
  default: file # Allow bare references like "task.md" without scheme prefix

workflow:
  auto_init: true
  session_retention_days: 30

# Environment variables passed to agents
env:
  ANTHROPIC_API_KEY: your-api-key-here
```

### Agent Aliases

Define custom agents that wrap existing agents with specific environment variables:

```yaml
# In .mehrhof/config.yaml
agents:
  glm:
    extends: claude # Base agent to wrap
    description: "Claude with GLM API key" # Shown in 'mehr agents list'
    env:
      ANTHROPIC_API_KEY: "${GLM_API_KEY}" # ${VAR} references system env vars

  glm-fast:
    extends: glm # Aliases can extend other aliases
    description: "GLM with lower token limit"
    env:
      MAX_TOKENS: "2048"
```

Usage:

```bash
# Set environment variable
export GLM_API_KEY="sk-ant-..."

# List available agents
mehr agents list

# Use an alias
mehr start --agent glm file:task.md
```

### User Settings

User-level settings are stored in `~/.mehrhof/settings.json`:

```json
{
  "preferred_agent": "claude",
  "target_branch": "main",
  "last_provider": "file",
  "recent_tasks": ["abc123", "def456"]
}
```

### Environment Variables

Mehrhof uses standard environment variables and CLI flags instead of custom `MEHR_*` variables:

| Variable            | Description                               |
| ------------------- | ----------------------------------------- |
| `ANTHROPIC_API_KEY` | Claude API key (used by Claude CLI)       |
| `GITHUB_TOKEN`      | GitHub API token for issue integration    |
| `MEHR_GITHUB_TOKEN` | Alternative GitHub token (takes priority) |
| `NO_COLOR`          | Disable colored output (any value)        |

For configuration, use `.mehrhof/config.yaml` (see Configuration section above).
For runtime options, use CLI flags: `--verbose`, `--no-color`.

## Workflow

### Typical Task Workflow

```
┌─────────────────────────────────────────────────────────────┐
│                                                             │
│  mehr init  ──►  mehr start  ──►  mehr plan  ──►            │
│                                                             │
│  ──►  mehr implement  ──►  mehr review  ──►  mehr finish    │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

1. **Initialize** (`mehr init`) - Set up the workspace (one-time)
2. **Start** (`mehr start`) - Begin a new task from a file or directory
3. **Plan** (`mehr plan`) - Generate implementation specifications
4. **Implement** (`mehr implement`) - Execute the specifications
5. **Review** (`mehr review`) - Run automated code review
6. **Finish** (`mehr finish`) - Merge changes and clean up

### Recovery Commands

- `mehr continue` - Resume work and see suggested next actions
- `mehr undo` - Revert to previous checkpoint
- `mehr redo` - Restore after undo
- `mehr delete` - Abandon task without merging

## Task Storage

Tasks are stored in `.mehrhof/` directory within your repository:

```
.mehrhof/
├── config.yaml              # Workspace configuration
├── .active_task             # Current active task reference (main repo only)
├── locks/                   # File locks for concurrent access
│   └── <task-id>.lock
├── planned/                 # Standalone planning results (when --new is used)
│   └── <timestamp>/
│       └── specification-1.md
└── work/
    └── <task-id>/
        ├── work.yaml        # Task state and metadata
        ├── specifications/
        │   ├── specification-1.md    # Implementation specifications
        │   └── specification-2.md
        ├── reviews/
        │   └── review-1.txt # Code review results
        └── sessions/
            └── <timestamp>.yaml
```

## Providers

Providers are task sources. You must specify a provider scheme prefix (e.g., `file:`, `dir:`), or configure a default provider.

### File Provider

Reads tasks from markdown files.

```bash
mehr start file:task.md
mehr start file:path/to/task.md
mehr start file:FEATURE-123.md          # Key auto-extracted: FEATURE-123, type: feature
mehr start --key JIRA-456 file:task.md  # Override key via CLI
```

**Frontmatter Support:**

```markdown
---
title: Add user authentication
key: AUTH-001
type: feature
---

# Task Description

...
```

### Directory Provider

Reads tasks from directories with README.md files.

```bash
mehr start dir:./tasks/
mehr start dir:path/to/tasks/
```

### GitHub Provider

Reads tasks from GitHub issues. Supports auto-detection of repository from git remote.

```bash
mehr start github:5                    # Issue #5 from auto-detected repo
mehr start github:owner/repo#123       # Explicit repository
mehr start gh:5                        # Short alias
```

**Features:**

- Auto-detects repository from `git remote origin`
- Fetches issue title, body, labels, and comments
- Extracts linked issues (`#123` references) and images
- Creates branches named `issue/{key}-{slug}` by default
- Commits prefixed with `[#{key}]`

**Token Resolution Priority:**

1. `MEHR_GITHUB_TOKEN` environment variable
2. `GITHUB_TOKEN` environment variable
3. `.mehrhof/config.yaml` `github.token`
4. `gh auth token` (GitHub CLI)

**PR Creation:**

```bash
mehr finish --pr                       # Create PR instead of local merge
mehr finish --pr --draft               # Create as draft PR
mehr finish --pr --pr-title "Fix bug"  # Custom PR title
```

**Configuration:**

```yaml
# .mehrhof/config.yaml
github:
  branch_pattern: "issue/{key}-{slug}"
  commit_prefix: "[#{key}]"
  target_branch: "main"
  draft_pr: false
  comments:
    enabled: false # Auto-post comments to issues
    on_branch_created: true # Post when branch is created
    on_plan_done: true # Post implementation plan summary
    on_implement_done: true # Post changelog with files changed
    on_pr_created: true # Post PR link
```

## Plugins

Mehrhof supports plugins for extending functionality without recompilation. Plugins communicate via JSON-RPC 2.0 over stdin/stdout.

### Plugin Types

| Type         | Purpose                  | Examples                       | Status |
| ------------ | ------------------------ | ------------------------------ | ------ |
| **Provider** | Custom task sources      | Jira, YouTrack, Linear, Notion | Stable |
| **Agent**    | Custom AI backends       | Local LLMs, Codex              | Stable |
| **Workflow** | State machine extensions | Approval steps, notifications  | Stable |

Workflow plugins support dynamic phase insertion (`after`/`before`), custom guards, and critical effects that can block workflow on failure.

### Using Plugins

```bash
# List available plugins (global and project-local)
mehr plugins list

# Install a plugin from git repository
mehr plugins install https://github.com/example/my-plugin.git

# Install a plugin from local path
mehr plugins install ./path/to/plugin

# Remove a plugin
mehr plugins remove my-plugin

# Validate a plugin (checks manifest and connectivity)
mehr plugins validate my-plugin

# Show detailed plugin information
mehr plugins info my-plugin
```

**Plugin Commands:**
| Command | Description |
|---------|-------------|
| `mehr plugins list` | List all discovered plugins with status |
| `mehr plugins install <source>` | Install from git URL or local path |
| `mehr plugins remove <name>` | Remove an installed plugin |
| `mehr plugins validate [name]` | Validate manifest and test connectivity |
| `mehr plugins info <name>` | Show detailed plugin information |

### Enabling Plugins

Add to `.mehrhof/config.yaml`:

```yaml
plugins:
  enabled:
    - jira
    - slack-notify
```

### Plugin Locations

```
~/.mehrhof/plugins/     # Global (all projects)
.mehrhof/plugins/       # Project-local (overrides global)
```

### Creating Plugins

For plugin development documentation, see `.mehrhof/plugins/docs/`:

| Guide                 | Description                  |
| --------------------- | ---------------------------- |
| `README.md`           | Getting started              |
| `provider-plugins.md` | Create task source providers |
| `agent-plugins.md`    | Create AI agent backends     |
| `workflow-plugins.md` | Extend the state machine     |
| `protocol.md`         | JSON-RPC protocol reference  |

## Architecture

```
cmd/mehr/           # CLI entry point
internal/
├── agent/          # AI agent abstraction
│   └── claude/     # Claude agent implementation
├── conductor/      # Main orchestrator
├── config/         # Configuration loading
├── events/         # Event bus pub/sub
├── log/            # Structured logging
├── plugin/         # Plugin system (JSON-RPC providers, agents, workflows)
├── provider/       # Task source providers
│   ├── file/       # File provider
│   ├── directory/  # Directory provider
│   └── github/     # GitHub issues provider
├── storage/        # YAML-based persistence
├── vcs/            # Git operations
└── workflow/       # State machine engine
```

## Development

### Build Commands

```bash
# Build the binary
make build

# Run tests
make test

# Run tests with coverage
make coverage

# Run linter
make lint

# Format code
make fmt

# Install locally
make install

# Clean build artifacts
make clean

# Tidy dependencies
make tidy
```

### Running Locally

```bash
# Build and run
make run

# Run with arguments
make run-args ARGS="start task.md"

# Show version info
make version
```

## Troubleshooting

### Common Issues

**"no active task"**

- Run `mehr start <file>` to begin a new task
- Or run `mehr continue` to see suggestions

**"ANTHROPIC_API_KEY not set"**

- Export your Claude API key: `export ANTHROPIC_API_KEY=your-key`
- Or add it to `.mehrhof/config.yaml` under `env:`

**"review tool not found"**

- Install CodeRabbit: `npm install -g coderabbitai`

**"workspace not initialized"**

- Run `mehr init` to create the `.mehrhof` directory

### Getting Help

```bash
# Show help for any command
mehr --help
mehr <command> --help
```

## License

Internal use only.
