# stop

Stop a running job.

## Usage

```bash
kvelmo stop <job-id>
```

## Examples

```bash
# List jobs to find ID
kvelmo jobs

# Stop job #1
kvelmo stop 1
```

## What Happens

1. Job execution stops
2. Worker becomes available
3. Task may need reset

## When to Use

- Job is taking too long
- Want to cancel execution
- Need to free up workers

## Related

- [jobs](/cli/jobs.md) — List jobs
- [workers](/cli/workers.md) — Worker status
