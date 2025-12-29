# Providers

Providers are task sources that Mehrhof can read from and interact with. Each provider supports different capabilities like reading tasks, creating issues, or managing status.

## Available Providers

| Provider | Schemes | Description |
|----------|---------|-------------|
| **File** | `file:` | Local markdown files |
| **Directory** | `dir:` | Local directories with markdown files |
| **GitHub** | `github:`, `gh:` | GitHub issues |
| **Wrike** | `wrike:`, `wk:` | Wrike tasks |

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
| Wrike | `wrike:ID` or permalink | `wrike:IEAGI2D4I4AL7YNL` |

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
