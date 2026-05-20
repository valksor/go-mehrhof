# quick

Quick-fix workflow: load, implement, and submit in one step.

## Usage

```bash
kvelmo quick [description]
```

## Options

| Flag     | Description                                                                  |
| -------- | ---------------------------------------------------------------------------- |
| `--from` | Task source                                                                  |
| `--text` | Inline task description                                                      |
| `--skip` | Additional phases to skip (comma-separated, e.g. `--skip simplify,optimize`) |

A description can also be passed as a positional argument.

## Examples

```bash
# Quick fix from GitHub issue
kvelmo quick --from github:owner/repo#123

# Quick fix from file
kvelmo quick --from file:task.md

# Quick fix with inline text (positional arg)
kvelmo quick "Fix typo in README"

# Quick fix skipping simplify and optimize
kvelmo quick --from file:task.md --skip simplify,optimize
```

## What It Does

Skips planning and review phases for trivial fixes where the full lifecycle is overkill. Loads the task, implements it, and submits a PR in one step.

## Related

- [start](/cli/start.md) — Full workflow start
- [implement](/cli/implement.md) — Implementation phase
- [submit](/cli/submit.md) — PR submission
