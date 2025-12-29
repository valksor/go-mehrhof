# Providers

Providers are task sources that Mehrhof can read from and interact with. Each provider supports different capabilities like reading tasks, creating issues, or managing status.

## Available Providers

| Provider | Schemes | Description |
|----------|---------|-------------|
| **File** | `file:` | Local markdown files |
| **Directory** | `dir:` | Local directories with markdown files |
| **GitHub** | `github:`, `gh:` | GitHub issues |
| **Jira** | `jira:`, `j:` | Jira issues |
| **Linear** | `linear:`, `ln:` | Linear issues |
| **Notion** | `notion:`, `nt:` | Notion pages and databases |
| **Wrike** | `wrike:`, `wk:` | Wrike tasks |
| **YouTrack** | `youtrack:`, `yt:` | YouTrack issues |

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

---

## File Provider

**Schemes:** `file:`

**Capabilities:** `read`

Reads tasks from local markdown files.

### Usage

```bash
mehr start file:task.md
mehr plan file:features/user-auth.md
```

### File Format

```markdown
---
title: Add User Authentication
agent: claude
---

Implement JWT-based authentication with login/logout endpoints.
```

The file provider extracts metadata from YAML frontmatter and uses the remaining content as the task description.

---

## Directory Provider

**Schemes:** `dir:`

**Capabilities:** `read`, `list`

Reads tasks from markdown files in a local directory. Can list all available task files.

### Usage

```bash
mehr start dir:./tasks
mehr plan dir:./docs
```

### Listing Files

The directory provider can enumerate all markdown files in a directory, allowing you to browse available tasks before starting one.

---

## GitHub Provider

**Schemes:** `github:`, `gh:`

**Capabilities:** `read`, `list`, `fetch_comments`, `comment`, `update_status`, `manage_labels`, `create_work_unit`, `create_pr`, `download_attachment`, `snapshot`

Interacts with GitHub issues for fully integrated task management.

### Usage

```bash
# By issue number (auto-detects repo)
mehr start github:123
mehr plan gh:123

# Explicit repository
mehr start github:owner/repo#456
mehr plan gh:owner/repo#456
```

### Configuration

Configure in `.mehrhof/config.yaml`:

```yaml
github:
  token: "${GITHUB_TOKEN}"        # GitHub API token
  owner: "myorg"                   # Repository owner
  repo: "myrepo"                   # Repository name
  branch_pattern: "issue/{key}-{slug}"  # Branch naming
  commit_prefix: "[#{key}]"        # Commit message prefix
  target_branch: "main"            # Default target branch
  draft_pr: false                  # Create draft PRs

  # Automated comments
  comments:
    enabled: false
    on_branch_created: true
    on_plan_done: true
    on_implement_done: true
    on_pr_created: true
```

### Token Resolution

The GitHub provider tries token sources in this order:

1. `MEHR_GITHUB_TOKEN` environment variable
2. `GITHUB_TOKEN` environment variable
3. Token from `config.yaml`
4. GitHub CLI token (`gh auth token`)

### Features

- **Issue Fetching**: Retrieves title, body, labels, assignees, comments
- **Label Inference**: Maps labels to task types (`bug` → `fix`, `feature` → `feature`)
- **Priority Inference**: Extracts priority from label names
- **Linked Issues**: Detects `#123` references in issue body
- **PR Creation**: Automatically creates pull requests after implementation
- **Status Updates**: Close/reopen issues
- **Label Management**: Add or remove labels
- **Issue Creation**: Create new GitHub issues

### Task Type Label Mapping

| GitHub Label | Task Type |
|--------------|-----------|
| `bug`, `bugfix`, `fix` | `fix` |
| `feature`, `enhancement` | `feature` |
| `docs`, `documentation` | `docs` |
| `refactor` | `refactor` |
| `chore` | `chore` |
| `test` | `test` |
| `ci` | `ci` |

---

## Jira Provider

**Schemes:** `jira:`, `j:`

**Capabilities:** `read`, `list`, `fetch_comments`, `comment`, `update_status`, `manage_labels`, `create_work_unit`, `download_attachment`, `snapshot`

Integrates with Jira for comprehensive issue tracking. Supports both Jira Cloud and Jira Server/Data Center.

### Usage

