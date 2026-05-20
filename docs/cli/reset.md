# reset

Recover from failed or stuck state.

## Usage

```bash
kvelmo reset
```

## What Happens

1. State transitions to `loaded`
2. Task data is preserved
3. Checkpoints remain intact

## When to Use

- State is `failed`
- Workflow is stuck
- After `abort`

## Examples

```bash
# After a failure
kvelmo reset
kvelmo status  # State: loaded
kvelmo plan    # Continue workflow
```

## Related

- [abort](/cli/abort.md) — Causes failed state
- [abandon](/cli/abandon.md) — Full cleanup instead
