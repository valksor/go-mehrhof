# Azure DevOps Provider

> **⚠️ Third-Party Integration**: This integration depends on external APIs that may change. Not fully tested beyond unit tests. Behavior may vary depending on the third-party service. Manual validation recommended before production use.


**Schemes:** `azdo:`, `azure:`

**Capabilities:** `read`, `list`, `fetch_comments`, `comment`, `update_status`, `manage_labels`, `create_work_unit`, `download_attachment`, `create_pr`, `snapshot`, `fetch_subtasks`

Integrates with Azure DevOps for work item tracking and pull request management.

## Usage

```bash
# By work item ID (requires configured org/project)
mehr start azdo:123
mehr plan azure:123

# Explicit organization and project
mehr start azdo:org/project#456

# By URL
mehr start azdo:https://dev.azure.com/org/project/_workitems/edit/123
```

## Configuration

```yaml
azure_devops:
  token: "${AZURE_DEVOPS_TOKEN}"    # Personal Access Token
  organization: "myorg"
  project: "MyProject"
  area_path: "MyProject\\Area"      # Optional: default area path
  branch_pattern: "feature/{key}-{slug}"
  target_branch: "main"
```

## Token Resolution

1. `MEHR_AZURE_DEVOPS_TOKEN` environment variable
2. `AZURE_DEVOPS_TOKEN` environment variable
3. Token from `config.yaml`

## Authentication

Create a Personal Access Token at: https://dev.azure.com/{organization}/_usersSettings/tokens

Required scopes: `Work Items (Read, Write)`, `Code (Read, Write)` for PR creation.

## Reference Formats

| Format | Example |
|--------|---------|
| Scheme with ID | `azdo:123` |
| Short scheme | `azure:456` |
| Org/project with ID | `azdo:org/project#123` |
| URL | `azdo:https://dev.azure.com/org/project/_workitems/edit/123` |

## Features

- **Work Item Fetching**: Retrieves title, description, state, priority, tags, assignees
- **List Work Items**: WIQL query support with status filtering
- **Discussion Support**: Fetch and add comments
- **State Updates**: Change work item state
- **Tag Management**: Add and remove tags
- **Work Item Creation**: Create Bugs, Tasks, User Stories, Features
- **PR Creation**: Create pull requests with automatic work item linking (AB#123 syntax)
- **Snapshots**: Export work item content as markdown

## Status Mapping

| Azure DevOps State | Provider Status |
|--------------------|-----------------|
| New, To Do | Open |
| Active, In Progress, Doing | In Progress |
| Resolved | Review |
| Closed, Done | Done |
| Removed | Closed |

## Priority Mapping

| Azure DevOps Priority | Provider Priority |
|-----------------------|-------------------|
| 1 | Critical |
| 2 | High |
| 3 | Normal |
| 4 | Low |

## Work Item Types

Supported work item types: Bug, Task, User Story, Feature, Epic (configurable by project).
