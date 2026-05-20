# screenshots

Screenshot management.

## Usage

```bash
kvelmo screenshots [subcommand]
```

## Subcommands

| Command         | Description         |
| --------------- | ------------------- |
| `list`          | List screenshots    |
| `view <name>`   | View a screenshot   |
| `delete <name>` | Delete a screenshot |

## Examples

```bash
# List screenshots
kvelmo screenshots list

# View
kvelmo screenshots view home-page
```

## Storage

Screenshots are stored in `.kvelmo/screenshots/`.

Also in Web UI: [Dashboard](/web-ui/dashboard.md).

## Related

- [Browser](/cli/browser.md) — Take screenshots
