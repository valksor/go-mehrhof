# jobs

List worker jobs.

## Usage

```bash
kvelmo jobs
```

## Output

```
Active jobs:
  #1 [project-a] planning - 45s
  #2 [project-b] implementing - 2m30s

Queued:
  #3 [project-c] planning
```

## What It Shows

- Job ID
- Project
- Phase
- Duration or queue position

## Related

- [workers](/cli/workers.md) — Worker pool status
- [stop](/cli/stop.md) — Stop a job
