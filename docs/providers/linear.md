# Linear Provider

Load tasks from [Linear](https://linear.app) issues.

## Reference Format

```
linear:<identifier>
```

Where `<identifier>` is the issue identifier like `ENG-123` or `TEAM-456`.

## Example

```bash
kvelmo start --from linear:ENG-123
```

## Authentication

Set the `LINEAR_TOKEN` environment variable:

```bash
export LINEAR_TOKEN=lin_api_xxxx
```

Or configure in settings:

```json
{
  "providers": {
    "linear": {
      "token": "lin_api_xxxx",
      "team": "ENG"
    }
  }
}
```

The `team` field sets a default team key for listing and creating issues.

## Features

### Task Fetching

Fetches full issue data including:

- Title and description
- Labels and priority
- Workflow state (backlog, started, completed, etc.)
- Parent/child relationships
- Assignee

### Hierarchy Support

Linear supports parent-child issue relationships. When loading a sub-issue, kvelmo fetches:

- **Parent context** — The parent issue title and description
- **Sibling context** — Other children of the same parent (up to 5)

This context enriches AI prompts with organizational awareness.

### Status Updates

Updates issue status after submission. Maps kvelmo statuses to Linear workflow states:

| kvelmo Status             | Linear State Type  |
| ------------------------- | ------------------ |
| `open`, `todo`, `backlog` | backlog, unstarted |
| `in_progress`, `doing`    | started            |
| `done`, `completed`       | completed          |
| `canceled`                | canceled           |

### Comments

Add comments to issues during the workflow:

```bash
kvelmo chat --comment "Implementation complete, ready for review"
```

### Labels

Labels from Linear issues are imported and available during the workflow.

### Attachments

Attachments from Linear are downloadable for context gathering. Supported hosts:

- `uploads.linear.app`
- `cdn.linear.app`
- Google Cloud Storage (Linear's CDN buckets)

## Priority Mapping

Linear's priority levels (1-4) map to kvelmo priorities:

| Linear Priority | kvelmo Priority |
| --------------- | --------------- |
| 1 (Urgent)      | critical        |
| 2 (High)        | high            |
| 3 (Medium)      | normal          |
| 4 (Low)         | low             |

## Sub-issues as Subtasks

Child issues appear as subtasks in the task view. Their completion status reflects their Linear workflow state (completed/canceled = done).

## Related

- [Providers Overview](/providers/index.md)
- [GitHub Provider](/providers/github.md)
- [Configuration](/configuration/index.md)
