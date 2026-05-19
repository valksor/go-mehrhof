# kvelmo show

Display task artifacts (specs, plans).

## Usage

```bash
kvelmo show <subcommand>
```

## Subcommands

| Subcommand | Description                               |
| ---------- | ----------------------------------------- |
| `spec`     | Show specification files for current task |
| `plan`     | Show the plan (alias for `show spec`)     |

## Options

| Flag     | Description              |
| -------- | ------------------------ |
| `--json` | Output raw JSON response |

## Examples

```bash
# Show specification
kvelmo show spec

# Show plan
kvelmo show plan

# JSON output
kvelmo show spec --json
```

## Output

```markdown
# .kvelmo/specifications/task-001.md

## Overview

Add user authentication to the API endpoints...

## Implementation Steps

1. Create JWT middleware
2. Add login/logout endpoints
   ...
```

## Related

- [plan](/cli/plan.md) — Generate implementation specification
- [status](/cli/status.md) — Show current task state
