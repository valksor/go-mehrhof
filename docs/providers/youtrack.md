# YouTrack Provider

> **⚠️ Third-Party Integration**: This integration depends on external APIs that may change. Not fully tested beyond unit tests. Behavior may vary depending on the third-party service. Manual validation recommended before production use.


**Schemes:** `youtrack:`, `yt:`

**Capabilities:** `read`, `list`, `fetch_comments`, `comment`, `update_status`, `manage_labels`, `create_work_unit`, `download_attachment`, `snapshot`, `fetch_subtasks`

Integrates with JetBrains YouTrack for comprehensive issue tracking.

## Usage

```bash
mehr start youtrack:ABC-123
mehr plan yt:ABC-123

mehr start ABC-123

mehr start yt:https://company.myjetbrains.com/youtrack/issue/ABC-123
mehr start yt:https://youtrack.cloud/issue/ABC-123
```

## Configuration

```yaml
youtrack:
  token: "${YOUTRACK_TOKEN}"
  host: "https://company.myjetbrains.com/youtrack"  # Optional: override host
```

## Token Resolution

1. `MEHR_YOUTRACK_TOKEN` environment variable
2. `YOUTRACK_TOKEN` environment variable
3. Token from `config.yaml`

## Reference Formats

| Format | Example |
|--------|---------|
| Scheme with ID | `youtrack:ABC-123` |
| Short scheme | `yt:ABC-123` |
| Bare ID | `ABC-123` (auto-detected) |
| Issue URL | `https://company.myjetbrains.com/youtrack/issue/ABC-123` |

## Features

- **Issue Fetching**: Retrieves title, description, status, priority, tags, assignees, custom fields
- **List Issues**: Query-based filtering with status/tag support and pagination
- **Comment Support**: Fetch all comments and add new ones
- **Tag Management**: Add/remove tags (YouTrack's label equivalent)
- **Status Updates**: Change issue state via custom field update
- **Issue Creation**: Create new issues with project, priority, type
- **Attachments**: Download file attachments
- **Snapshots**: Export issue content as markdown

## State Mapping

| Mehrhof Status | YouTrack States |
|----------------|-----------------|
| `open` | New, Submitted, To be done |
| `in_progress` | In Progress, Active |
| `review` | Code Review, Verification |
| `done` | Fixed, Done, Completed, Verified, Resolved |
| `closed` | Closed, Won't fix, Can't reproduce, Duplicate, Obsolete |

## Priority Mapping

| Mehrhof Priority | YouTrack Priority |
|------------------|-------------------|
| `critical` | Critical, Show-stopper, Urgent |
| `high` | Major, High |
| `normal` | Normal |
| `low` | Minor, Low |
