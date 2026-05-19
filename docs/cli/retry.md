# kvelmo retry

Re-run the last failed phase.

## Usage

```bash
kvelmo retry [--phase <phase>]
```

## Description

When a task is in `failed` state, `retry` resets the task and re-runs the phase that failed. The failed phase is inferred from the last error message, or can be overridden with `--phase`.

The command performs three steps:

1. Queries status to confirm the task is in `failed` state
2. Resets the task back to the previous stable state
3. Re-runs the failed phase

## Options

| Flag      | Description                                                         |
| --------- | ------------------------------------------------------------------- |
| `--phase` | Override which phase to retry (plan, implement, simplify, optimize) |

## Phase Inference

When `--phase` is not specified, the phase is inferred from the last error message:

| Error contains | Retries     |
| -------------- | ----------- |
| "implement"    | `implement` |
| "simplify"     | `simplify`  |
| "optimize"     | `optimize`  |
| "plan"         | `plan`      |
| (ambiguous)    | `plan`      |

## Examples

```bash
# Retry with auto-detected phase
kvelmo retry

# Retry a specific phase
kvelmo retry --phase implement

# Retry planning
kvelmo retry --phase plan
```

## Related

- [reset](/cli/reset.md) — Reset task state manually
- [abort](/cli/abort.md) — Abort the current task
- [status](/cli/status.md) — Check current task state
