# remote

Remote provider operations (approve, merge PR/MR).

## Usage

```bash
kvelmo remote <subcommand>
```

## Description

Commands for interacting with the remote provider (GitHub, GitLab) after submitting a PR/MR.

## Subcommands

### approve

Approve the pull request or merge request associated with the current task.

```bash
kvelmo remote approve
```

| Flag              | Description                      |
| ----------------- | -------------------------------- |
| `-c`, `--comment` | Comment to include with approval |

### merge

Merge the pull request or merge request associated with the current task.

```bash
kvelmo remote merge
```

| Flag             | Description                                           |
| ---------------- | ----------------------------------------------------- |
| `-m`, `--method` | Merge method: merge, squash, rebase (default: rebase) |

## Examples

```bash
# Approve PR
kvelmo remote approve

# Approve with comment
kvelmo remote approve --comment "LGTM!"

# Merge with default method (rebase)
kvelmo remote merge

# Squash merge
kvelmo remote merge --method squash
```

## Related

- [submit](/cli/submit.md) — Create and submit PR
- [finish](/cli/finish.md) — Clean up after merge
- [refresh](/cli/refresh.md) — Check PR status
