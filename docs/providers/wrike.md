# Wrike Provider

Load tasks from Wrike.

## Usage

```bash
kvelmo start --from wrike:IEAAABCD
```

## Authentication

Set your Wrike token:

```bash
export WRIKE_TOKEN=your-wrike-api-token
```

Or in settings:

```json
{
  "providers": {
    "wrike": {
      "token": "your-wrike-api-token"
    }
  }
}
```

### Creating a Token

1. Go to Wrike → Apps & Integrations → API
2. Create a new permanent token
3. Copy and save the token

## Reference Format

```
wrike:<task-id>
```

The task ID is the alphanumeric ID from Wrike (visible in the task URL).

Example:

- `wrike:FOO` — Task with ID FOO

## Extracted Data

| Field       | Source           |
| ----------- | ---------------- |
| Title       | Task title       |
| Description | Task description |
| External ID | Wrike task ID    |
| URL         | Wrike permalink  |
| Status      | Task status      |

## Hierarchy Context

Wrike supports hierarchy context (parent/sibling tasks). Enable in settings:

```json
{
  "providers": {
    "wrike": {
      "fetch_hierarchy": true,
      "max_siblings": 5
    }
  }
}
```

This provides context about related tasks to the AI agent.

## Submitting Back

When you run `kvelmo submit`, kvelmo can:

1. Create a PR with your changes
2. Update the Wrike task status
3. Add a comment with the PR link

## Configuration

```json
{
  "providers": {
    "wrike": {
      "token": "your-token",
      "fetch_hierarchy": true,
      "max_siblings": 5
    }
  }
}
```

## Troubleshooting

### "401 Unauthorized"

Your token is invalid or expired. Generate a new one in Wrike.

### "404 Not Found"

- Check the task ID is correct
- Ensure your account has access to the task
- Verify the task hasn't been deleted

## Related

- [Providers Overview](/providers/index.md)
- [GitHub Provider](/providers/github.md)
