# approve

Explicitly approve a workflow transition that requires human approval.

## Usage

```bash
kvelmo approve <event>
```

## Prerequisites

- Task must be loaded
- The event must be configured in `workflow.policy.approval_required`

## Examples

```bash
# Approve the submit transition
kvelmo approve submit

# Approve implementation (if configured)
kvelmo approve implement
```

## Configuration

Enable approval gates in project settings:

```yaml
workflow:
  policy:
    approval_required:
      submit: true # Require approval before submitting PR
      implement: true # Require approval before implementation
```

When a transition requires approval, the normal command will fail with a message directing you to run `kvelmo approve <event>` first.

## How It Works

1. Configure which transitions require approval in settings
2. When you try to submit (or other gated transition), kvelmo blocks it
3. Run `kvelmo approve submit` to grant approval
4. Now `kvelmo submit` succeeds

Approvals are timestamped and persisted with the task state.

## Related

- [submit](/cli/submit.md) — Submit after approval
- [checklist](/cli/checklist.md) — Review checklist management
- [Configuration](/configuration/settings.md) — Policy settings
