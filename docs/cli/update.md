# mehr update

Update mehr to the latest version from GitHub releases.

## Synopsis

```bash
mehr update [flags]
```

## Description

The `update` command checks for new releases on GitHub and updates the installed binary to the latest version. It supports:

- Automatic platform detection (linux/darwin, amd64/arm64)
- Optional SHA256 checksum verification
- Pre-release version updates
- Atomic binary replacement (no downtime)

## How It Works

1. **Check** - Queries GitHub releases API for the latest version
2. **Download** - Downloads the platform-specific binary to a temporary file
3. **Verify** - Verifies SHA256 checksum if available
4. **Install** - Atomically replaces the current binary

On Unix systems (Linux/macOS), the binary replacement is atomic using `os.Rename()`. The running process continues using the old binary, while new invocations use the updated version.

## Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-p, --pre-release` | Include pre-release versions | `false` |
| `--check` | Check for updates without installing | `false` |
| `-y, --yes` | Skip confirmation prompt | `false` |

## Examples

### Update to Latest Stable

```bash
mehr update
```

Output:

```
→ Checking for updates...

✓ Update available
  Current:   v1.0.0
  Latest:    v1.1.0
  Release:   https://github.com/valksor/go-mehrhof/releases/tag/v1.1.0
  Download:  mehrhof-linux-amd64 (15.2 MB)

Download and install v1.1.0? [y/N]: y
⠋ Downloading update
✓ Download complete
⠋ Installing update
✓ Installation complete

✓ Updated to v1.1.0
→ Restart mehr to use the new version
```

### Check Without Installing

```bash
mehr update --check
```

### Include Pre-Releases

```bash
mehr update --pre-release
```

### Skip Confirmation

```bash
mehr update -y
```

## Automatic Update Checks

By default, mehr checks for updates on startup (once every 24 hours). If an update is available, you'll see a notification:

```
→ v1.1.0 is available (you have v1.0.0)
→ Run 'mehr update' to install
```

The automatic check:
- Runs in the background (doesn't slow startup)
- Times out after 5 seconds
- Only notifies, doesn't auto-install
- Is skipped for dev builds

### Disabling Automatic Checks

To disable automatic update checks, add to `.mehrhof/config.yaml`:

```yaml
update:
  enabled: false
```

### Adjusting Check Interval

To change how often checks occur (default: 24 hours):

```yaml
update:
  check_interval: 168  # Check every week instead
```

## Permissions

If mehr is installed in a system directory like `/usr/local/bin`, you may need write permissions:

```bash
sudo mehr update
```

The command will detect if the directory is not writable and prompt you.

## Release Assets

The update command looks for binaries with these exact names:

```
mehrhof-linux-amd64
mehrhof-linux-arm64
mehrhof-darwin-amd64
mehrhof-darwin-arm64
checksums.txt  # Optional SHA256 checksums
```

## Authentication

GitHub token resolution (for higher rate limits):

1. `MEHR_GITHUB_TOKEN` environment variable
2. `GITHUB_TOKEN` environment variable
3. `gh auth token` (GitHub CLI)
4. Anonymous access (works for public repos, rate-limited)

## Dev Builds

Update checks are disabled for dev builds (when version is "dev"). To test the update mechanism during development, build with a version tag:

```bash
make build
```

## See Also

- [Quickstart](../quickstart.md) - Installing Mehrhof
- [Configuration](../configuration/index.md) - Configuring update behavior
- [CLI Overview](index.md) - All available commands
