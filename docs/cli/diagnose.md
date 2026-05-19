# diagnose

Check system requirements and configuration status.

## Usage

```bash
kvelmo diagnose
```

## Description

The `diagnose` command checks that all required tools are installed and configured correctly. Use it to troubleshoot connection issues or verify your setup.

## Checks Performed

| Check           | Description                                           |
| --------------- | ----------------------------------------------------- |
| Git             | Verifies git is installed                             |
| Claude CLI      | Checks for Claude CLI installation and authentication |
| Codex CLI       | Checks for Codex CLI installation (optional)          |
| Global socket   | Verifies the kvelmo server is running                 |
| Provider tokens | Lists configured provider authentication              |

## Example Output

```bash
kvelmo diagnose

  Git:           ✓ installed (2.43.0)
  Claude CLI:    ✓ installed (1.2.3)
  Codex CLI:     ✗ not found
  Global socket: ✓ running

  Providers:
    GitHub:  ✓ configured
    GitLab:  ✗ not configured
    Linear:  ✗ not configured
    Wrike:   ✗ not configured

  Next steps:
    • Run 'kvelmo gitlab login' to add GitLab
```

## Exit Codes

| Code | Description                |
| ---- | -------------------------- |
| 0    | All required checks passed |
| 1    | One or more checks failed  |

## See Also

- [cleanup](/cli/cleanup.md) - Remove stale socket files
- [serve](/cli/serve.md) - Start the kvelmo server
- [config](/cli/config.md) - Configuration management
