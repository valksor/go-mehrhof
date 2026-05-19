# Azure DevOps Provider

Load tasks from Azure DevOps work items and create pull requests.

## Usage

```bash
# From a work item ID
kvelmo start --from azuredevops:12345

# Short alias
kvelmo start --from ado:12345

# From a URL
kvelmo start --from https://dev.azure.com/myorg/myproject/_workitems/edit/12345
```

## Authentication

Set your personal access token (requires **Work Items** and **Code** scopes):

```bash
kvelmo config set providers.azuredevops.token "your-pat-here"
```

Or set the environment variable:

```bash
export AZURE_DEVOPS_TOKEN=your-pat-here
```

## Configuration

```yaml
providers:
  azuredevops:
    organization: myorg # Required: Azure DevOps organization name
    project: myproject # Required: Azure DevOps project name
    repository: myrepo # Git repo name (defaults to project name)
    base_url: https://dev.azure.com # Instance URL (default)
    allow_ticket_comment: true # Post status comments on work items
    status_sync: true # Update work item state on transitions
    status_mapping: # Map kvelmo states to Azure DevOps states
      implementing: Active
      reviewing: "In Review"
      submitted: Resolved
```

## Features

| Feature                  | Supported                        |
| ------------------------ | -------------------------------- |
| Fetch work items         | Yes                              |
| Update status            | Yes (JSON Patch on System.State) |
| Add comments             | Yes                              |
| Create pull requests     | Yes                              |
| Status sync              | Yes (configurable mapping)       |
| Hierarchy (parent/child) | No                               |

## Supported Work Item Types

Bug, Task, User Story, Feature, Epic, and custom types. The work item type is stored in task metadata as `azuredevops_type`.

## URL Formats

The provider recognizes these URL patterns:

- `https://dev.azure.com/{org}/{project}/_workitems/edit/{id}`
- `https://{org}.visualstudio.com/{project}/_workitems/edit/{id}`

## Status Mapping

Configure how kvelmo lifecycle states map to Azure DevOps work item states:

```yaml
status_mapping:
  loaded: New
  planning: Active
  implementing: Active
  reviewing: "In Review"
  submitted: Resolved
  finished: Closed
```

If no mapping is configured, the raw kvelmo state name is sent to Azure DevOps.
