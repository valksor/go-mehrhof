# Policy

Check workflow policy compliance for the current task.

## Opening

Open the Policy panel from the sidebar. It loads as a modal overlay.

## Running Checks

Click **Check Policies** to evaluate all configured workflow policies. The panel calls `policy.check` and displays results in a table:

| Column | Description |
|--------|-------------|
| Policy | Name of the policy rule |
| Status | `Pass` or `Fail` — color-coded badge |
| Message | Explanation of the result or reason for failure |

If no policies are configured, an empty state message is shown.

## Refreshing

Click **Check Policies** again at any time to re-evaluate. The panel also auto-loads when opened.

## Related

- [kvelmo policy](/cli/policy.md) — CLI equivalent
- [kvelmo approve](/cli/approve.md) — Approve transitions that policies may gate
