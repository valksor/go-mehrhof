# restore

Restore kvelmo state from a backup archive.

## Usage

```bash
kvelmo restore <archive-path> [flags]
```

## Options

| Flag        | Description                                      |
| ----------- | ------------------------------------------------ |
| `--dry-run` | List contents without extracting                 |
| `--target`  | Target directory (default: `~/.valksor/kvelmo/`) |
| `--json`    | Output as JSON                                   |

## Examples

```bash
# Restore to default location
kvelmo restore backup.tar.gz

# List contents only
kvelmo restore backup.tar.gz --dry-run

# Restore to custom location
kvelmo restore backup.tar.gz --target /tmp/kvelmo
```

## Related

- [backup](/cli/backup.md) — Create backups
