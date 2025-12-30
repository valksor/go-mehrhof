# Storage Structure

Complete reference for the `.mehrhof/` directory structure.

## Overview

All task data is stored in the `.mehrhof/` directory:

```
.mehrhof/
├── config.yaml              # Workspace configuration
├── .active_task             # Current active task reference
├── work/                    # Task work directories
│   └── <task-id>/
│       ├── work.yaml        # Task metadata
│       ├── notes.md         # User notes
│       ├── source/          # Source files (task content)
│       ├── specifications/  # Specifications
│       ├── reviews/         # Code reviews
│       └── sessions/        # Agent conversation logs
└── planned/                 # Standalone planning sessions
    └── <plan-id>/
```

## Root Files

### config.yaml

Workspace-level configuration:

```yaml
git:
  auto_commit: true
  commit_prefix: "[task]"
  branch_pattern: task/{task_id}

agent:
  default: claude
  timeout: 300

workflow:
  session_retention_days: 30
```

### .active_task

Currently active task reference (YAML):

```yaml
id: cb9a54db
ref: file:task.md
work_dir: .mehrhof/work/cb9a54db
state: idle
branch: task/cb9a54db
use_git: true
worktree_path: null
started: 2025-01-15T10:30:00Z
```

**Fields:**

| Field           | Description                        |
| --------------- | ---------------------------------- |
| `id`            | 8-character task identifier        |
| `ref`           | Source reference (file:, dir:)     |
| `work_dir`      | Path to work directory             |
| `state`         | Current workflow state             |
| `branch`        | Git branch name                    |
| `use_git`       | Whether git operations are enabled |
| `worktree_path` | Path if using git worktree         |
| `started`       | Task start timestamp               |

## Work Directory

Each task has a work directory at `.mehrhof/work/<task-id>/`.

### work.yaml

Task metadata and source information:

```yaml
version: "1"
metadata:
  id: cb9a54db
  title: Add Health Endpoint
  created_at: 2025-01-15T10:30:00Z
  updated_at: 2025-01-15T11:45:00Z
source:
  type: file
  ref: task.md
  read_at: 2025-01-15T10:30:00Z
  files:
    - source/task.md
git:
  branch: task/cb9a54db
  base_branch: main
  created_at: 2025-01-15T10:30:00Z
```

**Sections:**

#### metadata

| Field        | Description              |
| ------------ | ------------------------ |
| `id`         | Task identifier          |
| `title`      | Task title (from source) |
| `created_at` | Creation timestamp       |
| `updated_at` | Last modification        |

#### source

| Field     | Description                        |
| --------- | ---------------------------------- |
| `type`    | Source type (file, directory, github, jira, etc.) |
| `ref`     | Original reference                 |
| `read_at` | When source was read               |
| `files`   | Paths to source files in `source/` directory |

The `source/` directory contains the actual source files:

```
source/
├── task.md           # Main task content (file provider)
├── issue.md           # Issue content (GitHub/Jira/Linear)
├── comments.md        # Comments (GitHub)
├── notes.md           # Notes (GitLab)
└── linked/            # Linked issues (GitHub)
    └── issue-123.md
```

#### git

| Field         | Description          |
| ------------- | -------------------- |
| `branch`      | Task branch name     |
| `base_branch` | Branch created from  |
| `created_at`  | Branch creation time |

### notes.md

User notes accumulated through `mehr note`:

```markdown
# Notes

## 2025-01-15 10:45:00 [idle]

Use the existing HTTP router, don't create a new one.

## 2025-01-15 11:00:00 [idle]

The handler should return JSON, not plain text.
Make sure to set Content-Type header.

## 2025-01-15 11:30:00 [waiting]

**Q:** Should we use dependency injection?
**A:** Yes, use dependency injection for the version service.
```

Each entry includes:

- Timestamp
- State when note was added
- User's note (or Q&A format when answering agent questions)

### specifications/ Directory

Implementation specifications:

```
specifications/
├── specification-1.md
├── specification-2.md
└── specification-3.md
```

See [Specification File Format](reference/spec-format.md) for details.

### reviews/ Directory

Code review results:

```
reviews/
├── review-1.txt
├── review-2.txt
└── security-review.txt
```

Files are plain text with review findings.

### sessions/ Directory

Agent conversation logs:

```
sessions/
├── 2025-01-15T10-30-00-planning.yaml
├── 2025-01-15T11-00-00-implementing.yaml
└── 2025-01-15T11-30-00-reviewing.yaml
```

**Filename format:** `<timestamp>-<type>.yaml`

**Session file structure:**

```yaml
version: "1"
kind: Session
metadata:
  started_at: 2025-01-15T10:30:00Z
  ended_at: 2025-01-15T10:35:00Z
  type: planning
  agent: claude
  state: idle
usage:
  input_tokens: 1200
  output_tokens: 3500
  cached_tokens: 0
  cost_usd: 0.045
exchanges:
  - role: user
    timestamp: 2025-01-15T10:30:00Z
    content: "[prompt content]"
  - role: agent
    timestamp: 2025-01-15T10:30:15Z
    content: "[response content]"
    files_changed:
      - path: internal/api/health.go
        operation: create
```

**Exchange roles:**

- `user` - User prompt or system prompt
- `agent` - Agent response
- `system` - System message

**File operations:**

- `create` - New file created
- `update` - Existing file modified
- `delete` - File removed

## Planned Directory

Standalone planning sessions (from `mehr plan --new`):

```
planned/
└── xyz789ab/
    ├── plan.yaml
    └── PLAN_HISTORY.md
```

### plan.yaml

Planning session metadata:

```yaml
version: "1"
metadata:
  id: xyz789ab
  topic: "Design authentication system"
  created_at: 2025-01-15T10:30:00Z
  agent: claude
```

### PLAN_HISTORY.md

Conversation history in markdown:

```markdown
# Planning Session

Topic: Design authentication system

## User

How should I implement user authentication?

## Agent

For user authentication, I recommend...

## User

What about JWT tokens?

## Agent

JWT tokens are a good choice...
```

## File Ownership

| File/Directory       | Managed By | Editable    |
| -------------------- | ---------- | ----------- |
| config.yaml          | User       | Yes         |
| .active_task         | Mehrhof    | No          |
| work.yaml            | Mehrhof    | No          |
| source/              | Mehrhof    | Read-only   |
| notes.md             | User       | Yes         |
| specifications/\*.md | Mehrhof    | Read-only\* |
| reviews/\*.txt       | Mehrhof    | Read-only   |
| sessions/\*.yaml     | Mehrhof    | No          |

\*Specification files can be manually edited, but changes may be overwritten by `mehr plan`.

## Gitignore Recommendations

Add to `.gitignore`:

```gitignore
# Mehrhof task data
.mehrhof/work/
.mehrhof/planned/
.mehrhof/.active_task
```

Keep tracked:

```
.mehrhof/config.yaml
```

## Backup and Recovery

### Backup Task Data

```bash
cp -r .mehrhof/work/abc12345 ~/backup/
```

### Recovery

1. Restore work directory
2. Update `.active_task` manually
3. Checkout task branch

## Cleanup

### Remove Old Sessions

Sessions older than `session_retention_days` can be cleaned:

```bash
find .mehrhof/work/*/sessions/ -mtime +30 -delete
```

### Remove Completed Tasks

After `mehr finish`, work directories are removed automatically. For manual cleanup:

```bash
rm -rf .mehrhof/work/abc12345/
```
