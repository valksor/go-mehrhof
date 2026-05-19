# Diagnose

Check system health and identify configuration issues.

## Opening

Click the shield icon in the top-right of the Global View header. The panel opens as a modal overlay.

## System Checks

The panel runs preflight checks automatically when opened:

| Check         | What It Verifies                |
| ------------- | ------------------------------- |
| Git           | Git is installed and accessible |
| Claude CLI    | Claude CLI binary is available  |
| Claude Auth   | Claude CLI is authenticated     |
| Codex CLI     | Codex CLI binary is available   |
| Global Socket | Global socket server is running |

Each check shows a status badge (OK, Warning, or Failed) with a detail message. Failed checks include a fix suggestion.

## Provider Tokens

Lists each configured task provider and whether its authentication token is set. Providers show as **Configured** or **Not configured**.

## Next Steps

If any checks fail, a summary section lists the issues that need attention. This section is hidden when all checks pass.

## Re-running

Click **Re-run** to refresh all diagnostics. The panel shows an "All checks passed" alert when everything is healthy, or warns of the number of issues found.
