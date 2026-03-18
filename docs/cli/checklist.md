# kvelmo checklist

Manage review checklist items configured in workflow policy.

## Usage

```bash
kvelmo checklist                  # Show checklist status
kvelmo checklist --check <item>   # Mark item as checked
kvelmo checklist --uncheck <item> # Mark item as unchecked
```

## Description

The checklist command displays and manages review checklist items defined in your workflow policy. Checklist items must be checked before certain workflow transitions (e.g., review to submit).

## Flags

| Flag              | Description                    |
|-------------------|--------------------------------|
| `--check <item>`  | Mark a checklist item as done  |
| `--uncheck <item>`| Mark a checklist item as undone|

## Related

- [approve](/cli/approve.md) — Approve workflow transitions
- [review](/cli/review.md) — Review implementation
- [policy](/cli/policy.md) — Workflow policy checking
