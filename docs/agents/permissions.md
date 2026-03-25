# Agent Permissions

The permission system controls which operations AI agents can perform during task execution. It detects dangerous operations and enforces environment-specific restrictions.

## How It Works

When an agent invokes a tool (bash command, file write, file edit), the permission system evaluates the operation before it executes:

1. **Danger detection** — classifies the operation as Safe, Caution, or Dangerous
2. **Environment enforcement** — applies additional restrictions based on the configured environment (dev, staging, prod)

## Danger Levels

| Level | Description | Behavior |
|-------|-------------|----------|
| **Safe** | No risk detected | Proceeds without intervention |
| **Caution** | Potentially risky, context-dependent | May prompt for confirmation depending on agent configuration |
| **Dangerous** | Almost always destructive | Blocked or requires explicit approval |

## What Gets Detected

### Bash Commands

**Dangerous** (blocked):
- `rm -rf /` and variants targeting system directories
- `dd` writing to `/dev/` devices
- `mkfs`, `fdisk`, `parted` (disk operations)
- `reboot`, `shutdown`, `halt`, `poweroff`
- Fork bombs
- Direct disk overwrites
- `chmod 777 /` or `chown` on root

**Caution** (flagged):
- `rm -r` (recursive delete)
- `git push --force`, `git reset --hard`, `git clean -f`
- `kill -9`, `killall`, `pkill`
- `sudo`, `doas`, `su`
- Piping curl/wget to shell
- `npm publish`, `docker push`

### File Operations

**Dangerous paths**: `/etc/passwd`, `/etc/shadow`, `/etc/sudoers`, `/proc/`, `/sys/`, `/dev/`, `~/.ssh/`, `~/.gnupg/`

**Caution paths**: `.env` files, files containing `credentials`, `secrets`, `password`, `api_key`, `private_key`

## Environment Enforcement

Configure the environment in `kvelmo.yaml`:

```yaml
agent:
  environment: dev  # dev, staging, or prod
```

| Environment | Effect |
|-------------|--------|
| **dev** | No additional restrictions — danger levels pass through as-is |
| **staging** | Dangerous operations get a staging warning label |
| **prod** | Caution operations are **elevated to Dangerous** (blocked). Dangerous operations get a production label |

## Configuration

The permission system is built into the agent interface and applies automatically. No additional configuration is needed for basic protection.

For custom agents, implement danger detection by calling the `permission.DetectDanger()` function:

```go
import "github.com/valksor/kvelmo/pkg/agent/permission"

result := permission.DetectDanger("bash", map[string]any{
    "command": "rm -rf /tmp/build",
})

if result.Level == permission.Dangerous {
    // Block the operation
    return fmt.Errorf("blocked: %s", result.Reason)
}
```

## Related

- [Custom Agent](/agents/custom.md) — building custom agents that use the permission system
- [Strategies](/agents/strategies.md) — reasoning strategies that affect agent behavior
