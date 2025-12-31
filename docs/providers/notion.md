# Notion Provider

**Schemes:** `notion:`, `nt:`

**Capabilities:** `read`, `list`, `fetch_comments`, `comment`, `update_status`, `manage_labels`, `create_work_unit`, `snapshot`

Integrates with Notion for flexible task management using pages and databases.

## Usage

```bash
# By page ID (32-char UUID, with or without dashes)
mehr start notion:a1b2c3d4e5f678901234567890abcdef1
mehr plan nt:a1b2c3d4-e5f6-7890-1234-567890abcdef1

# By Notion URL
mehr start notion:https://www.notion.so/Page-Title-a1b2c3d4e5f678901234567890abcdef1
```

## Configuration

```yaml
notion:
  token: "${NOTION_TOKEN}"
  database_id: "optional-default-database"  # For list operations
  status_property: "Status"                  # Property name for status
  description_property: "Description"        # Property name for description
  labels_property: "Tags"                    # Multi-select property for labels
```

## Token Resolution

1. `MEHR_NOTION_TOKEN` environment variable
2. `NOTION_TOKEN` environment variable
3. Token from `config.yaml`

## Features

- **Page Fetching**: Retrieves title, content blocks, status, labels, assignees
- **Database Querying**: List pages from databases with status/label filtering
- **Hybrid Approach**: Works with individual pages or database queries
- **Status Mapping**: Maps Notion status/select properties to provider statuses
- **Label Management**: Add/remove multi-select labels
- **Comment Support**: Fetch and add comments to pages
- **Status Updates**: Change page status through configurable property
- **Page Creation**: Create new pages in databases
- **Snapshots**: Export pages as markdown with comments
- **Configurable Properties**: Customize which properties map to status/description/labels

## Status Mapping

| Notion Status | Provider Status |
|---------------|-----------------|
| Not Started, Backlog, Todo | Open |
| In Progress, Started, Doing | In Progress |
| In Review, Reviewing | Review |
| Done, Completed, Finished | Done |
| Cancelled, Archived | Closed |

## Reference Formats

| Format | Example |
|--------|---------|
| Scheme with page ID | `notion:a1b2c3d4e5f678901234567890abcdef1` |
| Short scheme | `nt:a1b2c3d4e5f678901234567890abcdef1` |
| UUID with dashes | `notion:a1b2c3d4-e5f6-7890-1234-567890abcdef1` |
| Notion URL | `notion:https://www.notion.so/Page-Title-a1b2c3d4e5f6...` |

## Property Configuration

The Notion provider maps Notion properties to work unit fields:

| Provider Field | Default Property Name | Notes |
|----------------|----------------------|-------|
| Status | `Status` | Status or select property type |
| Description | `Description` | Falls back to page content blocks |
| Labels | `Tags` | Multi-select property type |
| Assignee | `Assignee` or `Owner` | People property type |

You can customize these property names in your config.
