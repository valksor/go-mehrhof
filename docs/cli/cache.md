# kvelmo cache

Manage the semantic response cache used for agent prompt deduplication.

## Usage

```bash
kvelmo cache <subcommand>
```

## Subcommands

| Subcommand | Description                                |
| ---------- | ------------------------------------------ |
| `stats`    | Show cache hit/miss rates and entry count  |
| `clear`    | Remove all entries from the response cache |

## Examples

```bash
# View cache statistics
kvelmo cache stats

# Clear the cache
kvelmo cache clear
```

## How It Works

The response cache deduplicates semantically similar agent prompts to avoid redundant API calls. When the agent encounters a prompt similar to one it has already processed, it reuses the cached response instead of making a new request.

## Output

`cache stats` outputs raw JSON with hit/miss rates and entry counts.

## Web UI

Cache statistics and clearing are also available from the web UI.

## Related

- [memory](/cli/memory.md) — Semantic memory store
- [stats](/cli/stats.md) — Task analytics
