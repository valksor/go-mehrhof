# Providers

Providers are task sources that Mehrhof can read from and interact with. Each provider supports different capabilities like reading tasks, creating issues, or managing status.

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

**Capabilities:** `read`, `list`, `fetch_comments`, `comment`, `update_status`, `manage_labels`, `create_work_unit`, `create_pr`, `download_attachment`, `snapshot`, `fetch_subtasks`

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

  # API caching (enabled by default)
  cache:
    disabled: false                # Set to true to disable caching
```

### Caching

The GitHub provider caches API responses to reduce rate limit usage and improve performance. Cached data includes:

- **Issue details** - Cached for 5 minutes
- **Comments** - Cached for 1 minute
- **Repository metadata** (default branch) - Cached for 30 minutes

Cache is automatically invalidated when data is modified (e.g., adding a comment invalidates the comments cache).

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

## GitLab Provider

**Schemes:** `gitlab:`, `gl:`

**Capabilities:** `read`, `list`, `fetch_comments`, `comment`, `update_status`, `manage_labels`, `create_work_unit`, `download_attachment`, `snapshot`, `fetch_subtasks`

Interacts with GitLab issues for fully integrated task management. Works with both GitLab.com and self-hosted GitLab instances.

### Usage

```bash
# By issue number (requires configured project)
mehr start gitlab:123
mehr plan gl:123

# Explicit project path
mehr start gitlab:group/project#456
mehr plan gl:group/project#456

# By project ID
mehr start gitlab:12345#456  # project-id#issue-iid
```

### Configuration

Configure in `.mehrhof/config.yaml`:

```yaml
gitlab:
  token: "${GITLAB_TOKEN}"       # GitLab personal access token
  host: "https://gitlab.com"     # Optional: defaults to gitlab.com
  project_path: "group/project"  # Default project for operations
  branch_pattern: "issue/{key}-{slug}"  # Branch naming
  commit_prefix: "[#{key}]"      # Commit message prefix
```

For self-hosted GitLab instances:

```yaml
gitlab:
  token: "${GITLAB_TOKEN}"
  host: "https://gitlab.example.com"
  project_path: "mygroup/myproject"
```

### Token Resolution

The GitLab provider tries token sources in this order:

1. `MEHR_GITLAB_TOKEN` environment variable
2. `GITLAB_TOKEN` environment variable
3. Token from `config.yaml`

### Authentication

Create a Personal Access Token in GitLab:
- Navigate to **User Settings → Access Tokens**
- Select scopes: `api`, `read_api`, `read_repository`
- Use the token in your configuration or environment variable

### Reference Formats

| Format | Example |
|--------|---------|
| Scheme with IID | `gitlab:123` |
| Short scheme | `gl:456` |
| Explicit project path | `gitlab:group/project#123` |
| Project ID with IID | `gitlab:12345#678` |

### Features

- **Issue Fetching**: Retrieves title, description, labels, assignees, comments
- **Label Inference**: Maps labels to task types (`bug` → `fix`, `feature` → `feature`)
- **Priority Inference**: Extracts priority from label names
- **Linked Issues**: Detects `#123` references in issue description
- **Status Updates**: Close/reopen issues
- **Label Management**: Add or remove labels
- **Issue Creation**: Create new GitLab issues
- **Attachments**: Download file attachments
- **Snapshots**: Export issue content as markdown
- **Self-Hosted Support**: Works with GitLab self-hosted instances

### Task Type Label Mapping

| GitLab Label | Task Type |
|--------------|-----------|
| `bug`, `bugfix`, `fix` | `fix` |
| `feature`, `enhancement` | `feature` |
| `docs`, `documentation` | `docs` |
| `refactor` | `refactor` |
| `chore` | `chore` |
| `test` | `test` |
| `ci` | `ci` |

### Status Mapping

| GitLab State | Provider Status |
|--------------|-----------------|
| `opened` | Open |
| `closed` | Closed |

### Priority Label Mapping

