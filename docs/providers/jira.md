# Jira

Load tasks from Jira issues.

## Usage

```bash
# From a Jira issue
kvelmo start --from jira:PROJ-123
```

## Authentication

Set your Jira credentials:

```bash
export JIRA_BASE_URL=https://your-domain.atlassian.net
export JIRA_EMAIL=you@example.com
export JIRA_TOKEN=your-api-token
```

Or in settings:

```yaml
providers:
  jira:
    base_url: https://your-domain.atlassian.net
    email: you@example.com
    token: your-api-token
```

### Creating an API Token

1. Go to [Atlassian Account Settings](https://id.atlassian.com/manage-profile/security/api-tokens)
2. Click "Create API token"
3. Copy and save the token

## Reference Format

```
jira:<issue-key>
```

Examples:

- `jira:PROJ-123` — Issue PROJ-123
- `jira:BACKEND-456` — Issue BACKEND-456

## Extracted Data

| Field       | Source                           |
| ----------- | -------------------------------- |
| Title       | Issue summary                    |
| Description | Issue description                |
| External ID | Issue key                        |
| URL         | Jira URL                         |
| Labels      | Issue labels                     |
| Priority    | Issue priority (mapped to p0-p3) |

## Supported Features

| Feature       | Supported                       |
| ------------- | ------------------------------- |
| Fetch tasks   | Yes                             |
| Update status | Yes (via transitions)           |
| Comments      | Yes                             |
| Labels        | Yes                             |
| Dependencies  | Yes (linked issues)             |
| Hierarchy     | Yes (parent/subtasks)           |
| Create issues | Yes                             |
| Submit PRs    | Yes (adds comment with PR link) |

## Submitting Back

When you run `kvelmo submit`, kvelmo:

1. Creates a PR with your changes
2. Adds a comment on the Jira issue with the PR link
3. Optionally transitions the issue status

## Troubleshooting

### "401 Unauthorized"

Your token or email is incorrect. Verify credentials.

### "JIRA_TOKEN not set"

Set `JIRA_TOKEN` environment variable or configure in settings.

### "404 Not Found"

- Check the issue key format (PROJECT-NUMBER)
- Ensure your account has access to the project

## Related

- [Providers Overview](/providers/index.md)
- [GitHub Provider](/providers/github.md)
- [Linear Provider](/providers/linear.md)
