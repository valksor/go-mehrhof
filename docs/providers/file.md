# File Provider

> **⚠️ Experimental**: This provider is not fully tested beyond unit tests. Edge cases may exist. Manual validation recommended before production use.

**Schemes:** `file:`

**Capabilities:** `read`

Reads tasks from local markdown files.

## Usage

```bash
mehr start file:task.md
mehr plan file:features/user-auth.md
```

## File Format

```markdown
---
title: Add User Authentication
agent: claude
---

Implement JWT-based authentication with login/logout endpoints.
```

The file provider extracts metadata from YAML frontmatter and uses the remaining content as the task description.