| GitLab Label | Provider Priority |
|--------------|-------------------|
| `critical`, `urgent` | Critical |
| `priority:high`, `high-priority` | High |
| `priority:low`, `low-priority` | Low |
| *(no match)* | Normal |

---

## Jira Provider

**Schemes:** `jira:`, `j:`

**Capabilities:** `read`, `list`, `fetch_comments`, `comment`, `update_status`, `manage_labels`, `create_work_unit`, `download_attachment`, `snapshot`, `fetch_subtasks`

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

**Capabilities:** `read`, `list`, `fetch_comments`, `comment`, `update_status`, `manage_labels`, `create_work_unit`, `snapshot`, `fetch_subtasks`

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

**Capabilities:** `read`, `list`, `fetch_comments`, `comment`, `update_status`, `manage_labels`, `create_work_unit`, `download_attachment`, `snapshot`, `fetch_subtasks`

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

**Capabilities:** `read`, `list`, `fetch_comments`, `comment`, `update_status`, `manage_labels`, `create_work_unit`, `download_attachment`, `snapshot`, `fetch_subtasks`

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

## Trello Provider

**Schemes:** `trello:`, `tr:`

**Capabilities:** `read`, `list`, `fetch_comments`, `comment`, `update_status`, `manage_labels`, `create_work_unit`, `download_attachment`, `snapshot`, `fetch_subtasks`

Integrates with Trello boards and cards for visual project management.

### Usage

```bash
# By card ID (24-character alphanumeric)
mehr start trello:507f1f77bcf86cd799439011
mehr plan tr:507f1f77bcf86cd799439011

# By short link (8-character)
mehr start trello:abc12XYZ
mehr start tr:abc12XYZ

# By card URL
mehr start trello:https://trello.com/c/abc12XYZ/card-name
mehr start tr:https://trello.com/c/abc12XYZ/card-name
```

### Configuration

```yaml
trello:
  api_key: "${TRELLO_API_KEY}"    # Trello API key
  token: "${TRELLO_TOKEN}"         # Trello token
  board: "default-board-id"        # Optional: default board for operations
```

### Token Resolution

1. `MEHR_TRELLO_API_KEY` / `MEHR_TRELLO_TOKEN` environment variables
2. `TRELLO_API_KEY` / `TRELLO_TOKEN` environment variables
3. Keys from `config.yaml`

### Authentication

Generate API credentials at: https://trello.com/app-key

1. Get your API key from the Trello developer page
2. Generate a token with the link provided on that page
3. Set both `api_key` and `token` in your configuration

### Reference Formats

| Format | Example |
|--------|---------|
| Scheme with card ID | `trello:507f1f77bcf86cd799439011` |
| Short scheme | `tr:507f1f77bcf86cd799439011` |
| Short link (8 chars) | `trello:abc12XYZ` |
| Card URL | `trello:https://trello.com/c/abc12XYZ/card-name` |

### Features

- **Card Fetching**: Retrieves card title, description, due date, labels, members, attachments, checklists
- **List Cards**: Browse cards from boards
- **Comment Support**: Fetch comments and add new ones
- **Status Updates**: Move cards between lists to change status
- **Label Management**: Add and remove labels on cards
- **Attachments**: Download file attachments
- **Checklists**: Fetch checklist items and their completion state

### Status Mapping

Status is determined by the card's list name (case-insensitive):

| Trello List | Provider Status |
|-------------|-----------------|
| To Do, Backlog, Todo | Open |
| In Progress, Doing, Started | In Progress |
| In Review, Review, Reviewing | Review |
| Done, Complete, Completed, Finished | Done |
| Archived, Cancelled, Canceled | Closed |

### Moving Cards (Status Change)

To change a card's status, the provider moves it to a list with a matching name:

```bash
# This will move the card to a list named "In Progress" (or similar)
mehr status set in_progress
```

The provider searches for lists with names matching the target status.

---

## Asana Provider

**Schemes:** `asana:`, `as:`

**Capabilities:** `read`, `list`, `fetch_comments`, `comment`, `update_status`, `manage_labels`, `snapshot`, `fetch_subtasks`

Integrates with Asana for task and project management.

