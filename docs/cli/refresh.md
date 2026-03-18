# kvelmo refresh

Check PR status and update local state.

## Usage

```bash
kvelmo refresh
```

## Description

Refresh checks the current PR status and provides guidance on next steps:

- If the PR is **merged**, it suggests running `kvelmo finish`
- If the PR is **still open**, it checks if the branch needs rebasing
- Updates local task state to reflect remote changes

## Related

- [finish](/cli/finish.md) — Clean up after PR merge
- [submit](/cli/submit.md) — Create pull request
- [remote](/cli/remote.md) — Approve or merge PR
