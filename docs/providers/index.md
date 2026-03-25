# Providers

Providers are external task sources that kvelmo can load into the local workflow.

They are how issues, work items, and task descriptions enter the orchestration system before planning and implementation begin.

## Supported Providers

| Provider | Description | Reference Format |
|----------|-------------|------------------|
| [Azure DevOps](/providers/azuredevops.md) | Azure DevOps work items | `ado:12345` |
| [File](/providers/file.md) | Local markdown files | `file:path/to/task.md` |
| [GitHub](/providers/github.md) | GitHub issues and PRs | `github:owner/repo#123` |
| [GitLab](/providers/gitlab.md) | GitLab issues and merge requests | `gitlab:group/project#123` |
| [Jira](/providers/jira.md) | Jira issues | `jira:PROJ-123` |
| [Linear](/providers/linear.md) | Linear issues | `linear:ENG-123` |
| [Wrike](/providers/wrike.md) | Wrike tasks | `wrike:taskid` |

## How Providers Fit the Workflow

Providers only supply the task source. Once loaded, the task behaves like any other local kvelmo task:

- it enters the same workflow state machine
- it is visible in the Web UI, CLI, desktop app, and TUI
- it gets the same checkpoints, review flow, and submission control

## Loading a Task

Use `start` with a provider reference:

```bash
kvelmo start --from file:task.md
kvelmo start --from github:valksor/kvelmo#123
kvelmo start --from gitlab:group/project#456
kvelmo start --from linear:ENG-123
kvelmo start --from wrike:abc123
kvelmo start --from ado:12345
kvelmo start --from jira:PROJ-123
```

You can also start from plain text when no provider is needed:

```bash
kvelmo start "Refactor the settings panel and update tests"
```

## Authentication

Some providers need authentication before kvelmo can fetch task data.

| Provider | Typical token variable |
|----------|------------------------|
| Azure DevOps | `AZURE_DEVOPS_TOKEN` |
| GitHub | `GITHUB_TOKEN` |
| GitLab | `GITLAB_TOKEN` |
| Jira | `JIRA_TOKEN` |
| Linear | `LINEAR_TOKEN` |
| Wrike | `WRIKE_TOKEN` |

Recommended path:

```bash
kvelmo github login
kvelmo gitlab login
kvelmo linear login
kvelmo wrike login
```

You can also manage provider configuration through kvelmo settings and environment files.

## What Provider Data Becomes

Provider loading usually extracts:

- title
- description
- external identifier
- canonical URL
- provider-specific metadata

That data becomes the local task context used for planning, implementation, review, and submission.

## Related

- [CLI start](/cli/start.md)
- [Configuration](/configuration/index.md)
- [Provider login subcommands](/cli/login.md)
