# mehr templates

Manage and apply task templates for common development patterns.

## Usage

```bash
mehr templates                    # List available templates
mehr templates show <name>        # Show template details
mehr templates apply <name> <file> # Apply template to a task file
```

## Description

Templates provide pre-configured frontmatter, agent selection, and workflow settings for common task types. Using templates ensures consistency across similar tasks and reduces repetitive configuration.

Templates can be applied:
1. **Via CLI** when starting a task with `--template` flag
2. **Via `templates apply`** to existing files
3. **Manually** by copying the frontmatter to any task file

## Available Templates

| Template    | Description                                    |
| ----------- | ---------------------------------------------- |
| `bug-fix`   | Bug fix tasks with stricter validation         |
| `feature`   | New feature development                        |
| `refactor`  | Code refactoring (quality-focused)             |
| `docs`      | Documentation changes (skips quality checks)   |
| `test`      | Adding or improving tests                      |
| `chore`     | Maintenance tasks and chores                   |

## Commands

### List Templates

```bash
$ mehr templates

Available templates:

  bug-fix     Template for bug fix tasks with stricter validation
  feature     Template for new feature development
  refactor    Template for code refactoring tasks
  docs        Template for documentation changes
  test        Template for test-related tasks
  chore       Template for maintenance tasks

Usage:
  mehr templates show <name>              Show template details
  mehr templates apply <name> <file>      Apply template to file
  mehr start --template <name> file:task.md
```

### Show Template Details

```bash
$ mehr templates show bug-fix

Template: bug-fix
Description: Template for bug fix tasks with stricter validation

Frontmatter:
  type: fix

Agent:
  Default: claude-sonnet

Git:
  branch_pattern: fix/{key}--{slug}
  commit_prefix: "[fix/{key}]"

Workflow:
  skip_quality: false

Example usage:
  mehr templates apply bug-fix my-task.md
  mehr start --template bug-fix file:my-task.md
```

### Apply Template

```bash
# Apply to existing file
mehr templates apply bug-fix task.md

# Output
Applied template 'bug-fix' to task.md

Frontmatter added:
  type: fix
  agent: claude-sonnet
```

## Using with `mehr start`

Apply a template when starting a new task:

```bash
mehr start --template bug-fix file:task.md
mehr start --template feature file:FEATURE-123.md
mehr start -t refactor file:cleanup.md
```

The template is applied **before** the task is registered, so the template's frontmatter will be used for branch naming, agent selection, and workflow configuration.

## Template Reference

### bug-fix

For bug fix tasks requiring stricter validation.

```yaml
type: fix
agent: claude-sonnet
git:
  branch_pattern: "fix/{key}--{slug}"
  commit_prefix: "[fix/{key}]"
workflow:
  skip_quality: false
```

### feature

For new feature development.

```yaml
type: feature
agent: claude
git:
  branch_pattern: "feature/{key}--{slug}"
  commit_prefix: "[{key}]"
workflow:
  skip_quality: false
```

### refactor

For code refactoring and quality improvements.

```yaml
type: refactor
agent: claude-sonnet
git:
  branch_pattern: "refactor/{key}--{slug}"
  commit_prefix: "[refactor]"
workflow:
  skip_quality: false
```

### docs

For documentation changes (quality checks skipped).

```yaml
type: docs
agent: claude-haiku
git:
  branch_pattern: "docs/{key}--{slug}"
  commit_prefix: "[docs]"
workflow:
  skip_quality: true
```

### test

For adding or improving tests.

```yaml
type: test
agent: claude-sonnet
git:
  branch_pattern: "test/{key}--{slug}"
  commit_prefix: "[test]"
workflow:
  skip_quality: false
```

### chore

For maintenance tasks and chores.

```yaml
type: chore
agent: claude-sonnet
git:
  branch_pattern: "chore/{key}--{slug}"
  commit_prefix: "[chore]"
workflow:
  skip_quality: true
```

## Creating Custom Templates

You can create custom templates in `.mehrhof/templates/`:

```bash
# Create custom template directory
mkdir -p .mehrhof/templates

# Create a custom template
cat > .mehrhof/templates/security.yaml <<'EOF'
name: security
description: Template for security fixes
frontmatter:
  type: security
  priority: critical
agent: claude
git:
  branch_pattern: "security/{key}--{slug}"
  commit_prefix: "[security/{key}]"
workflow:
  skip_quality: false
EOF
```

Custom templates are loaded alongside built-in templates.

## Template Merging Behavior

When applying a template to a file with existing frontmatter:
- Template values **take precedence** over existing values
- Existing values not in template are preserved
- The merge is shallow (nested maps are replaced, not merged)

Example:

**Before:**
```yaml
---
title: Fix login bug
key: BUG-123
---
```

**After applying `bug-fix` template:**
```yaml
---
title: Fix login bug
key: BUG-123
type: fix
agent: claude-sonnet
---
```