```bash
# By issue key
mehr start jira:JIRA-123
mehr plan j:PROJ-456

# By URL
mehr start jira:https://domain.atlassian.net/browse/JIRA-123
mehr start j:https://jira.example.com/browse/PROJ-456
```

### Configuration

```yaml
jira:
  token: "${JIRA_TOKEN}"        # API token
  email: "user@example.com"     # Email for Cloud auth
  base_url: "https://domain.atlassian.net"  # Optional, auto-detected
  project: "PROJ"               # Default project key for operations
```

### Token Resolution

1. `MEHR_JIRA_TOKEN` environment variable
2. `JIRA_TOKEN` environment variable
3. Token from `config.yaml`

### Authentication

The Jira provider supports different authentication methods based on your Jira deployment:

- **Jira Cloud**: Uses email + API token via Basic Auth. Generate a token at [id.atlassian.com](https://id.atlassian.com/manage-profile/security/api-tokens).
- **Jira Server/Data Center**: Uses PAT (Personal Access Token) or Basic Auth.

### Reference Formats

| Format | Example |
|--------|---------|
| Scheme with key | `jira:JIRA-123` |
| Short scheme | `j:PROJ-456` |
| Issue URL | `jira:https://domain.atlassian.net/browse/JIRA-123` |
| Jira Server URL | `j:https://jira.example.com/browse/PROJ-456` |

### Features

- **Issue Fetching**: Retrieves title, description, status, priority, labels, assignees, attachments
- **List Issues**: JQL-based filtering with status/label support and pagination
- **Comment Support**: Fetch all comments and add new ones
- **Status Updates**: Change issue status via workflow transitions
- **Label Management**: Add and remove labels on issues
- **Issue Creation**: Create new issues with project, priority, type
- **Attachments**: Download file attachments
- **Snapshots**: Export issue content as markdown
- **Auto-Detection**: Base URL automatically detected from issue URLs

### Status Mapping

| Jira Status | Provider Status |
|-------------|-----------------|
| To Do, Backlog, Open, New | Open |
| In Progress, Started, In Development | In Progress |
| In Review, Code Review, Under Review, Verification | Review |
| Done, Closed, Resolved, Complete, Finished | Done |
| Won't Fix, Cancelled, Obsolete, Won't Do | Closed |

### Priority Mapping

| Jira Priority | Provider Priority |
|---------------|------------------|
| Highest, Critical | Critical |
| High | High |
| Medium, Normal, Default | Normal |
| Low, Lowest | Low |

### Workflow Transitions

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

---

## Linear Provider

**Schemes:** `linear:`, `ln:`

**Capabilities:** `read`, `list`, `fetch_comments`, `comment`, `update_status`, `manage_labels`, `create_work_unit`, `snapshot`

Integrates with Linear for modern project management and issue tracking.

### Usage

```bash
# By issue identifier
mehr start linear:ENG-123
mehr plan ln:ENG-123

# By issue URL
mehr start linear:https://linear.app/team/issue/ENG-123-title
```

### Configuration

```yaml
linear:
  token: "${LINEAR_API_KEY}"
  team: "ENG"  # Optional: default team key for operations
```

### Token Resolution

1. `MEHR_LINEAR_API_KEY` environment variable
2. `LINEAR_API_KEY` environment variable
3. Token from `config.yaml`

### Features

- **Issue Fetching**: Retrieves title, description, status, priority, labels, assignees
- **Status Mapping**: Maps Linear states to provider statuses (Backlog/Todo → Open, In Progress → In Progress, Done → Done)
- **Priority Mapping**: Maps Linear priorities (Urgent → Critical, High → High, Medium → Normal, Low → Low)
- **Comment Support**: Fetch and add comments to issues
- **Label Management**: Add and remove labels on issues
- **Status Updates**: Change issue state through workflow
- **Issue Creation**: Create new Linear issues
- **Snapshots**: Export issues as markdown with comments

### Status Mapping

| Linear State | Provider Status |
|--------------|-----------------|
| Backlog, Todo, Unstarted | Open |
| In Progress, Started, In Review | In Progress |
| Done, Completed | Done |
| Canceled, Cancelled | Closed |

### Priority Mapping

| Linear Priority | Provider Priority |
|-----------------|------------------|
| Urgent | Critical |
| High | High |
| Medium | Normal |
| Low | Low |
| No priority | Normal |

---

## Notion Provider

**Schemes:** `notion:`, `nt:`

**Capabilities:** `read`, `list`, `fetch_comments`, `comment`, `update_status`, `manage_labels`, `create_work_unit`, `snapshot`

Integrates with Notion for flexible task management using pages and databases.

### Usage

```bash
# By page ID (32-char UUID, with or without dashes)
mehr start notion:a1b2c3d4e5f678901234567890abcdef1
mehr plan nt:a1b2c3d4-e5f6-7890-1234-567890abcdef1

# By Notion URL
mehr start notion:https://www.notion.so/Page-Title-a1b2c3d4e5f678901234567890abcdef1
```

### Configuration

```yaml
notion:
  token: "${NOTION_TOKEN}"
  database_id: "optional-default-database"  # For list operations
  status_property: "Status"                  # Property name for status
  description_property: "Description"        # Property name for description
  labels_property: "Tags"                    # Multi-select property for labels
```

### Token Resolution

1. `MEHR_NOTION_TOKEN` environment variable
2. `NOTION_TOKEN` environment variable
3. Token from `config.yaml`

### Features

- **Page Fetching**: Retrieves title, content blocks, status, labels, assignees
- **Database Querying**: List pages from databases with status/label filtering
- **Hybrid Approach**: Works with individual pages or database queries
- **Status Mapping**: Maps Notion status/select properties to provider statuses
- **Label Management**: Add/remove multi-select labels
- **Comment Support**: Fetch and add comments to pages
- **Status Updates**: Change page status through configurable property
- **Page Creation**: Create new pages in databases
- **Snapshots**: Export pages as markdown with comments
- **Configurable Properties**: Customize which properties map to status/description/labels

### Status Mapping

| Notion Status | Provider Status |
|---------------|-----------------|
| Not Started, Backlog, Todo | Open |
| In Progress, Started, Doing | In Progress |
| In Review, Reviewing | Review |
| Done, Completed, Finished | Done |
| Cancelled, Archived | Closed |

### Reference Formats

| Format | Example |
|--------|---------|
| Scheme with page ID | `notion:a1b2c3d4e5f678901234567890abcdef1` |
| Short scheme | `nt:a1b2c3d4e5f678901234567890abcdef1` |
| UUID with dashes | `notion:a1b2c3d4-e5f6-7890-1234-567890abcdef1` |
| Notion URL | `notion:https://www.notion.so/Page-Title-a1b2c3d4e5f6...` |

### Property Configuration

The Notion provider maps Notion properties to work unit fields:

| Provider Field | Default Property Name | Notes |
|----------------|----------------------|-------|
| Status | `Status` | Status or select property type |
| Description | `Description` | Falls back to page content blocks |
| Labels | `Tags` | Multi-select property type |
| Assignee | `Assignee` or `Owner` | People property type |

You can customize these property names in your config.

---

## YouTrack Provider

**Schemes:** `youtrack:`, `yt:`

**Capabilities:** `read`, `list`, `fetch_comments`, `comment`, `update_status`, `manage_labels`, `create_work_unit`, `download_attachment`, `snapshot`

Integrates with JetBrains YouTrack for comprehensive issue tracking.

### Usage

```bash
# By issue ID
mehr start youtrack:ABC-123
mehr plan yt:ABC-123

# By bare ID (auto-detected if format matches)
mehr start ABC-123

# By URL
mehr start yt:https://company.myjetbrains.com/youtrack/issue/ABC-123
mehr start yt:https://youtrack.cloud/issue/ABC-123
```

### Configuration

```yaml
youtrack:
  token: "${YOUTRACK_TOKEN}"
  host: "https://company.myjetbrains.com/youtrack"  # Optional: override host
```

### Token Resolution

1. `MEHR_YOUTRACK_TOKEN` environment variable
2. `YOUTRACK_TOKEN` environment variable
3. Token from `config.yaml`

### Reference Formats

| Format | Example |
|--------|---------|
| Scheme with ID | `youtrack:ABC-123` |
| Short scheme | `yt:ABC-123` |
| Bare ID | `ABC-123` (auto-detected) |
| Issue URL | `https://company.myjetbrains.com/youtrack/issue/ABC-123` |

### Features

- **Issue Fetching**: Retrieves title, description, status, priority, tags, assignees, custom fields
- **List Issues**: Query-based filtering with status/tag support and pagination
- **Comment Support**: Fetch all comments and add new ones
- **Tag Management**: Add/remove tags (YouTrack's label equivalent)
- **Status Updates**: Change issue state via custom field update
- **Issue Creation**: Create new issues with project, priority, type
- **Attachments**: Download file attachments
- **Snapshots**: Export issue content as markdown

### State Mapping

| Mehrhof Status | YouTrack States |
|----------------|-----------------|
| `open` | New, Submitted, To be done |
| `in_progress` | In Progress, Active |
| `review` | Code Review, Verification |
| `done` | Fixed, Done, Completed, Verified, Resolved |
| `closed` | Closed, Won't fix, Can't reproduce, Duplicate, Obsolete |

### Priority Mapping

| Mehrhof Priority | YouTrack Priority |
|------------------|-------------------|
| `critical` | Critical, Show-stopper, Urgent |
| `high` | Major, High |
| `normal` | Normal |
| `low` | Minor, Low |

---

## Wrike Provider

**Schemes:** `wrike:`, `wk:`

**Capabilities:** `read`, `list`, `fetch_comments`, `comment`, `download_attachment`, `snapshot`

Integrates with Wrike for enterprise project management.

### Usage

```bash
# By task ID
mehr start wrike:IEAGI2D4I4AL7YNL
mehr plan wk:IEAGI2D4I4AL7YNL

# By permalink
mehr start wrike:https://app.wrike.com/open.htm?id=123456

# By numeric ID (10+ digits)
mehr start wk:1234567890
```

### Configuration

```yaml
wrike:
  token: "${WRIKE_TOKEN}"
  host: "https://www.wrike.com/api/v4"  # Optional: override API base URL
```

### Token Resolution

1. `MEHR_WRIKE_TOKEN` environment variable
2. `WRIKE_TOKEN` environment variable
3. Token from `config.yaml`

### Features

- **Task Fetching**: Retrieves title, description, status, priority, permalink
- **Subtasks**: Recursively fetches nested subtasks (max depth: 5)
- **Comments**: Fetches all comments with automatic pagination support
- **Attachments**: Lists and downloads file attachments
- **Retry Logic**: Automatic exponential backoff for rate limit errors (429)
- **Multiple ID Formats**: Supports numeric IDs, API IDs (`IEAAJ...`), and permalink URLs

---

## Plugin Providers

You can extend Mehrhof with custom providers through the plugin system. See [Plugins](../concepts/plugins.md) for details.

### Installing a Provider Plugin

```bash
# From GitHub
mehr plugins install https://github.com/user/mehrhof-jira

# From local path
mehr plugins install ./plugins/my-provider

# List installed providers
mehr plugins list
```

### Creating a Custom Provider

Provider plugins communicate via JSON-RPC 2.0 over stdin/stdout. See the plugin development guide for implementation details.

---

## Provider Reference Formats

| Provider | Format | Example |
|----------|--------|---------|
| File | `file:path/to/file.md` | `file:tasks/auth.md` |
| Directory | `dir:path/to/directory` | `dir:./tasks` |
| GitHub | `github:N` or `github:owner/repo#N` | `github:123`, `github:owner/repo#456` |
| Jira | `jira:KEY-NUM` or URL | `jira:JIRA-123`, `jira:https://domain.atlassian.net/browse/...` |
| Linear | `linear:TEAM-NUM` or URL | `linear:ENG-123`, `linear:https://linear.app/...` |
| Notion | `notion:page-id` or URL | `notion:a1b2c3d4e5f6...`, `notion:https://notion.so/...` |
| Wrike | `wrike:ID` or permalink | `wrike:IEAGI2D4I4AL7YNL` |
| YouTrack | `youtrack:ABC-123` or URL | `youtrack:ABC-123`, `youtrack:https://...` |

---

## Auto-Detection

Some providers can auto-detect configuration from your environment:

- **GitHub**: Detects `owner/repo` from `git remote origin`
- **File/Directory**: Resolves relative paths from current working directory

---

## Default Provider

Set a default provider to avoid typing scheme prefixes:

```yaml
# .mehrhof/config.yaml
providers:
  default: file  # or github, dir, etc.
```

Now you can use bare references:

```bash
mehr start task.md           # Uses file: provider
mehr start 123               # Uses github: if configured
```
