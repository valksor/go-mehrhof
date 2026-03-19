# Providers

Providers are task sources that kvelmo can load tasks from.

## Supported Providers

| Provider                                    | Description               | Reference Format        |
|---------------------------------------------|---------------------------|-------------------------|
| [Azure DevOps](/providers/azuredevops.md)   | Azure DevOps work items   | `ado:12345`             |
| [File](/providers/file.md)                  | Local markdown files      | `file:path/to/task.md`  |
| [GitHub](/providers/github.md)              | GitHub issues and PRs     | `github:owner/repo#123` |
| [GitLab](/providers/gitlab.md)              | GitLab issues and MRs     | `gitlab:project#123`    |
| [Jira](/providers/jira.md)                  | Jira issues               | `jira:PROJ-123`         |
| [Linear](/providers/linear.md)              | Linear app issues         | `linear:ENG-123`        |
| [Wrike](/providers/wrike.md)                | Wrike tasks               | `wrike:taskid`          |

## Using Providers

Load a task from a provider:

```bash
# From a file
kvelmo start --from file:task.md

# From GitHub
kvelmo start --from github:valksor/kvelmo#123

# From GitLab
kvelmo start --from gitlab:group/project#456

# From Linear
kvelmo start --from linear:ENG-123

# From Wrike
kvelmo start --from wrike:abc123

# From Azure DevOps
kvelmo start --from ado:12345

# From Jira
kvelmo start --from jira:PROJ-123
```

## Provider Authentication

Some providers require authentication:

| Provider    | Token Variable       |
|-------------|----------------------|
| Azure DevOps| `AZURE_DEVOPS_TOKEN` |
| File        | None                 |
| GitHub      | `GITHUB_TOKEN`       |
| GitLab      | `GITLAB_TOKEN`       |
| Jira        | `JIRA_TOKEN`         |
| Linear      | `LINEAR_TOKEN`       |
| Wrike       | `WRIKE_TOKEN`        |

### Setting Tokens

Use the login command (recommended):
```bash
kvelmo github login
kvelmo gitlab login
kvelmo linear login
kvelmo wrike login
```

Or add directly to `.env` file:
```bash
# Global (~/.valksor/kvelmo/.env)
echo "GITHUB_TOKEN=ghp_xxxx" >> ~/.valksor/kvelmo/.env

# Project-specific (<project>/.valksor/.env)
echo "GITHUB_TOKEN=ghp_yyyy" >> .valksor/.env
```

## Task Data

Providers extract:

| Field       | Description            |
|-------------|------------------------|
| Title       | Task title             |
| Description | Task body/description  |
| External ID | Provider-specific ID   |
| URL         | Link to original task  |
| Metadata    | Provider-specific data |

## Provider Registry

kvelmo's provider registry allows lookup by name:

```go
provider := provider.Get("github")
task, err := provider.Fetch(ctx, "owner/repo#123")
```

## Adding Custom Providers

Implement the Provider interface:

```go
type Provider interface {
    Name() string
    Fetch(ctx context.Context, ref string) (*Task, error)
    Update(ctx context.Context, task *Task) error
}
```

Register with:
```go
provider.Register("myprovider", NewMyProvider)
```

## Related

- [Azure DevOps Provider](/providers/azuredevops.md)
- [File Provider](/providers/file.md)
- [GitHub Provider](/providers/github.md)
- [GitLab Provider](/providers/gitlab.md)
- [Jira Provider](/providers/jira.md)
- [Linear Provider](/providers/linear.md)
- [Wrike Provider](/providers/wrike.md)
