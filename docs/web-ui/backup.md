# Backup

Create and restore backups of kvelmo state.

## Opening

Open the Backup panel from the sidebar. It loads as a modal overlay.

## Creating a Backup

Click **Create Backup** to archive current kvelmo state. On success, the panel shows the archive path, file size, and number of files included. The backup list refreshes automatically.

## Backup List

Existing backups are listed with:

- **Name** — Archive filename
- **Size** — File size (human-readable)
- **Date** — Creation timestamp
- **Restore button** — Restore from this backup

If no backups exist, the panel shows an empty state prompting you to create one.

## Restoring

Click **Restore** on any backup entry. On success, the panel reports the target directory, number of files and directories restored, and any skipped items.

Restoring replaces current kvelmo state with the archived version. Only one restore can run at a time.
