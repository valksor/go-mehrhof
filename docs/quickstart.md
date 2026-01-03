# Quickstart

Get started with Valksor Mehrhof in 5 minutes.

## Prerequisites

- **Git** - for version control integration
- **Claude CLI** - Mehrhof delegates AI operations to Claude ([setup guide](https://claude.com/product/claude-code))

```bash
claude --version
```

## Install

### Option 1: Pre-built Binary (Recommended)

| Platform | Architecture | Binary |
|----------|--------------|--------|
| Linux | AMD64 | `mehr-linux-amd64` |
| Linux | ARM64 | `mehr-linux-arm64` |
| macOS | Intel | `mehr-darwin-amd64` |
| macOS | Apple Silicon | `mehr-darwin-arm64` |

> **Windows Users:** Use [WSL2](https://learn.microsoft.com/en-us/windows/wsl/) with the Linux binary (`mehr-linux-amd64` or `mehr-linux-arm64`).

```bash
curl -L https://github.com/valksor/go-mehrhof/releases/latest/download/mehr-darwin-arm64 -o mehr
chmod +x mehr
sudo mv mehr /usr/local/bin/

mehr version
```

### Option 2: Nightly Build

Get the latest development build (use with caution):

```bash
curl -L https://github.com/valksor/go-mehrhof/releases/download/nightly/mehr-darwin-arm64 -o mehr
chmod +x mehr
sudo mv mehr /usr/local/bin/
```

### Option 3: Build from Source

Requires Go 1.25+:

```bash
git clone https://github.com/valksor/go-mehrhof.git
cd go-mehrhof
make install
mehr version
```

## Your First Task

### 1. Initialize Your Project

```bash
cd your-project
mehr init
```

This creates `.mehrhof/` for task storage.

### 2. Create a Task File

```bash
cat > feature.md << 'EOF'
---
title: Add Health Check Endpoint
---

Create a `/health` endpoint that returns:
- HTTP 200 when healthy
- JSON with status and timestamp
- No authentication required
EOF
```

### 3. Start the Task

```bash
mehr start feature.md
```

Output:
```
Task registered: a1b2c3d4
Branch created: task/a1b2c3d4--add-health-check-endpoint
Switched to branch task/a1b2c3d4--add-health-check-endpoint
```

### 4. Generate & Implement

```bash
mehr plan

mehr implement
```

### 5. Review & Finish

```bash
git diff

mehr finish
```

## Common Commands

| Command | Description |
|---------|-------------|
| `mehr start <file>` | Start a task from markdown file |
| `mehr auto <file>` | Full automation (plan + implement + finish) |
| `mehr plan` | Generate AI implementation specifications |
| `mehr implement` | Execute the specifications |
| `mehr status` | Show current task status |
| `mehr continue` | Resume work with suggested actions |
| `mehr undo` / `mehr redo` | Navigate checkpoints |
| `mehr note "..."` | Add context for the AI |
| `mehr finish` | Complete and merge |
| `mehr abandon` | Discard task without merging |

## Updating

```bash
mehr update
mehr update --check
```

## Next Steps

- [Your First Task (detailed)](guides/first-task.md) - Step-by-step tutorial
- [Workflow Concepts](concepts/workflow.md) - Understanding the task lifecycle
- [CLI Reference](cli/index.md) - All commands and flags
- [Configuration](configuration/index.md) - Customize behavior
- [Task Providers](providers/index.md) - Load tasks from GitHub, Jira, Linear, etc.
