# workers

Worker pool status.

## Usage

```bash
kvelmo workers
```

## Output

```
Workers: 2/4 active
Queue: 3 jobs

Active:
  Worker 1: project-a (planning)
  Worker 2: project-b (implementing)
```

## What It Shows

- Active worker count
- Queue size
- Worker assignments

## Configuration

Set max workers:

```bash
kvelmo config set max_workers 8
```

## Related

- [jobs](/cli/jobs.md) — List jobs
- [Workers](/advanced/workers.md) — Worker pool docs
