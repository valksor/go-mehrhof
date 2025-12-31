# Asana Provider

**Schemes:** `asana:`, `as:`

**Capabilities:** `read`, `list`, `fetch_comments`, `comment`, `update_status`, `manage_labels`, `snapshot`, `fetch_subtasks`

Integrates with Asana for task and project management.

## Usage

```bash
# By task GID (16-17 digit number)
mehr start asana:1234567890123456
mehr plan as:1234567890123456
```

## Configuration

```yaml
asana:
  token: "${ASANA_TOKEN}"
  workspace: "123456789"           # Optional: default workspace GID
  default_project: "987654321"     # Optional: default project for listing
```

## Token Resolution

1. `MEHR_ASANA_TOKEN` environment variable
2. `ASANA_TOKEN` environment variable
3. Token from `config.yaml`

## Authentication

Create a Personal Access Token at: https://app.asana.com/0/developer-console

## Features

- **Task Fetching**: Retrieves task name, notes, due dates, assignees, tags, custom fields
- **List Tasks**: Browse tasks from projects with status filtering
- **Comment Support**: Fetch and add stories (comments) to tasks
- **Status Updates**: Move tasks between sections
- **Tag Management**: Add and remove tags
- **Snapshots**: Export task content as markdown

## Status Mapping

Status is determined by the task's section in Asana (case-insensitive):

| Asana Section | Provider Status |
|---------------|-----------------|
| To Do, Backlog, New | Open |
| In Progress, Doing | In Progress |
| In Review, Review | Review |
| Done, Complete | Done |
| Archived | Closed |
