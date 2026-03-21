# Access Tokens

Create and manage access tokens for the kvelmo socket API.

## Opening

Open the Access panel from the sidebar. It loads as a modal overlay.

## Creating Tokens

Use the form at the top of the panel:

| Field | Description |
|-------|-------------|
| Label | A human-readable name for the token |
| Role | `Operator` (full access) or `Viewer` (read-only) |

Click **Create Token** to generate a new token. The token value is displayed once in a success banner — copy it immediately, as it cannot be retrieved later.

## Viewing Tokens

Existing tokens appear in a table:

| Column | Description |
|--------|-------------|
| ID | Truncated token identifier |
| Label | The label assigned at creation |
| Role | `operator` or `viewer` badge |
| Created | Date the token was created |
| Revoke | Button to permanently revoke the token |

## Revoking Tokens

Click **Revoke** on any token row. A confirmation dialog appears before the token is revoked. Revocation is permanent and cannot be undone.

## Related

- [kvelmo access](/cli/access.md) — CLI equivalent
