# Worker Pool

kvelmo uses a worker pool to manage concurrent AI agent executions.

## Overview

The worker pool:

- Manages multiple agent executions
- Queues jobs when workers are busy
- Provides real-time status updates
- Handles job completion and errors

## Configuration

```json
{
  "max_workers": 4,
  "queue_size": 100
}
```

| Option        | Description                | Default |
| ------------- | -------------------------- | ------- |
| `max_workers` | Maximum concurrent workers | 4       |
| `queue_size`  | Maximum jobs in queue      | 100     |

## Viewing Workers

### CLI

```bash
kvelmo workers
```

Output:

```
Workers: 2/4 active
Jobs: 5 queued

Active:
  #1 [project-a] planning - 45s
  #2 [project-b] implementing - 2m30s

Queued:
  #3 [project-c] planning
  #4 [project-d] implementing
  #5 [project-e] reviewing
```

### Web UI

Click **Workers** in the sidebar to see:

- Active workers
- Queued jobs
- Job progress

## Jobs

### Viewing Jobs

```bash
kvelmo jobs
```

### Stopping a Job

```bash
kvelmo stop <job-id>
```

## Worker Events

The worker pool emits events:

| Event           | Description                  |
| --------------- | ---------------------------- |
| `job_started`   | Worker began execution       |
| `job_completed` | Worker finished successfully |
| `job_failed`    | Worker encountered error     |
| `job_queued`    | Job added to queue           |

## Scaling

### More Concurrent Workers

Increase `max_workers` for more parallelism:

```bash
kvelmo config set max_workers 8
```

**Note:** More workers = more resource usage.

### Queue Size

Increase `queue_size` for busy environments:

```bash
kvelmo config set queue_size 200
```

## Resource Management

Workers consume:

- Memory (per agent execution)
- CPU (for agent processing)
- Network (API calls)

Monitor system resources when increasing workers.

## Error Handling

When a worker fails:

1. Job is marked as failed
2. Error is logged
3. Task state reflects the failure
4. Use `kvelmo reset` to recover

## Best Practices

- Start with default settings
- Increase workers gradually
- Monitor resource usage
- Use queue for batch operations
