# kvelmo backup

Backup kvelmo state to a tar.gz archive.

## Usage

```bash
kvelmo backup [output-path]
```

## Options

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON |

## What Gets Backed Up

- Configuration files
- Task data and specifications
- Recordings
- Memory store
- Chat history

Excludes transient files (sockets, locks).

## Examples

```bash
# Default backup (kvelmo-backup-<timestamp>.tar.gz)
kvelmo backup

# Custom output path
kvelmo backup /tmp/my-backup.tar.gz

# JSON output for scripting
kvelmo backup --json
```

## Related

- [restore](/cli/restore.md) — Restore from backup
- [config](/cli/config.md) — Configuration management
