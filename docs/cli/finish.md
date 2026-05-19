# finish

Clean up after a pull request has been merged.

## Usage

```bash
kvelmo finish [flags]
```

## Description

After your PR has been merged, `finish` cleans up local and remote state:

1. Switches to the base branch (main/master)
2. Pulls the latest changes
3. Deletes the local feature branch
4. Optionally deletes the remote feature branch
5. Clears the task state

## Flags

| Flag              | Description                         |
| ----------------- | ----------------------------------- |
| `--delete-remote` | Delete the remote feature branch    |
| `--force`         | Finish even if the PR is not merged |

## Examples

### After PR Merge

```bash
kvelmo finish
```

Output:

```
Task finished!
  Switched to: main
  Deleted local branch: feature/add-auth

Ready for next task. Run 'kvelmo start' to begin.
```

### With Remote Cleanup

```bash
kvelmo finish --delete-remote
```

Deletes both local and remote branches.

### Force Finish

If the PR was closed without merging:

```bash
kvelmo finish --force
```

## Workflow

Typical end-of-task flow:

```bash
kvelmo submit          # Create PR
# ... PR reviewed and merged ...
kvelmo refresh         # Check PR status
kvelmo finish          # Clean up
```

## Related

- [refresh](/cli/refresh.md) — Check PR status before finishing
- [submit](/cli/submit.md) — Create the PR
- [Workflow](/concepts/workflow.md) — Task lifecycle
