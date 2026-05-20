# GitHub

Load tasks from GitHub issues and pull requests.

## Usage

```bash
# From an issue
kvelmo start --from github:owner/repo#123

# From a PR
kvelmo start --from github:owner/repo#456
```

## Authentication

Set your GitHub token:

```bash
export GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxx
```

Or in settings:

```json
{
  "providers": {
    "github": {
      "token": "ghp_xxxxxxxxxxxxxxxxxxxx"
    }
  }
}
```

### Creating a Token

1. Go to GitHub → Settings → Developer settings → Personal access tokens
2. Generate a new token with `repo` scope
3. Copy and save the token

## Reference Format

```
github:<owner>/<repo>#<number>
```

Examples:

- `github:valksor/kvelmo#123` — Issue or PR #123
- `github:facebook/react#28000` — React issue/PR

## Extracted Data

| Field       | Source          |
| ----------- | --------------- |
| Title       | Issue/PR title  |
| Description | Issue/PR body   |
| External ID | Issue/PR number |
| URL         | GitHub URL      |
| Labels      | Issue/PR labels |

## Submitting Back

When you run `kvelmo submit`, kvelmo:

1. Creates a PR with your changes
2. Links to the original issue (if applicable)
3. Adds any configured labels

## Configuration

```json
{
  "providers": {
    "github": {
      "token": "ghp_xxxx",
      "default_org": "valksor"
    }
  }
}
```

With `default_org`, you can use shorter references:

```bash
kvelmo start --from github:kvelmo#123
# Expands to github:valksor/kvelmo#123
```

## Enterprise GitHub

For GitHub Enterprise:

```json
{
  "providers": {
    "github": {
      "base_url": "https://github.mycompany.com/api/v3"
    }
  }
}
```

## Troubleshooting

### "401 Unauthorized"

Your token is invalid or expired. Generate a new one.

### "404 Not Found"

- Check the owner/repo/number are correct
- Ensure your token has access to the repository
- For private repos, verify `repo` scope

### Rate Limiting

GitHub API has rate limits. If you hit them:

- Wait for the reset (shown in error)
- Use a token with higher limits

## Related

- [Providers Overview](/providers/index.md)
- [GitLab Provider](/providers/gitlab.md)
