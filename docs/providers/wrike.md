# Wrike Provider

**Schemes:** `wrike:`, `wk:`

**Capabilities:** `read`, `list`, `fetch_comments`, `comment`, `update_status`, `manage_labels`, `create_work_unit`, `download_attachment`, `snapshot`, `fetch_subtasks`

Integrates with Wrike for enterprise project management.

## Usage

```bash
# By task ID
mehr start wrike:IEAGI2D4I4AL7YNL
mehr plan wk:IEAGI2D4I4AL7YNL

# By permalink
mehr start wrike:https://app.wrike.com/open.htm?id=123456

# By numeric ID (10+ digits)
mehr start wk:1234567890
```

## Configuration

```yaml
wrike:
  token: "${WRIKE_TOKEN}"
  host: "https://www.wrike.com/api/v4"  # Optional: override API base URL
```

## Token Resolution

1. `MEHR_WRIKE_TOKEN` environment variable
2. `WRIKE_TOKEN` environment variable
3. Token from `config.yaml`

## Features

- **Task Fetching**: Retrieves title, description, status, priority, permalink
- **Subtasks**: Recursively fetches nested subtasks (max depth: 5)
- **Comments**: Fetches all comments with automatic pagination support
- **Attachments**: Lists and downloads file attachments
- **Retry Logic**: Automatic exponential backoff for rate limit errors (429)
- **Multiple ID Formats**: Supports numeric IDs, API IDs (`IEAAJ...`), and permalink URLs
