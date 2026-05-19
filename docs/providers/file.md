# File Provider

Load tasks from local markdown files.

## Usage

```bash
kvelmo start --from file:task.md
kvelmo start --from file:path/to/task.md
```

## File Format

Task files are markdown with optional YAML frontmatter:

```markdown
---
title: Add user authentication
---

Add login and signup pages with JWT tokens.

## Requirements

- Email/password authentication
- JWT token generation
- Secure password hashing

## Acceptance Criteria

- Users can register with email/password
- Users can login and receive a JWT
- JWT is validated on protected routes
```

## Frontmatter Fields

| Field    | Description        | Required    |
| -------- | ------------------ | ----------- |
| `title`  | Task title         | Recommended |
| `agent`  | Agent to use       | Optional    |
| `branch` | Custom branch name | Optional    |

If no title is provided, kvelmo uses the filename.

## Body Content

The markdown body becomes the task description. Use:

- Headers for sections
- Lists for requirements
- Code blocks for examples
- Any standard markdown

## Example Files

### Feature Request

```markdown
---
title: Add search functionality
---

Add a search bar to the header that filters products.

## Requirements

- Search input in header
- Filter products by name
- Debounce input (300ms)
- Show "No results" when empty
```

### Bug Fix

```markdown
---
title: Fix null pointer in checkout
---

Users report crash when checking out with empty cart.

## Steps to Reproduce

1. Go to checkout with empty cart
2. Click "Complete Order"
3. App crashes

## Expected Behavior

Show error message: "Your cart is empty"
```

### Documentation Update

```markdown
---
title: Update API documentation
---

Update the API docs for the new v2 endpoints.

## Changes Needed

- Add /api/v2/users endpoint
- Update authentication section
- Add rate limiting info
```

## Authentication

No authentication required. Files are read from the local filesystem.

## Limitations

- Files must be accessible from the current directory
- No remote file support
- Path must be relative or absolute

## Related

- [Providers Overview](/providers/index.md)
- [GitHub Provider](/providers/github.md)
