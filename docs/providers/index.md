# Providers

Providers are task sources that Mehrhof can read from and interact with. Each provider supports different capabilities like reading tasks, creating issues, or managing status.

> **⚠️ Integration Testing Notice**: Agent and provider implementations depend on third-party APIs that may change. While unit-tested, these integrations may have edge cases not covered by automated tests. Manual testing in your environment is recommended before production use.


## Available Providers

| Provider | Schemes | Description |
|----------|---------|-------------|
| **File** | `file:` | Local markdown files |
| **Directory** | `dir:` | Local directories with markdown files |
| **GitHub** | `github:`, `gh:` | GitHub issues |
| **GitLab** | `gitlab:`, `gl:` | GitLab issues |
| **Jira** | `jira:`, `j:` | Jira issues |
| **Linear** | `linear:`, `ln:` | Linear issues |
| **Notion** | `notion:`, `nt:` | Notion pages and databases |
| **Wrike** | `wrike:`, `wk:` | Wrike tasks |
| **YouTrack** | `youtrack:`, `yt:` | YouTrack issues |
| **Trello** | `trello:`, `tr:` | Trello cards |
| **Asana** | `asana:`, `as:` | Asana tasks |
| **ClickUp** | `clickup:`, `cu:` | ClickUp tasks |
| **Azure DevOps** | `azdo:`, `azure:` | Azure DevOps work items |
| **Bitbucket** | `bitbucket:`, `bb:` | Bitbucket issues |

## Provider Capabilities

| Capability | Description |
|------------|-------------|
| `read` | Fetch individual tasks |
| `list` | Browse/list multiple tasks |
| `comment` | Add comments to tasks |
| `fetch_comments` | Retrieve existing comments |
| `update_status` | Change task status |
| `manage_labels` | Add/remove labels |
| `create_work_unit` | Create new tasks |
| `create_pr` | Create pull requests |
| `download_attachment` | Download file attachments |
| `snapshot` | Capture task content for storage |
| `fetch_subtasks` | Retrieve subtasks/child items |

### Subtask Support

Providers implement subtasks differently based on their API:

| Provider | Subtask Source |
|----------|----------------|
| Jira, Asana, ClickUp, Wrike, YouTrack | Native subtask API |
| Linear | Child issues via GraphQL |
| Azure DevOps | Child work items via relations |
| Trello | Checklist items converted to subtasks |
| GitHub, GitLab, Bitbucket | Task lists parsed from markdown (`- [ ]` / `- [x]`) |

## Plugin Providers

You can extend Mehrhof with custom providers through the plugin system. See [Plugins](../concepts/plugins.md) for details.

### Installing a Provider Plugin

```bash
mehr plugins install https://github.com/user/mehrhof-jira

mehr plugins install ./plugins/my-provider

mehr plugins list
```

### Creating a Custom Provider

Provider plugins communicate via JSON-RPC 2.0 over stdin/stdout. See the plugin development guide for implementation details.

## Provider Reference Formats

| Provider | Format | Example |
|----------|--------|---------|
| File | `file:path/to/file.md` | `file:tasks/auth.md` |
| Directory | `dir:path/to/directory` | `dir:./tasks` |
| GitHub | `github:N` or `github:owner/repo#N` | `github:123`, `github:owner/repo#456` |
| GitLab | `gitlab:N` or `gitlab:group/project#N` | `gitlab:123`, `gitlab:group/project#456` |
| Jira | `jira:KEY-NUM` or URL | `jira:JIRA-123`, `jira:https://domain.atlassian.net/browse/...` |
| Linear | `linear:TEAM-NUM` or URL | `linear:ENG-123`, `linear:https://linear.app/...` |
| Notion | `notion:page-id` or URL | `notion:a1b2c3d4e5f6...`, `notion:https://notion.so/...` |
| Wrike | `wrike:ID` or permalink | `wrike:IEAGI2D4I4AL7YNL` |
| YouTrack | `youtrack:ABC-123` or URL | `youtrack:ABC-123`, `youtrack:https://...` |
| Trello | `trello:ID` or `trello:shortLink` | `trello:507f1f77bcf86cd799439011`, `trello:abc12XYZ` |
| Asana | `asana:TASK-GID` | `asana:1234567890123456` |
| ClickUp | `clickup:ID` or `clickup:TASK-ID` | `clickup:abc123xyz`, `clickup:TASK-123` |
| Azure DevOps | `azdo:ID` or `azdo:org/project#ID` | `azdo:123`, `azdo:org/project#456` |
| Bitbucket | `bitbucket:ID` or `bb:workspace/repo#ID` | `bb:123`, `bb:workspace/repo#456` |

## Auto-Detection

Some providers can auto-detect configuration from your environment:

- **GitHub**: Detects `owner/repo` from `git remote origin`
- **File/Directory**: Resolves relative paths from current working directory

## Default Provider

Set a default provider to avoid typing scheme prefixes:

```yaml
# .mehrhof/config.yaml
providers:
  default: file  # or github, dir, etc.
```

Now you can use bare references:

```bash
mehr start task.md
mehr start 123
```
