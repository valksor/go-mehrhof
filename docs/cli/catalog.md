# catalog

Task template catalog.

## Usage

```bash
kvelmo catalog <subcommand>
```

## Subcommands

| Subcommand   | Description                             |
| ------------ | --------------------------------------- |
| `list`       | List available templates                |
| `use <name>` | Start a task from a template            |
| `add <path>` | Import a template file into the catalog |

## Built-in Templates

| Template   | Description        |
| ---------- | ------------------ |
| `bug-fix`  | Fix reported bugs  |
| `feature`  | Implement features |
| `refactor` | Code refactoring   |

## Examples

```bash
# List available templates
kvelmo catalog list

# Start a task from a template
kvelmo catalog use bug-fix

# Import a custom template
kvelmo catalog add ./my-template.yaml
```

## Custom Templates

Templates are YAML files stored in `~/.valksor/kvelmo/templates/`. Custom templates override built-in ones with the same name.

## Related

- [start](/cli/start.md) — Start a task
- [queue](/cli/queue.md) — Queue management
