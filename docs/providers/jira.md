# Jira Provider

**Schemes:** `jira:`, `j:`

**Capabilities:** `read`, `list`, `fetch_comments`, `comment`, `update_status`, `manage_labels`, `create_work_unit`, `download_attachment`, `snapshot`, `fetch_subtasks`

Integrates with Jira for comprehensive issue tracking. Supports both Jira Cloud and Jira Server/Data Center.

## Usage

```bash
# By issue key
mehr start jira:JIRA-123
mehr plan j:PROJ-456

# By URL
mehr start jira:https://domain.atlassian.net/browse/JIRA-123
mehr start j:https://jira.example.com/browse/PROJ-456
```

## Configuration

```yaml
jira:
  token: "${JIRA_TOKEN}"        # API token
  email: "user@example.com"     # Email for Cloud auth
  base_url: "https://domain.atlassian.net"  # Optional, auto-detected
  project: "PROJ"               # Default project key for operations
```

## Token Resolution

1. `MEHR_JIRA_TOKEN` environment variable
2. `JIRA_TOKEN` environment variable
3. Token from `config.yaml`

## Authentication

The Jira provider supports different authentication methods based on your Jira deployment:

- **Jira Cloud**: Uses email + API token via Basic Auth. Generate a token at [id.atlassian.com](https://id.atlassian.com/manage-profile/security/api-tokens).
- **Jira Server/Data Center**: Uses PAT (Personal Access Token) or Basic Auth.

## Reference Formats

| Format | Example |
|--------|---------|
| Scheme with key | `jira:JIRA-123` |
| Short scheme | `j:PROJ-456` |
| Issue URL | `jira:https://domain.atlassian.net/browse/JIRA-123` |
| Jira Server URL | `j:https://jira.example.com/browse/PROJ-456` |

## Features

- **Issue Fetching**: Retrieves title, description, status, priority, labels, assignees, attachments
- **List Issues**: JQL-based filtering with status/label support and pagination
- **Comment Support**: Fetch all comments and add new ones
- **Status Updates**: Change issue status via workflow transitions
- **Label Management**: Add and remove labels on issues
- **Issue Creation**: Create new issues with project, priority, type
- **Attachments**: Download file attachments
- **Snapshots**: Export issue content as markdown
- **Auto-Detection**: Base URL automatically detected from issue URLs

## Status Mapping

| Jira Status | Provider Status |
|-------------|-----------------|
| To Do, Backlog, Open, New | Open |
| In Progress, Started, In Development | In Progress |
| In Review, Code Review, Under Review, Verification | Review |
| Done, Closed, Resolved, Complete, Finished | Done |
| Won't Fix, Cancelled, Obsolete, Won't Do | Closed |

## Priority Mapping

| Jira Priority | Provider Priority |
|---------------|------------------|
| Highest, Critical | Critical |
| High | High |
| Medium, Normal, Default | Normal |
| Low, Lowest | Low |

## Workflow Transitions

Jira requires workflow transitions rather than direct status changes. The provider:

1. Fetches available transitions for the issue
2. Finds a matching transition by name (case-insensitive)
3. Executes the transition

Common transition names mapped:
- **Open**: "To Do", "Backlog", "Open", "Reopen", "New"
- **In Progress**: "In Progress", "Start Progress", "Start Development"
- **Review**: "In Review", "Code Review", "Ready for Review"
- **Done**: "Done", "Close", "Resolve", "Complete", "Mark as Done"
- **Closed**: "Closed", "Cancel", "Won't Fix", "Won't Do"
