# Activity

View the RPC activity log and compliance audit trail.

## Opening

Open the Activity panel from the sidebar. It loads as a modal overlay.

## View Modes

Toggle between three tabs at the top:

### Activity

Shows all JSON-RPC calls made to kvelmo sockets. Each entry displays timestamp, method name, duration, and success/error status.

### Timeline

Human-readable activity feed showing what happened in plain language (e.g., "Agent started planning", "PR submitted") instead of raw RPC method names. Results are ordered newest-first.

### Audit

Compliance-focused view that adds a **User** column to the activity table and shows an **Active Tasks** summary at the top listing each task's ID and current state.

## Filtering

All filters apply to both view modes and update results automatically:

| Filter        | Options                              |
| ------------- | ------------------------------------ |
| Time range    | Last hour, 6 hours, 24 hours, 7 days |
| Errors only   | Checkbox to show only failed calls   |
| Method filter | Free-text search on RPC method name  |

Click **Refresh** to reload data with current filters.

## Export

Click **Export** to download the activity data (tasks + activity entries) as a timestamped JSON file. The export uses the currently selected time range.
