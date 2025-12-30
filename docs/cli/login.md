# Provider Login Commands

The `mehr <provider> login` commands provide an interactive way to configure authentication tokens for various providers.

## Supported Providers

| Provider | Command | Environment Variable |
|----------|---------|----------------------|
| GitHub | `mehr github login` | `GITHUB_TOKEN` |
| GitLab | `mehr gitlab login` | `GITLAB_TOKEN` |
| Notion | `mehr notion login` | `NOTION_TOKEN` |
| Jira | `mehr jira login` | `JIRA_TOKEN` |
| Linear | `mehr linear login` | `LINEAR_API_KEY` |
| Wrike | `mehr wrike login` | `WRIKE_TOKEN` |
| YouTrack | `mehr youtrack login` | `YOUTRACK_TOKEN` |

## Usage

```bash
# Configure GitHub token
mehr github login

# Configure Notion token
mehr notion login

# Configure Jira token
mehr jira login
```

## What Happens When You Run Login

1. **Check for existing tokens**: The command checks if a token is already configured in:
   - Environment variables
   - `.mehrhof/.env` file
   - `config.yaml`

2. **Prompt for override**: If a token exists, you'll be asked if you want to replace it.

3. **Enter your token**: Paste your token when prompted.

4. **Save to .env**: The token is saved to `.mehrhof/.env` with secure permissions (`0o600`).

## Example Session

```bash
$ mehr github login

Enter your GitHub API token
Get a token at: https://github.com/settings/tokens
Token will be saved to .mehrhof/.env
Leave empty to cancel: ghp_xxxxxxxxxxxxxxxxxxxx

Token saved to /project/.mehrhof/.env
```

### When Token Already Exists

```bash
$ mehr github login
Token already configured via .mehrhof/.env file: ghp_...abcd
Override? [y/N]: y

Enter your GitHub API token
...
```

## Token Storage

Tokens are stored in `.mehrhof/.env` (which is gitignored) with file permissions `0o600` (read/write for owner only).

Example `.env` file:

```bash
# Mehrhof environment variables
# This file is gitignored - store secrets here safely.

GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxx
NOTION_TOKEN=secret_xxxxxxxxxxxxxxxxxxxxxx
```

## Provider Aliases

Some providers have short aliases:

| Alias | Resolves To |
|-------|-------------|
| `gh` | github |
| `gl` | gitlab |
| `nt` | notion |
| `yt` | youtrack |

```bash
# These are equivalent
mehr github login
mehr gh login
```

## Getting Tokens

### GitHub
Visit: https://github.com/settings/tokens
- Create a **Classic** Personal Access Token
- Required scopes: `repo`, `read:org`

### GitLab
Visit: https://gitlab.com/-/user_settings/personal_access_tokens
- Create a personal access token
- Required scopes: `api`, `read_repository`

### Notion
Visit: https://www.notion.so/my-integrations
- Create a new integration
- Copy the "Internal Integration Token"

### Jira
Visit: https://id.atlassian.com/manage-profile/security/api-tokens
- Create an API token
- For Jira Cloud, use your email as the username

### Linear
Visit: https://linear.app/settings/api
- Create a personal API token
- Requires `read` scope

### Wrike
Visit: https://www.wrike.com/workspace.htm
- Generate an API token in your profile settings

### YouTrack
Visit: https://www.jetbrains.com/help/youtrack/manage-user-token.html
- Create a permanent token in your profile
