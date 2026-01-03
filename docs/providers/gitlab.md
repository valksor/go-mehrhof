# GitLab Provider

> **⚠️ Third-Party Integration**: This integration depends on external APIs that may change. Not fully tested beyond unit tests. Behavior may vary depending on the third-party service. Manual validation recommended before production use.


**Schemes:** `gitlab:`, `gl:`

**Capabilities:** `read`, `list`, `fetch_comments`, `comment`, `update_status`, `manage_labels`, `create_work_unit`, `download_attachment`, `snapshot`, `fetch_subtasks`

Interacts with GitLab issues for fully integrated task management. Works with both GitLab.com and self-hosted GitLab instances.

## Usage

```bash
mehr start gitlab:123
mehr plan gl:123

mehr start gitlab:group/project#456
mehr plan gl:group/project#456

mehr start gitlab:12345#456
```

## Configuration

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

## Token Resolution

The GitLab provider tries token sources in this order:

1. `MEHR_GITLAB_TOKEN` environment variable
2. `GITLAB_TOKEN` environment variable
3. Token from `config.yaml`

## Authentication

Create a Personal Access Token in GitLab:
- Navigate to **User Settings → Access Tokens**
- Select scopes: `api`, `read_api`, `read_repository`
- Use the token in your configuration or environment variable

## Reference Formats

| Format | Example |
|--------|---------|
| Scheme with IID | `gitlab:123` |
| Short scheme | `gl:456` |
| Explicit project path | `gitlab:group/project#123` |
| Project ID with IID | `gitlab:12345#678` |

## Features

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

## Task Type Label Mapping

| GitLab Label | Task Type |
|--------------|-----------|
| `bug`, `bugfix`, `fix` | `fix` |
| `feature`, `enhancement` | `feature` |
| `docs`, `documentation` | `docs` |
| `refactor` | `refactor` |
| `chore` | `chore` |
| `test` | `test` |
| `ci` | `ci` |

## Status Mapping

| GitLab State | Provider Status |
|--------------|-----------------|
| `opened` | Open |
| `closed` | Closed |

## Priority Label Mapping

| GitLab Label | Provider Priority |
|--------------|-------------------|
| `critical`, `urgent` | Critical |
| `priority:high`, `high-priority` | High |
| `priority:low`, `low-priority` | Low |
| *(no match)* | Normal |
