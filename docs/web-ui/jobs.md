# Jobs

Inspect the job queue and view execution details.

## Opening

Open a **Jobs** tab from the tab bar or the mobile "More" menu. Jobs load automatically when the tab is active.

## Job List

Jobs display as expandable rows showing:

| Column | Description |
|--------|-------------|
| Status | Colored dot — green (completed), orange (running), red (failed), blue (queued) |
| ID | First 12 characters of the job ID |
| Status badge | Current state: completed, running, failed, queued, or pending |
| Type | Job type (e.g., plan, implement) |
| Worktree | Target worktree (truncated) |
| Created | Creation timestamp |

## Job Details

Click a row to expand it. The detail view shows:

- Full job ID, type, status, and last updated timestamp
- Worktree ID (if applicable)
- Error message (shown in red for failed jobs)
- Result object as formatted JSON (scrollable)

## Refreshing

Click the **Refresh** button in the header to reload the job list. The header badge shows the total job count.

## Empty State

When no jobs exist, the panel shows: "Jobs will appear here when tasks are planned or implemented."
