# GitHub Provider

> **⚠️ Third-Party Integration**: This integration depends on external APIs that may change. Not fully tested beyond unit tests. Behavior may vary depending on the third-party service. Manual validation recommended before production use.


**Schemes:** `github:`, `gh:`

**Capabilities:** `read`, `list`, `fetch_comments`, `comment`, `update_status`, `manage_labels`, `create_work_unit`, `create_pr`, `download_attachment`, `snapshot`, `fetch_subtasks`

Interacts with GitHub issues for fully integrated task management.

## Usage

```bash
# By issue number (auto-detects repo)
mehr start github:123
mehr plan gh:123

# Explicit repository
mehr start github:owner/repo#456
mehr plan gh:owner/repo#456
```

## Configuration

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

## Caching

The GitHub provider caches API responses to reduce rate limit usage and improve performance. Cached data includes:

- **Issue details** - Cached for 5 minutes
- **Comments** - Cached for 1 minute
- **Repository metadata** (default branch) - Cached for 30 minutes

Cache is automatically invalidated when data is modified (e.g., adding a comment invalidates the comments cache).

## Token Resolution

The GitHub provider tries token sources in this order:

1. `MEHR_GITHUB_TOKEN` environment variable
2. `GITHUB_TOKEN` environment variable
3. Token from `config.yaml`
4. GitHub CLI token (`gh auth token`)

## Features

- **Issue Fetching**: Retrieves title, body, labels, assignees, comments
- **Label Inference**: Maps labels to task types (`bug` → `fix`, `feature` → `feature`)
- **Priority Inference**: Extracts priority from label names
- **Linked Issues**: Detects `#123` references in issue body
- **PR Creation**: Automatically creates pull requests after implementation
- **Status Updates**: Close/reopen issues
- **Label Management**: Add or remove labels
- **Issue Creation**: Create new GitHub issues

## Task Type Label Mapping

| GitHub Label | Task Type |
|--------------|-----------|
| `bug`, `bugfix`, `fix` | `fix` |
| `feature`, `enhancement` | `feature` |
| `docs`, `documentation` | `docs` |
| `refactor` | `refactor` |
| `chore` | `chore` |
| `test` | `test` |
| `ci` | `ci` |
