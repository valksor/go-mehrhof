# kvelmo notify

Notification management.

## Usage

```bash
kvelmo notify <subcommand>
```

## Subcommands

| Subcommand | Description                                         |
| ---------- | --------------------------------------------------- |
| `test`     | Send a test notification to all configured webhooks |

## Examples

```bash
# Send test notification
kvelmo notify test
```

## Configuration

Configure webhooks in settings:

```yaml
notify:
  endpoints:
    - url: https://hooks.slack.com/services/T.../B.../xxx
      format: slack
      events: [submitted, failed]
    - url: https://example.com/webhook
      format: json
```

## Related

- [config](/cli/config.md) — Configuration
