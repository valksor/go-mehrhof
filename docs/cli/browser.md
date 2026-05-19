# kvelmo browser

Browser automation commands.

## Usage

```bash
kvelmo browser <subcommand>
```

## Subcommands

| Command          | Description           |
| ---------------- | --------------------- |
| `start`          | Start browser session |
| `navigate <url>` | Navigate to URL       |
| `screenshot`     | Take screenshot       |
| `eval <script>`  | Execute JavaScript    |
| `stop`           | Stop browser          |

## Examples

```bash
# Start browser
kvelmo browser start

# Navigate
kvelmo browser navigate https://example.com

# Screenshot
kvelmo browser screenshot --name home

# Stop
kvelmo browser stop
```

## Related

- [Browser Automation](/advanced/browser.md) — Full documentation
