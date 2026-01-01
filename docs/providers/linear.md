# Linear Provider

> **⚠️ Third-Party Integration**: This integration depends on external APIs that may change. Not fully tested beyond unit tests. Behavior may vary depending on the third-party service. Manual validation recommended before production use.


**Schemes:** `linear:`, `ln:`

**Capabilities:** `read`, `list`, `fetch_comments`, `comment`, `update_status`, `manage_labels`, `create_work_unit`, `snapshot`, `fetch_subtasks`

Integrates with Linear for modern project management and issue tracking.

## Usage

```bash
# By issue identifier
mehr start linear:ENG-123
mehr plan ln:ENG-123

# By issue URL
mehr start linear:https://linear.app/team/issue/ENG-123-title
```

## Configuration

```yaml
linear:
  token: "${LINEAR_API_KEY}"
  team: "ENG"  # Optional: default team key for operations
```

## Token Resolution

1. `MEHR_LINEAR_API_KEY` environment variable
2. `LINEAR_API_KEY` environment variable
3. Token from `config.yaml`

## Features

- **Issue Fetching**: Retrieves title, description, status, priority, labels, assignees
- **Status Mapping**: Maps Linear states to provider statuses (Backlog/Todo → Open, In Progress → In Progress, Done → Done)
- **Priority Mapping**: Maps Linear priorities (Urgent → Critical, High → High, Medium → Normal, Low → Low)
- **Comment Support**: Fetch and add comments to issues
- **Label Management**: Add and remove labels on issues
- **Status Updates**: Change issue state through workflow
- **Issue Creation**: Create new Linear issues
- **Snapshots**: Export issues as markdown with comments

## Status Mapping

| Linear State | Provider Status |
|--------------|-----------------|
| Backlog, Todo, Unstarted | Open |
| In Progress, Started, In Review | In Progress |
| Done, Completed | Done |
| Canceled, Cancelled | Closed |

## Priority Mapping

| Linear Priority | Provider Priority |
|-----------------|------------------|
| Urgent | Critical |
| High | High |
| Medium | Normal |
| Low | Low |
| No priority | Normal |
