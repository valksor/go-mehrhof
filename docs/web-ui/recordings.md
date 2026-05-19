# Recordings

View agent session recordings to review what happened during task execution.

## Opening

Click the recording icon in the top-right of the Global View header. The panel opens as a modal overlay.

## Recording List

Each recording entry shows:

- **Job ID** — Truncated identifier
- **Agent** — Which agent ran the session
- **Model** — AI model used (if available)
- **Lines** — Number of recorded events
- **Date** — Session start time
- **Path** — Recording file path

## Viewing Details

Click a recording row to expand it. The detail view shows:

- Header info: Job ID, Agent, Model, Work directory
- Records table with columns:

| Column    | Description                                 |
| --------- | ------------------------------------------- |
| Time      | Event timestamp (HH:MM:SS)                  |
| Direction | `<-` for incoming, `->` for outgoing        |
| Type      | Event type badge                            |
| Data      | Event payload (truncated to 200 characters) |

Click the expanded row again to collapse it.

## Filtering

Enter a job ID in the filter field and press Enter or click **Filter** to show only recordings for that job.
