# kvelmo upgrade

Update kvelmo to the latest version from GitHub releases.

## Usage

```bash
kvelmo upgrade [flags]
```

## Options

| Flag | Description |
|------|-------------|
| `--nightly`, `-n` | Install latest available release including pre-releases |
| `--version`, `-v` | Install specific version tag (e.g. `v1.2.3`) |
| `--check` | Check for updates without installing |
| `--yes`, `-y` | Skip confirmation prompt |
| `--skip-verify` | Allow installation when signature verification is unavailable (checksum-only) |

## Update Process

1. Checks for the latest release on GitHub
2. Downloads the checksums file and verifies its Minisign signature
3. Downloads the binary for your platform
4. Verifies SHA256 checksum
5. Replaces the current binary atomically

After a successful update, restart kvelmo:

```bash
kvelmo shutdown && kvelmo serve
```

## Examples

```bash
# Check for updates
kvelmo upgrade --check

# Upgrade to latest stable
kvelmo upgrade

# Upgrade to latest nightly
kvelmo upgrade --nightly

# Install specific version
kvelmo upgrade --version v1.2.3

# Non-interactive upgrade (CI/scripts)
kvelmo upgrade --yes
```

## Notes

- Requires write permission to the binary directory. Use `sudo` if needed.
- `--nightly` and `--version` are mutually exclusive.
- Uses `GITHUB_TOKEN` from `.env` files for authenticated API access (optional — anonymous access works for public repos).

## Related

- [serve](/cli/serve.md) — Start kvelmo server
- [shutdown](/cli/shutdown.md) — Stop kvelmo server
- [diagnose](/cli/diagnose.md) — System diagnostics
