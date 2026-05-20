# GitLab

Load tasks from GitLab issues and merge requests.

## Usage

```bash
# From an issue
kvelmo start --from gitlab:group/project#123

# From a merge request
kvelmo start --from gitlab:group/project#456
```

## Authentication

Set your GitLab token:

```bash
export GITLAB_TOKEN=glpat-xxxxxxxxxxxxxxxxxxxx
```

Or in settings:

```json
{
  "providers": {
    "gitlab": {
      "token": "glpat-xxxxxxxxxxxxxxxxxxxx"
    }
  }
}
```

### Creating a Token

1. Go to GitLab → Preferences → Access Tokens
2. Create a token with `api` scope
3. Copy and save the token

## Reference Format

```
gitlab:<project>#<number>
```

Examples:

- `gitlab:group/project#123` — Issue or MR #123
- `gitlab:namespace/subgroup/project#456` — Nested group project

## Extracted Data

| Field       | Source               |
| ----------- | -------------------- |
| Title       | Issue/MR title       |
| Description | Issue/MR description |
| External ID | Issue/MR IID         |
| URL         | GitLab URL           |
| Labels      | Issue/MR labels      |

## Submitting Back

When you run `kvelmo submit`, kvelmo:

1. Creates a merge request with your changes
2. Links to the original issue (if applicable)
3. Adds any configured labels

## Configuration

```json
{
  "providers": {
    "gitlab": {
      "token": "glpat-xxxx",
      "base_url": "https://gitlab.com"
    }
  }
}
```

## Self-Hosted GitLab

For self-hosted GitLab:

```json
{
  "providers": {
    "gitlab": {
      "base_url": "https://gitlab.mycompany.com"
    }
  }
}
```

## Troubleshooting

### "401 Unauthorized"

Your token is invalid or expired. Generate a new one.

### "404 Not Found"

- Check the project path is correct
- Ensure your token has access to the project
- For private projects, verify `api` scope

## Related

- [Providers Overview](/providers/index.md)
- [GitHub Provider](/providers/github.md)
