# CI Status

View CI pipeline check results for the current task's branch.

## Opening

Open the CI Status panel from the sidebar. It loads as a modal overlay.

## Viewing Checks

The panel calls `ci.status` and displays a table of CI checks:

| Column | Description |
|--------|-------------|
| Check | Name of the CI check (e.g., `build`, `lint`, `test`) |
| Status | `Pass`, `Fail`, or `Pending` — color-coded badge |
| View | Link to the check's detail page (when available) |

If no CI data is available (no PR submitted, or provider does not support status checks), an empty state message is shown.

## Refreshing

Click **Refresh** to re-fetch CI status from the provider. The panel also auto-loads when opened.

## Related

- [kvelmo ci](/cli/ci.md) — CLI equivalent
- [Reviewing](/web-ui/reviewing.md) — Review workflow that may depend on CI passing
