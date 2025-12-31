# ClickUp Provider

**Schemes:** `clickup:`, `cu:`

**Capabilities:** `read`, `list`, `fetch_comments`, `comment`, `update_status`, `manage_labels`, `create_work_unit`, `snapshot`, `fetch_subtasks`

Integrates with ClickUp for comprehensive task management.

## Usage

```bash
# By task ID
mehr start clickup:abc123xyz
mehr plan cu:abc123xyz

# By custom task ID (if enabled)
mehr start cu:TASK-123
```

## Configuration

```yaml
clickup:
  token: "${CLICKUP_TOKEN}"
  workspace: "123456"            # Optional: workspace ID
  default_list: "987654"         # Optional: default list for operations
  custom_task_ids: true          # Enable PROJ-123 format
```

## Token Resolution

1. `MEHR_CLICKUP_TOKEN` environment variable
2. `CLICKUP_TOKEN` environment variable
3. Token from `config.yaml`

## Authentication

Create an API token at: https://app.clickup.com/settings/apps

## Reference Formats

| Format | Example |
|--------|---------|
| Scheme with task ID | `clickup:abc123xyz` |
| Short scheme | `cu:abc123xyz` |
| Custom task ID | `cu:TASK-123` |

## Features

- **Task Fetching**: Retrieves task name, description, status, priority, tags, assignees, due dates
- **List Tasks**: Browse tasks from lists with status filtering
- **Comment Support**: Fetch and add comments
- **Status Updates**: Change task status
- **Tag Management**: Add and remove tags
- **Task Creation**: Create new tasks in lists
- **Snapshots**: Export task content as markdown

## Status Mapping

| ClickUp Status | Provider Status |
|----------------|-----------------|
| to do, open | Open |
| in progress | In Progress |
| in review | Review |
| complete, closed | Done |
