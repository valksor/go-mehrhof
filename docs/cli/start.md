# mehr start

Register a new task from a file or directory.

## Synopsis

```bash
mehr start <scheme:reference> [flags]
```

## Description

The `start` command registers a new task and prepares the workspace. It:

1. Generates a unique task ID
2. Creates a git branch (unless `--branch=false`)
3. Copies source content to the work directory
4. Sets the task as active

This command does **not** run planning. Use `mehr plan` for that.

## Provider Schemes

**Important:** You must specify a provider scheme prefix (e.g., `file:`, `dir:`, `github:`):

```bash
mehr start file:task.md          # From a markdown file
mehr start dir:./tasks/          # From a directory
mehr start github:5              # From GitHub issue #5
mehr start github:owner/repo#123 # From explicit repo/issue
```

Alternatively, configure a default provider in `.mehrhof/config.yaml`:

```yaml
providers:
  default: file # Bare references like "task.md" will use file: provider
```

Then you can omit the scheme:

```bash
mehr start task.md   # Uses configured default provider
```

## Arguments

| Argument           | Description                                                     |
| ------------------ | --------------------------------------------------------------- |
| `scheme:reference` | Provider scheme and path (e.g., `file:task.md`, `dir:./tasks/`) |

## Flags

| Flag                   | Short | Type   | Default                | Description                                           |
| ---------------------- | ----- | ------ | ---------------------- | ----------------------------------------------------- |
| `--agent`              | `-a`  | string | auto                   | Agent to use (overrides all steps)                    |
| `--agent-planning`     |       | string |                        | Agent for planning step                               |
| `--agent-implementing` |       | string |                        | Agent for implementation step                         |
| `--agent-reviewing`    |       | string |                        | Agent for review step                                 |
| `--agent-dialogue`     |       | string |                        | Agent for dialogue/talk step                          |
| `--branch`             | `-b`  | bool   | true                   | Create a git branch (use `--branch=false` to disable) |
| `--worktree`           | `-w`  | bool   | false                  | Create a separate git worktree                        |
| `--key`                | `-k`  | string | auto                   | External key for branch/commit naming                 |
| `--commit-prefix`      |       | string | `[{key}]`              | Commit prefix template                                |
| `--branch-pattern`     |       | string | `{type}/{key}--{slug}` | Branch pattern template                               |

### Naming Template Variables

| Variable    | Description                                | Example                  |
| ----------- | ------------------------------------------ | ------------------------ |
| `{key}`     | External key from filename/frontmatter/CLI | `FEATURE-123`            |
| `{task_id}` | Internal task ID                           | `a1b2c3d4`               |
| `{type}`    | Task type from filename prefix             | `feature`, `fix`, `task` |
| `{slug}`    | URL-safe slugified title                   | `add-user-auth`          |

**Key resolution priority:** CLI `--key` > frontmatter `key:` > filename > task ID

## Examples

### Start from a File

```bash
mehr start file:task.md
```

Output:

```
Task registered: a1b2c3d4
  Title: Add user authentication
  Key: task
  Branch: task/task--add-user-auth
```

### Start with Ticket ID (Auto-Extracted)

```bash
mehr start file:FEATURE-123.md
```

The key and type are extracted from the filename:

```
Task registered: a1b2c3d4
  Title: Add user authentication
  Key: FEATURE-123
  Branch: feature/FEATURE-123--add-user-auth
```

### Override Key via CLI

```bash
mehr start --key JIRA-456 file:task.md
```

```
Task registered: a1b2c3d4
  Key: JIRA-456
  Branch: task/JIRA-456--add-user-auth
```

### Start from a Directory

```bash
mehr start dir:./tasks/feature/
```

All files in the directory become part of the task context.

### Start from GitHub Issue

```bash
mehr start github:42                    # Issue #42 from detected repo
mehr start github:owner/repo#123        # Explicit repository
mehr start gh:42                        # Short alias
```

Repository is auto-detected from `git remote origin`. Requires `GITHUB_TOKEN` or `MEHR_GITHUB_TOKEN`.

```
Task registered: a1b2c3d4
  Title: Fix authentication bug
  Key: 42
  Type: fix (from labels)
  Branch: issue/42--fix-authentication-bug
```

**Features:**

- Fetches issue title, body, labels, and comments
- Extracts linked issues (`#123` references)
- Infers task type from labels (`bug` → `fix`, `enhancement` → `feature`)
- Default branch pattern: `issue/{key}-{slug}`

### Start Without Branch

```bash
mehr start --branch=false file:task.md
```

Work happens on the current branch. Useful for quick experiments.

### Start with Worktree (Parallel Tasks)

```bash
mehr start --worktree file:task.md
```

Creates a separate working directory for complete isolation:

```
Task registered: a1b2c3d4
Worktree: ../project-worktrees/a1b2c3d4
```

**Why use worktrees?**

- Run multiple tasks simultaneously in separate terminals
- Each worktree has its own branch - no switching needed
- Commands auto-detect which task based on your current directory

**Parallel workflow:**

```bash
# Terminal 1
mehr start --worktree file:feature-a.md
cd ../project-worktrees/a1b2c3d4
mehr plan && mehr implement

# Terminal 2 (from main repo)
mehr start --worktree file:feature-b.md
cd ../project-worktrees/e5f6g7h8
mehr plan && mehr implement

# See all tasks
mehr list
```

**Note:** New tasks must be started from the main repository, not from within a worktree.

### Specify Agent

```bash
mehr start --agent claude file:task.md
```

### Specify Per-Step Agents

Use different agents for different workflow phases:

```bash
# Use Opus for planning, Sonnet for implementation
mehr start --agent-planning claude-opus --agent-implementing claude-sonnet file:task.md

# Override just the planning agent
mehr start --agent-planning glm file:task.md
```

See [Agents - Per-Step Configuration](../concepts/agents.md#per-step-agent-configuration) for details.

## Task File Format

Task files are markdown with optional YAML frontmatter:

```markdown
---
title: Add user authentication
key: AUTH-001
type: feature
---

# Task Title

Brief description of what needs to be done.

## Requirements

- Requirement 1
- Requirement 2

## Constraints

- Use existing patterns
- Maintain backward compatibility
```

### Frontmatter Fields

| Field   | Description                  | Example                   |
| ------- | ---------------------------- | ------------------------- |
| `title` | Task title (overrides H1)    | `Add user authentication` |
| `key`   | External key for naming      | `AUTH-001`, `JIRA-123`    |
| `type`  | Task type for branch pattern | `feature`, `fix`, `docs`  |

## What Happens

1. **ID Generation** - 8-character unique identifier

2. **Naming Resolution**
   - External key resolved from CLI > frontmatter > filename
   - Type extracted from filename prefix (e.g., `FEATURE-` → `feature`)
   - Slug generated from title

3. **Branch Creation** (unless `--branch=false`)
   - Branch name: `{type}/{key}--{slug}` (e.g., `feature/FEATURE-123--add-auth`)
   - Base branch: current HEAD

4. **Work Directory**
   - Created at `.mehrhof/work/<id>/`
   - Source content copied (read-only)
   - `work.yaml` metadata file created

5. **Git Operations**
   - New branch created
   - Initial commit (optional)
   - Switched to task branch

## After Starting

Check your task:

```bash
mehr status
```

Continue to planning:

```bash
mehr plan
```

## See Also

- [plan](cli/plan.md) - Create specifications
- [status](cli/status.md) - View task status
- [Tasks Concept](../concepts/tasks.md) - Understanding tasks
