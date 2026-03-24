# kvelmo checklist

Manage review checklist items configured in workflow policy.

## Usage

```bash
kvelmo checklist [flags]
```

## Options

| Flag | Description |
|------|-------------|
| `--check` | Mark a checklist item as checked |
| `--uncheck` | Mark a checklist item as unchecked |

## Examples

```bash
# Show checklist status
kvelmo checklist

# Mark an item as checked
kvelmo checklist --check "Tests pass"

# Uncheck an item
kvelmo checklist --uncheck "Tests pass"
```

## Description

Review checklists are configured in `kvelmo.yaml` under `workflow.policy.review_checklist`. All items must be checked before `kvelmo submit` is allowed.

## Web UI

The checklist is also available in the Review panel as interactive checkboxes with a progress bar.

## Related

- [approve](/cli/approve.md) — Approve workflow transitions
- [review](/cli/review.md) — Enter review mode
- [submit](/cli/submit.md) — Submit PR (requires checklist completion)