### Usage

```bash
# By task GID (16-17 digit number)
mehr start asana:1234567890123456
mehr plan as:1234567890123456
```

### Configuration

```yaml
asana:
  token: "${ASANA_TOKEN}"
  workspace: "123456789"           # Optional: default workspace GID
  default_project: "987654321"     # Optional: default project for listing
```

### Token Resolution

1. `MEHR_ASANA_TOKEN` environment variable
2. `ASANA_TOKEN` environment variable
3. Token from `config.yaml`

### Authentication

Create a Personal Access Token at: https://app.asana.com/0/developer-console

### Features

- **Task Fetching**: Retrieves task name, notes, due dates, assignees, tags, custom fields
- **List Tasks**: Browse tasks from projects with status filtering
- **Comment Support**: Fetch and add stories (comments) to tasks
- **Status Updates**: Move tasks between sections
- **Tag Management**: Add and remove tags
- **Snapshots**: Export task content as markdown

### Status Mapping

Status is determined by the task's section in Asana (case-insensitive):

| Asana Section | Provider Status |
|---------------|-----------------|
| To Do, Backlog, New | Open |
| In Progress, Doing | In Progress |
| In Review, Review | Review |
| Done, Complete | Done |
| Archived | Closed |

---

## ClickUp Provider

**Schemes:** `clickup:`, `cu:`

**Capabilities:** `read`, `list`, `fetch_comments`, `comment`, `update_status`, `manage_labels`, `create_work_unit`, `snapshot`, `fetch_subtasks`

Integrates with ClickUp for comprehensive task management.

### Usage

```bash
# By task ID
mehr start clickup:abc123xyz
mehr plan cu:abc123xyz

# By custom task ID (if enabled)
mehr start cu:TASK-123
```

### Configuration

```yaml
clickup:
  token: "${CLICKUP_TOKEN}"
  workspace: "123456"            # Optional: workspace ID
  default_list: "987654"         # Optional: default list for operations
  custom_task_ids: true          # Enable PROJ-123 format
```

### Token Resolution

1. `MEHR_CLICKUP_TOKEN` environment variable
2. `CLICKUP_TOKEN` environment variable
3. Token from `config.yaml`

### Authentication

Create an API token at: https://app.clickup.com/settings/apps

### Reference Formats

| Format | Example |
|--------|---------|
| Scheme with task ID | `clickup:abc123xyz` |
| Short scheme | `cu:abc123xyz` |
| Custom task ID | `cu:TASK-123` |

### Features

- **Task Fetching**: Retrieves task name, description, status, priority, tags, assignees, due dates
- **List Tasks**: Browse tasks from lists with status filtering
- **Comment Support**: Fetch and add comments
- **Status Updates**: Change task status
- **Tag Management**: Add and remove tags
- **Task Creation**: Create new tasks in lists
- **Snapshots**: Export task content as markdown

### Status Mapping

| ClickUp Status | Provider Status |
|----------------|-----------------|
| to do, open | Open |
| in progress | In Progress |
| in review | Review |
| complete, closed | Done |

---

## Azure DevOps Provider

**Schemes:** `azdo:`, `azure:`

**Capabilities:** `read`, `list`, `fetch_comments`, `comment`, `update_status`, `manage_labels`, `create_work_unit`, `download_attachment`, `create_pr`, `snapshot`, `fetch_subtasks`

Integrates with Azure DevOps for work item tracking and pull request management.

### Usage

```bash
# By work item ID (requires configured org/project)
mehr start azdo:123
mehr plan azure:123

# Explicit organization and project
mehr start azdo:org/project#456

# By URL
mehr start azdo:https://dev.azure.com/org/project/_workitems/edit/123
```

### Configuration

```yaml
azure_devops:
  token: "${AZURE_DEVOPS_TOKEN}"    # Personal Access Token
  organization: "myorg"
  project: "MyProject"
  area_path: "MyProject\\Area"      # Optional: default area path
  branch_pattern: "feature/{key}-{slug}"
  target_branch: "main"
```

### Token Resolution

