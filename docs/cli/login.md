# Provider Login

Authenticate with task providers by saving API tokens to `.env` files.

## Usage

```bash
kvelmo <provider> login [flags]
```

Supported providers: `github`, `gitlab`, `linear`, `wrike`, `jira`, `azuredevops`

## Description

The login command provides a guided flow for configuring provider authentication:

1. Displays helpful information (token URL, required scopes)
2. Prompts for your API token (input is hidden)
3. Saves the token to a `.env` file

Tokens are stored in `.env` files, not in configuration files, keeping secrets separate from settings.

## Flags

| Flag        | Description                                             |
|-------------|---------------------------------------------------------|
| `--project` | Save token to project `.valksor/.env` instead of global |

## Token Storage

| Scope                 | Location                 |
|-----------------------|--------------------------|
| Global (default)      | `~/.valksor/kvelmo/.env` |
| Project (`--project`) | `.valksor/.env`          |

Project-level tokens override global tokens for that project.

## Examples

### GitHub Login

```bash
kvelmo github login
```

Output:
```
GitHub Token Setup
--------------------------------------------------
Get token: https://github.com/settings/tokens
Steps:     Settings -> Developer settings -> Personal access tokens -> Tokens (classic)
Required:  repo, read:user (or Fine-grained with repository access)
Format:    Token starts with 'ghp_'
--------------------------------------------------

Enter your GitHub API token:

Token saved to ~/.valksor/kvelmo/.env
Use 'kvelmo start <task>' to begin working.
```

### GitLab Login

```bash
kvelmo gitlab login
```

### Linear Login

```bash
kvelmo linear login
```

### Wrike Login

```bash
kvelmo wrike login
```

### Jira Login

```bash
kvelmo jira login
```

### Azure DevOps Login

```bash
kvelmo azuredevops login
```

### Project-Specific Token

```bash
kvelmo github login --project
```

Saves the token to `.valksor/.env` in the current project directory. Useful when different projects need different tokens.

### Override Existing Token

If a token is already configured, you'll be prompted:

```
Token already configured via ~/.valksor/kvelmo/.env: ghp_****abcd
Override? [y/N]:
```

## Required Scopes

| Provider       | Scopes                                                       |
|----------------|--------------------------------------------------------------|
| GitHub         | `repo`, `read:user` (or Fine-grained with repository access) |
| GitLab         | `api`, `read_user`, `read_repository`                        |
| Linear         | Workspace access                                             |
| Wrike          | Default (read/write access)                                  |
| Jira           | Read/write project access                                    |
| Azure DevOps   | Work Items (Read & Write), Code (Read & Write)              |

## Web UI

Provider tokens can also be configured in the Settings modal under the Providers section. Each token field includes a "Get token" link.

## Related

- [config](/cli/config.md) — Manage non-sensitive configuration
- [start](/cli/start.md) — Start working on a task
- [Providers](/providers/index.md) — Provider documentation
