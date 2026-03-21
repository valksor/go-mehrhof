# Hooks

View workflow hooks configured for the current project.

## Opening

Open the Hooks panel from the sidebar. It loads as a modal overlay.

## Viewing Hooks

The panel calls `hooks.list` and groups hooks by event name (e.g., `pre-plan`, `post-implement`). Each hook shows:

| Field | Description |
|-------|-------------|
| Badge | `required` (red) or `optional` (grey) |
| Description | Human-readable description of what the hook does |
| Command | The shell command that runs when the hook fires |

If no hooks are configured, an empty state message is shown.

## Refreshing

Click **Refresh** to reload the hook configuration.

## Related

- [kvelmo hooks](/cli/hooks.md) — CLI equivalent
- [Settings](/web-ui/settings.md) — Configure hooks via settings
