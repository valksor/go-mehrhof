# kvelmo security

Security scanning.

## Usage

```bash
kvelmo security <subcommand>
```

## Subcommands

| Subcommand | Description |
|------------|-------------|
| `scan [dir]` | Scan a directory for security issues |

## Options

| Flag | Description |
|------|-------------|
| `--json` | Output raw JSON response |

## What It Scans

- **Secrets**: Hardcoded AWS keys, GitHub tokens, API keys, private keys, JWTs, passwords
- **Dependencies**: Vulnerable packages via `govulncheck` (if available)

## Severity Levels

Critical, High, Medium, Low, Info

## Examples

```bash
# Scan current directory
kvelmo security scan

# Scan specific directory
kvelmo security scan /path/to/project

# JSON output
kvelmo security scan --json
```

## Related

- [quality](/cli/quality.md) — Code quality
- [policy](/cli/policy.md) — Policy checking
