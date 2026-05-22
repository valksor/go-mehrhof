# Azure DevOps

Load tasks from Azure DevOps work items and create pull requests.

> **Support tier: basic.** Azure DevOps covers the core lifecycle — fetch a work
> item, sync status, comment, and open a pull request — which is enough to run a
> task end to end. It does **not** yet implement the richer capabilities that
> GitHub, GitLab, and Linear offer (hierarchy context, label management, task
> listing/creation, fetching existing comments). If you need those, prefer one of
> those providers or open an issue. See the capability matrix below.

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

## Capability matrix

| Capability                       | Azure DevOps | GitHub / GitLab / Linear |
| -------------------------------- | ------------ | ------------------------ |
| Fetch work item / issue          | Yes          | Yes                      |
| Update status (configurable map) | Yes          | Yes                      |
| Post status comment              | Yes          | Yes                      |
| Create pull request              | Yes          | Yes                      |
| Hierarchy context (parent/sibs)  | No           | Yes                      |
| Label management                 | No           | Yes                      |
| List tasks                       | No           | Yes                      |
| Create tasks                     | No           | Yes                      |
| Fetch existing comments          | No           | Yes                      |

Capabilities are detected at runtime via optional provider interfaces, so a
feature that depends on an unsupported capability is simply skipped for Azure
DevOps rather than failing the workflow.

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
