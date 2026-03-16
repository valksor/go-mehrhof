# kvelmo access

Socket access token management.

## Usage

```bash
kvelmo access token <subcommand>
```

## Subcommands

| Subcommand | Description |
|------------|-------------|
| `token create` | Create a new access token |
| `token revoke <id>` | Revoke an access token |
| `token list` | List all access tokens |

## Options

| Flag | Description |
|------|-------------|
| `--role` | Token role: `operator` or `viewer` (default: `operator`) |
| `--label` | Human-readable label for the token |

## Examples

```bash
# Create a token with operator role
kvelmo access token create --role operator --label "CI pipeline"

# List all tokens
kvelmo access token list

# Revoke a token by ID
kvelmo access token revoke tok_abc123
```

## Related

- [config](/cli/config.md) — Configuration management
