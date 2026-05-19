# cleanup

Remove stale socket files left behind by crashed processes.

## Usage

```bash
kvelmo cleanup [flags]
```

## Flags

| Flag        | Description                                 |
| ----------- | ------------------------------------------- |
| `--dry-run` | Show what would be removed without deleting |
| `--force`   | Remove sockets without confirmation         |

## Description

Socket files can become stale if kvelmo crashes or is killed without proper shutdown. The `cleanup` command finds and removes these orphaned sockets.

A socket is considered stale if the file exists but no process is listening on it.

## Examples

```bash
# Preview what would be cleaned up
kvelmo cleanup --dry-run

# Clean up with confirmation
kvelmo cleanup

# Clean up without confirmation
kvelmo cleanup --force
```

## Example Output

```bash
kvelmo cleanup --dry-run

Found stale sockets:
  /home/user/.valksor/kvelmo/global.sock (stale)
  /home/user/project/.kvelmo/worktree.sock (stale)

Run without --dry-run to remove these files.
```

## See Also

- [diagnose](/cli/diagnose.md) - Check system requirements
- [serve](/cli/serve.md) - Start the kvelmo server
