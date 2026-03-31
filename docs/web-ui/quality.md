# Quality Gates

Run and view code quality gate results for the current task.

## Opening

Open the Quality Gates panel from the **Tools** dropdown menu in the project toolbar. It loads as a modal overlay.

## Running Quality Checks

Click **Run** to execute quality gates via the `quality.respond` RPC method. The panel displays findings grouped by severity:

| Severity | Description |
|----------|-------------|
| Error | Must-fix issues that block submission |
| Warning | Should-fix issues flagged for review |
| Info | Informational findings |

Each finding shows the rule name, file path, line number, and message.

## Auto-Fix Status

When auto-fix is active, the panel shows the current attempt number, maximum attempts, and any errors from the last fix attempt.

## Failure Classification

The panel also displays failure classification statistics from the `failclass.stats` RPC, showing how many findings are classified as flaky, genuine, intermittent, or unclassified.

## Related

- [kvelmo quality](/cli/quality.md) — CLI equivalent
- `/quality` — Chat command equivalent
- [Policy](/web-ui/policy.md) — Workflow policy compliance (separate from code quality)
