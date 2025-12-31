# Troubleshooting

Solutions for common issues with Mehrhof.

## Categories

- [Installation Issues](installation.md) - Setup and build problems
- [Claude CLI Issues](claude-cli.md) - Agent-related problems
- [Task Issues](tasks.md) - Task management and workflow
- [Git Issues](git.md) - Repository and branch problems
- [Diagnostics](diagnostics.md) - Debugging and getting help

## Quick Fixes

| Problem            | Quick Fix                                          |
| ------------------ | -------------------------------------------------- |
| Command not found  | `export PATH="$PATH:$(go env GOPATH)/bin"`         |
| Claude not working | Ensure Claude CLI is installed: `claude --version` |
| No active task     | `mehr start file:task.md`                          |
| Bad implementation | `mehr undo`                                        |
| Merge conflict     | Resolve manually, `git add .`, `git commit`        |
| Timeout            | Increase `agent.timeout` in `.mehrhof/config.yaml` |
| Start fresh        | `mehr abandon --yes && mehr start file:task.md`     |
