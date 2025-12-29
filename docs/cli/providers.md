# mehr providers

List and manage task providers.

## Synopsis

```bash
mehr providers list
mehr providers info <provider>
```

## Description

Providers are sources for tasks - files, directories, GitHub issues, Jira tickets, etc. The `providers` command helps you discover available providers and understand their configuration requirements.

## Subcommands

### `mehr providers list`

List all available task providers with their schemes and descriptions.

```bash
$ mehr providers list

SCHEME    PROVIDER   DESCRIPTION
------    --------   -----------
file      File       Single markdown file
dir       Directory  Directory with README.md
github    GitHub     GitHub issues and pull requests
gitlab    GitLab     GitLab issues and merge requests
jira      Jira       Atlassian Jira tickets
linear    Linear     Linear issues
notion    Notion     Notion pages and databases
wrike     Wrike      Wrike tasks
youtrack  YouTrack   JetBrains YouTrack issues

Usage:
  mehr start <scheme>:<reference>  # Use provider with scheme

Examples:
  mehr start file:task.md
  mehr start dir:./tasks/
  mehr start github:owner/repo#123
  mehr start jira:PROJECT-123
```

### `mehr providers info <provider>`

Show detailed information about a specific provider, including setup requirements and configuration.

```bash
$ mehr providers info github

Provider: GitHub Provider

Scheme: github

Description:
  Load tasks from GitHub issues and PRs

Required environment variables:
  GITHUB_TOKEN

Configuration (in .mehrhof/config.yaml):
  github:
    token: "${GITHUB_TOKEN}"
    owner: "your-org"
    repo: "your-repo"

Usage:
  mehr start github:owner/repo#123
```

## Provider Aliases

Some providers have short scheme aliases:

| Provider   | Schemes         | Short Alias |
| ---------- | --------------- | ----------- |
| GitHub     | `github:`       | `gh:`        |
| Jira       | `jira:`         | `j:`         |
| Linear     | `linear:`       | `ln:`        |
| Notion     | `notion:`       | `nt:`        |
| YouTrack   | `youtrack:`     | `yt:`        |
| Wrike      | `wrike:`        | `wk:`        |

## Available Providers

### File Provider

Loads tasks from individual markdown files.

```bash
mehr start file:task.md
mehr start file:path/to/task.md
```

No configuration required.

### Directory Provider

Loads tasks from directories containing a README.md file.

```bash
mehr start dir:./tasks/
```

No configuration required.

### GitHub Provider

Loads tasks from GitHub issues and pull requests.

```bash
mehr start github:owner/repo#123
mehr start gh:123
```

**Environment Variables:**
- `GITHUB_TOKEN` (or `MEHR_GITHUB_TOKEN`)

**Configuration:**
```yaml
github:
  token: "${GITHUB_TOKEN}"
  owner: "your-org"
  repo: "your-repo"
```

### Jira Provider

Loads tasks from Atlassian Jira.

```bash
mehr start jira:PROJECT-123
mehr start j:PROJ-456
```

**Environment Variables:**
- `JIRA_TOKEN` (or `MEHR_JIRA_TOKEN`)

**Configuration:**
```yaml
jira:
  url: "https://your-domain.atlassian.net"
  token: "${JIRA_TOKEN}"
```

### Linear Provider

Loads tasks from Linear.

```bash
mehr start linear:ENG-123
mehr start ln:ENG-123
```

**Environment Variables:**
- `LINEAR_API_KEY` (or `MEHR_LINEAR_API_KEY`)

**Configuration:**
```yaml
linear:
  api_key: "${LINEAR_API_KEY}"
```

### Notion Provider

Loads tasks from Notion pages and databases.

```bash
mehr start notion:a1b2c3d4e5f678901234567890abcdef
mehr start nt:a1b2c3d4e5f678901234567890abcdef
```

**Environment Variables:**
- `NOTION_TOKEN` (or `MEHR_NOTION_TOKEN`)

**Configuration:**
```yaml
notion:
  token: "${NOTION_TOKEN}"
```

### Wrike Provider

Loads tasks from Wrike.

```bash
mehr start wrike:1234567890
mehr start wk:1234567890
```

**Environment Variables:**
- `WRIKE_TOKEN` (or `MEHR_WRIKE_TOKEN`)

**Configuration:**
```yaml
wrike:
  token: "${WRIKE_TOKEN}"
```

### YouTrack Provider

Loads tasks from JetBrains YouTrack.

```bash
mehr start youtrack:ABC-123
mehr start yt:ABC-123
```

**Environment Variables:**
- `YOUTRACK_TOKEN` (or `MEHR_YOUTRACK_TOKEN`)

**Configuration:**
```yaml
youtrack:
  host: "https://company.myjetbrains.com/youtrack"
  token: "${YOUTRACK_TOKEN}"
```

## Configuration Priority

For provider tokens and credentials, the resolution priority is:

1. `MEHR_<PROVIDER>_TOKEN` environment variable (highest)
2. `<PROVIDER>_TOKEN` environment variable
3. `.mehrhof/config.yaml` provider configuration (lowest)

## See Also

- [start](start.md) - Start a task from a provider
- [Providers Reference](../reference/providers.md) - Detailed provider documentation
