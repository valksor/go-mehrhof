# review

Start the review phase for implemented changes.

## Usage

```bash
kvelmo review
```

## Options

| Flag        | Short | Description                                   |
| ----------- | ----- | --------------------------------------------- |
| `--approve` |       | Immediately approve (skip interactive review) |
| `--reject`  |       | Reject and return to planning state           |
| `--message` | `-m`  | Review message/notes                          |
| `--fix`     |       | Auto-fix issues after entering review state   |
| `--force`   |       | Re-run review even if already reviewed        |
| `--wait`    | `-w`  | Wait for job to complete, streaming output    |

## Subcommands

| Subcommand        | Description                       |
| ----------------- | --------------------------------- |
| `review list`     | List all reviews for current task |
| `review view <N>` | View review number N              |

## Prerequisites

- Task must be in `implemented`, `planned`, or `submitted` state
- From `implemented`: standard review after implementation
- From `planned`: only available after at least one implementation has run
- From `submitted`: re-review after re-entry (new commits on existing PR)

## Examples

```bash
# Start review
kvelmo review
```

## What Happens

1. State transitions to `reviewing`
2. Security scanning runs (if configured)
3. Quality gate checks run
4. You review the changes
5. Complete the review checklist (if configured): `kvelmo checklist --check <item>`
6. Approve the submit transition (if required): `kvelmo approve submit`
7. Decide to submit or undo

## Reviewing Changes

View the changes:

```bash
git diff
```

## Approving

If satisfied, submit:

```bash
kvelmo submit
```

## Rejecting

If changes aren't right:

```bash
kvelmo undo
```

Then adjust and re-implement.

Also in Web UI: [Review Phase](/web-ui/reviewing.md).

## Related

- [implement](/cli/implement.md) — Implement before reviewing
- [checklist](/cli/checklist.md) — Manage review checklist
- [approve](/cli/approve.md) — Approve gated transitions
- [submit](/cli/submit.md) — Submit after approval
- [undo](/cli/undo.md) — Revert if needed
