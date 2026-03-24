# kvelmo eventlog

View task lifecycle event log.

## Usage

```bash
kvelmo eventlog [flags]
```

## Description

Query the lifecycle event log for the current task. Events are recorded as the task progresses through phases and include phase transitions, checkpoint creation, finding detection, and other lifecycle milestones.

Unlike the activity log (which tracks RPC calls), the event log captures high-level workflow events written by the conductor.

## Options

| Flag      | Description                                      |
|-----------|--------------------------------------------------|
| `--type`  | Filter by event type (e.g., `phase_started`)     |
| `--phase` | Filter by phase (e.g., `plan`, `implement`)      |
| `--since` | Time range (e.g., `1h`, `30m`)                   |
| `--json`  | Output as JSON                                   |

## Event Types

| Type                  | Description                          |
|-----------------------|--------------------------------------|
| `task_loaded`         | Task loaded from source              |
| `phase_started`       | Phase execution began                |
| `phase_completed`     | Phase finished successfully          |
| `phase_failed`        | Phase failed with error              |
| `checkpoint_created`  | Git checkpoint created               |
| `finding_detected`    | Quality finding detected             |
| `router_decision`     | Failure router made a decision       |
| `spec_changed`        | Specification was modified           |
| `guardrail_checked`   | Policy guardrail was evaluated       |
| `task_finished`       | Task completed                       |

## Examples

```bash
# Show all lifecycle events
kvelmo eventlog

# Show only phase transitions
kvelmo eventlog --type phase_started

# Show events for the implement phase
kvelmo eventlog --phase implement

# Events from the last 30 minutes
kvelmo eventlog --since 30m

# Machine-readable output
kvelmo eventlog --json
```

## Output

```
Lifecycle events (5 total)

  14:20:01  task_loaded               Task loaded from github#42
  14:20:05  phase_started      [plan]  Plan phase started
  14:22:30  phase_completed    [plan]  Plan phase completed
  14:22:31  checkpoint_created [plan]  Checkpoint: plan complete
  14:22:32  phase_started      [implement]  Implement phase started
```

## Related

- [activity](/cli/activity.md) — View RPC activity log (lower-level)
- [status](/cli/status.md) — Show current workflow state
- [checkpoints](/cli/checkpoints.md) — List git checkpoints
