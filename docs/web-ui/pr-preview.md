# PR Preview

Preview the pull request that will be created when submitting the current task.

## Opening

The PR Preview panel appears as a tab when running `/submit` with dry-run mode or when the submit modal generates a preview. It shows exactly what the PR will look like before creating it.

## Preview Contents

The panel displays:

| Field          | Description                                                |
| -------------- | ---------------------------------------------------------- |
| Title          | PR title generated from the task                           |
| Body           | PR description with summary, specifications, and changelog |
| Branch         | Source branch for the PR                                   |
| Base Branch    | Target branch the PR will merge into                       |
| Diff Stat      | Summary of files changed, insertions, and deletions        |
| Checkpoints    | Number of git checkpoints created during the task          |
| Specifications | Number of specifications attached                          |

## Editing

Click **Edit** to modify the PR body before submission. Changes are preserved and used when the actual PR is created via the submit command.

## Related

- [kvelmo submit](/cli/submit.md) — CLI command with `--dry-run` for preview
- `/submit` — Chat command (opens submit modal with preview option)