1. `MEHR_AZURE_DEVOPS_TOKEN` environment variable
2. `AZURE_DEVOPS_TOKEN` environment variable
3. Token from `config.yaml`

### Authentication

Create a Personal Access Token at: https://dev.azure.com/{organization}/_usersSettings/tokens

Required scopes: `Work Items (Read, Write)`, `Code (Read, Write)` for PR creation.

### Reference Formats

| Format | Example |
|--------|---------|
| Scheme with ID | `azdo:123` |
| Short scheme | `azure:456` |
| Org/project with ID | `azdo:org/project#123` |
| URL | `azdo:https://dev.azure.com/org/project/_workitems/edit/123` |

### Features

- **Work Item Fetching**: Retrieves title, description, state, priority, tags, assignees
- **List Work Items**: WIQL query support with status filtering
- **Discussion Support**: Fetch and add comments
- **State Updates**: Change work item state
- **Tag Management**: Add and remove tags
- **Work Item Creation**: Create Bugs, Tasks, User Stories, Features
- **PR Creation**: Create pull requests with automatic work item linking (AB#123 syntax)
- **Snapshots**: Export work item content as markdown

### Status Mapping

| Azure DevOps State | Provider Status |
|--------------------|-----------------|
| New, To Do | Open |
| Active, In Progress, Doing | In Progress |
| Resolved | Review |
| Closed, Done | Done |
| Removed | Closed |

### Priority Mapping

| Azure DevOps Priority | Provider Priority |
|-----------------------|-------------------|
| 1 | Critical |
| 2 | High |
| 3 | Normal |
| 4 | Low |

### Work Item Types

Supported work item types: Bug, Task, User Story, Feature, Epic (configurable by project).

---

## Bitbucket Provider

**Schemes:** `bitbucket:`, `bb:`

**Capabilities:** `read`, `list`, `fetch_comments`, `comment`, `update_status`, `manage_labels`, `create_work_unit`, `download_attachment`, `create_pr`, `snapshot`, `fetch_subtasks`

Integrates with Bitbucket Cloud for issue tracking and pull request management.

### Usage

```bash
# By issue ID (requires configured workspace/repo)
mehr start bitbucket:123
mehr plan bb:123

# Explicit workspace and repository
mehr start bb:workspace/repo#456

# By URL
mehr start bb:https://bitbucket.org/workspace/repo/issues/123
```

### Configuration

```yaml
bitbucket:
  token: "${BITBUCKET_TOKEN}"       # App password
  workspace: "myworkspace"
  repo: "myrepo"
  branch_pattern: "issue/{key}-{slug}"
  target_branch: "main"
  draft_pr: false
```

### Token Resolution

1. `MEHR_BITBUCKET_TOKEN` environment variable
2. `BITBUCKET_TOKEN` environment variable
3. Token from `config.yaml`

### Authentication

Create an App Password at: https://bitbucket.org/account/settings/app-passwords/

Required permissions: `Issues: Read, Write`, `Repositories: Read, Write`, `Pull requests: Write`.

### Reference Formats

| Format | Example |
|--------|---------|
| Scheme with issue ID | `bitbucket:123` |
| Short scheme | `bb:456` |
| Workspace/repo with ID | `bb:workspace/repo#123` |
| Issue URL | `bb:https://bitbucket.org/workspace/repo/issues/123` |

### Features

- **Issue Fetching**: Retrieves title, content, state, priority, assignee
- **List Issues**: Browse repository issues with status filtering
- **Comment Support**: Fetch and add comments
- **State Updates**: Change issue state
- **PR Creation**: Create pull requests
- **Snapshots**: Export issue content as markdown

### Status Mapping

| Bitbucket State | Provider Status |
|-----------------|-----------------|
| new, open | Open |
| on hold | In Progress |
| resolved | Done |
| closed, invalid, duplicate, wontfix | Closed |

### Priority Mapping

| Bitbucket Priority | Provider Priority |
|--------------------|-------------------|
| critical | Critical |
| major | High |
| minor | Normal |
| trivial | Low |

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
