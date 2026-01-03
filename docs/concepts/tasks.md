# Tasks

A task represents a unit of work to be completed with AI assistance. Tasks have sources, specifications, and a lifecycle.

## What is a Task?

A task consists of:

- **Source** - The original requirements (markdown file, directory)
- **Specifications** - AI-generated implementation plans
- **Work directory** - Storage for all task artifacts
- **Git branch** - Isolated workspace for changes
- **State** - Current position in the workflow

## Task Sources

### File Source

A single markdown file describing requirements:

```bash
mehr start task.md
```

Example file:

```markdown
# Add Search Feature

Implement full-text search for the product catalog.

## Requirements

- Search by product name and description
- Support fuzzy matching
- Return top 10 results by relevance

## Constraints

- Use existing database (PostgreSQL)
- Response time < 200ms
```

### Task File Frontmatter

Task files support YAML frontmatter for metadata:

```yaml
---
title: Add Search Feature
priority: high
labels:
  - feature
  - backend
key: FEATURE-123
type: feature
agent: glm
agent_env:
  MAX_TOKENS: "8192"
---
# Add Search Feature
...
```

| Field       | Description                                       |
| ----------- | ------------------------------------------------- |
| `title`     | Task title (overrides `# Heading`)                |
| `priority`  | Priority: `critical`, `high`, `normal`, `low`     |
| `labels`    | Array of labels/tags                              |
| `key`       | External key for branch naming (e.g., `FEAT-123`) |
| `type`      | Task type: `feature`, `fix`, `chore`, etc.        |
| `agent`     | Agent name or alias to use for this task          |
| `agent_env` | Inline environment variables for the agent        |

See [AI Agents](../agents/index.md#per-task-agent-configuration) for details on `agent` and `agent_env`.

### Directory Source

A directory containing multiple related files:

```bash
mehr start ./tasks/auth-feature/
```

Directory structure:

```
auth-feature/
├── requirements.md
├── api-spec.yaml
└── mockups/
    └── login-page.png
```

All files are read and provided as context to the AI.

## Task Identification

Each task gets a unique 8-character ID:

```
Task ID: a1b2c3d4
Branch:  task/a1b2c3d4
Work:    .mehrhof/work/a1b2c3d4/
```

## Specifications

Specifications are detailed implementation plans created during planning:

```
.mehrhof/work/<id>/specifications/
├── specification-1.md    # First specification
├── specification-2.md    # Second specification
└── specification-3.md    # Third specification
```

Each specification file contains:

- Implementation details
- File changes required
- Step-by-step instructions

See [SPEC File Format](../reference/spec-format.md) for details.

## Task States

| State        | Meaning                 |
| ------------ | ----------------------- |
| idle         | Ready for commands      |
| planning     | Creating specifications |
| implementing | Generating code         |
| reviewing    | Running code review     |
| done         | Completed and merged    |
| failed       | Error occurred          |

## Working with Multiple Tasks

### List All Tasks

```bash
mehr status --all
```

Output:

```
Active: a1b2c3d4 (idle) - Add search feature
        b5c6d7e8 (implementing) - Auth system
        c9d0e1f2 (done) - Bug fix #123
```

### Switch Tasks

Tasks live on git branches. Switch with:

```bash
git checkout task/b5c6d7e8
```

### Use Worktrees

For complete isolation, use git worktrees:

```bash
mehr start --worktree task.md
```

This creates a separate working directory:

```
../your-project-task-a1b2c3d4/
```

## Task Notes

Add context during development:

```bash
mehr note "Use the existing UserService instead of creating a new one"
```

Notes are saved to `.mehrhof/work/<id>/notes.md` and included in future AI prompts.

## Task Lifecycle Example

```bash
# 1. Start a task
mehr start feature.md
# Creates: task/abc12345 branch, .mehrhof/work/abc12345/

# 2. Plan the implementation
mehr plan
# Creates: .mehrhof/work/abc12345/specifications/specification-1.md

# 3. Add clarification
mehr note "The API should be REST, not GraphQL"
# Updates: .mehrhof/work/abc12345/notes.md

# 4. Implement
mehr implement
# Generates code, creates checkpoint

# 5. Undo if needed
mehr undo
# Reverts to previous checkpoint

# 6. Finish
mehr finish
# Merges to main, deletes branch
```

## Abandoning Tasks

Delete a task without merging:

```bash
mehr abandon
```

Options:

- `--yes` - Skip confirmation
- `--keep-branch` - Only delete work directory
- `--keep-work` - Only delete branch

## Task Storage

All task data is stored in `.mehrhof/work/<id>/`:

```
.mehrhof/work/abc12345/
├── work.yaml           # Task metadata
├── notes.md            # User notes
├── specifications/              # Specifications
│   └── specification-1.md
├── reviews/            # Code reviews
│   └── REVIEW-1.txt
└── sessions/           # Agent logs
    └── 2025-01-15T10-30-00-planning.yaml
```

See [Storage Structure](../reference/storage.md) for complete details.
