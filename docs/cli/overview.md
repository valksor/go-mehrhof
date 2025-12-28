# CLI Overview

Mehrhof provides a command-line interface for AI-powered task automation.

## Basic Usage

```bash
mehr <command> [flags]
```

## Global Flags

These flags work with any command:

| Flag         | Short | Description            |
| ------------ | ----- | ---------------------- |
| `--verbose`  | `-v`  | Enable verbose output  |
| `--no-color` |       | Disable colored output |

## Commands

### Task Management

| Command                     | Description                                |
| --------------------------- | ------------------------------------------ |
| [start](cli/start.md)       | Register a new task from file or directory |
| [status](cli/status.md)     | Show task status                           |
| [continue](cli/continue.md) | Show status and suggested next actions     |
| [delete](cli/delete.md)     | Delete task without merging                |

### Workflow

| Command                       | Description                                        |
| ----------------------------- | -------------------------------------------------- |
| [plan](cli/plan.md)           | Create implementation specifications               |
| [implement](cli/implement.md) | Implement the specifications                       |
| [review](cli/review.md)       | Run code review                                    |
| [talk](cli/talk.md)           | Add notes or discuss with agent                    |
| [finish](cli/finish.md)       | Complete task and merge                            |
| [yolo](cli/yolo.md)           | Full automation: start → plan → implement → finish |

### History

| Command                       | Description                   |
| ----------------------------- | ----------------------------- |
| [undo](cli/undo-redo.md)      | Revert to previous checkpoint |
| [redo](cli/undo-redo.md#redo) | Restore undone checkpoint     |

### Utility

| Command                   | Description                  |
| ------------------------- | ---------------------------- |
| [init](cli/init.md)       | Initialize task workspace    |
| [agents](cli/agents.md)   | List available AI agents     |
| [config](cli/config.md)   | Validate configuration files |
| [plugins](cli/plugins.md) | Manage extension plugins     |
| [version](cli/version.md) | Print version information    |

## Command Help

Get help for any command:

```bash
mehr --help
mehr <command> --help
```

## Workflow Summary

```bash
# Typical workflow
mehr start file:task.md   # Register task
mehr plan                 # Create specs
mehr implement            # Generate code
mehr finish               # Merge changes
```

## Output Formats

By default, output is human-readable text. For scripting:

```bash
# JSON output (if supported)
export MEHR_UI_FORMAT=json
```

## Exit Codes

| Code | Meaning           |
| ---- | ----------------- |
| 0    | Success           |
| 1    | General error     |
| 2    | Invalid arguments |

## Environment Variables

See [Environment Variables](../configuration/environment.md) for configuration options.

## Examples

### Quick Task (Yolo Mode)

```bash
mehr yolo file:feature.md
```

### Quick Task (Manual)

```bash
mehr start file:feature.md && mehr plan && mehr implement && mehr finish
```

### Interactive Development

```bash
mehr start file:feature.md
mehr plan
# Review specs...
mehr talk "Add error handling for edge cases"
mehr implement
# Test changes...
mehr finish
```

### Exploring Without a Task

```bash
mehr plan --new "How should I implement caching?"
```
