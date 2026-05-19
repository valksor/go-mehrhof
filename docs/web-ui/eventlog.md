# Event Log

View the task lifecycle event log for the current worktree.

## Opening

Open the Event Log panel from the **Tools** dropdown menu in the project toolbar. It loads as a modal overlay.

## Viewing Events

The panel calls `eventlog.query` and displays a chronological list of lifecycle events:

| Event Type         | Description                               |
| ------------------ | ----------------------------------------- |
| phase_started      | A workflow phase began                    |
| phase_completed    | A workflow phase finished                 |
| phase_failed       | A workflow phase encountered an error     |
| checkpoint_created | A git checkpoint was created              |
| finding_detected   | A quality or security finding was flagged |
| task_loaded        | A task was loaded into the worktree       |
| task_finished      | A task completed its lifecycle            |
| router_decision    | The conductor made a routing decision     |
| spec_changed       | A specification was updated               |
| guardrail_checked  | A guardrail check was performed           |

Events are color-coded by type for quick scanning. Each event shows its timestamp, phase, and message.

## Filtering

Filter events by type using the dropdown filter. Click an event to expand its details and view any associated metadata.

## Related

- [kvelmo eventlog](/cli/eventlog.md) — CLI equivalent
- `/eventlog` — Chat command equivalent
- [State Machine](/concepts/state-machine.md) — How phases and transitions work
