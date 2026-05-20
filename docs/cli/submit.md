# submit

Create a PR and submit to the provider.

## Usage

```bash
kvelmo submit
```

## Options

| Flag              | Short | Description                                                    |
| ----------------- | ----- | -------------------------------------------------------------- |
| `--title`         | `-t`  | PR/MR title (defaults to task title)                           |
| `--body`          | `-b`  | PR/MR body (defaults to task description)                      |
| `--draft`         |       | Create as draft PR                                             |
| `--reviewers`     |       | Assign reviewers (comma-separated)                             |
| `--labels`        |       | Add labels (comma-separated)                                   |
| `--delete-branch` |       | Delete local branch after successful submission                |
| `--skip-review`   |       | Skip review gate and submit directly                           |
| `--dry-run`       |       | Preview the PR without creating it                             |
| `--section`       |       | Add custom PR section (format: `"Header=Content"`, repeatable) |
| `--json`          |       | Output result as JSON                                          |

## Prerequisites

- Task must be in `reviewing` or `submitted` state (re-submit after re-entry)
- Run `kvelmo review` first (or use `--skip-review`)
- All review checklist items must be checked (if configured)
- Transition must be approved via `kvelmo approve submit` (if approval is required)
- Documentation requirements must be met (if configured)

## Examples

```bash
# Submit PR
kvelmo submit

# Preview PR without creating it
kvelmo submit --dry-run

# Submit with custom section
kvelmo submit --section "Test Plan=Manually tested login flow"

# Submit as draft with reviewers
kvelmo submit --draft --reviewers alice,bob
```

## What Happens

1. Review checklist and approval gates are verified
2. Changelog entry is appended (if `storage.changelog_path` is set)
3. Repo PR template is detected and auto-filled (if present)
4. Changes are pushed to remote
5. A PR is created with the configured title pattern
6. Task status is synced to the ticket system (if `status_sync` is enabled)
7. State transitions to `submitted`

## Output

```
PR created: https://github.com/owner/repo/pull/123
State: submitted
```

## Provider Integration

For GitHub/GitLab tasks:

- PR is linked to the original issue
- Labels may be applied
- Assignees may be set

For file tasks:

- PR is created with task title
- Description includes task details

## After Submission

From the `submitted` state, you can:

- `kvelmo finish` — Clean up branch and return to ready state
- `kvelmo plan` — Re-plan (new commits push to the existing PR)
- `kvelmo implement` — Re-implement (new commits push to the existing PR)
- `kvelmo review` — Re-review the current changes
- Monitor the PR in your provider

Also in Web UI: [Review Phase](/web-ui/reviewing.md).

## Related

- [review](/cli/review.md) — Review before submitting
- [approve](/cli/approve.md) — Approve gated transitions
- [checklist](/cli/checklist.md) — Manage review checklist
- [Workflow](/concepts/workflow.md) — Complete workflow
