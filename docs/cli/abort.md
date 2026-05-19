# kvelmo abort

Abort the current task and transition to failed state.

## Usage

```bash
kvelmo abort
```

## What Happens

1. Current execution stops
2. State transitions to `failed`
3. Use `reset` to recover

## When to Use

- Task is taking too long
- Agent is stuck
- Want to stop immediately

## Recovery

After abort, use reset to recover:

```bash
kvelmo reset
```

## Related

- [reset](/cli/reset.md) — Recover after abort
- [abandon](/cli/abandon.md) — Full cleanup
