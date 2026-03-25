# Forks

The Forks feature lets you try multiple implementation approaches in parallel and pick the best one.

## Creating Forks

From the project view, open the Fork tab to create new forks. Each fork branches from the current checkpoint into a separate git worktree where the agent works independently.

## Comparing Forks

The Fork Comparison panel shows all active forks side-by-side with:

- Files added and modified
- Lines added and removed
- Diff statistics per fork
- Current state of each fork

## Selecting a Winner

After comparing approaches, select the winning fork to merge its changes back into the main worktree. The other forks are cleaned up automatically.

## CLI Equivalent

All fork operations are also available via CLI:

```bash
kvelmo fork create "approach-name"
kvelmo fork list
kvelmo fork compare
kvelmo fork select <fork-id>
```

See [fork CLI reference](/cli/fork.md) for details.

## Related

- [Reviewing](/web-ui/reviewing.md) — Review implementation
- [Dashboard](/web-ui/dashboard.md) — Project overview
