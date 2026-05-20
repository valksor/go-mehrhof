# pipe

Run a one-shot prompt through the configured AI agent.

## Usage

```bash
kvelmo pipe [prompt]
```

## Description

Runs a single prompt through the configured AI agent and streams the response to stdout. No running `kvelmo serve` instance is required — the agent is invoked directly.

The prompt can be provided as arguments or piped via stdin.

## Options

| Flag            | Description                                  |
| --------------- | -------------------------------------------- |
| `-a`, `--agent` | Agent to use (claude, codex, or custom name) |
| `--timeout`     | Maximum execution time (default: 10m)        |

## Examples

```bash
# Prompt as argument
kvelmo pipe "summarize the README"

# Pipe from stdin
echo "what files are here?" | kvelmo pipe

# Use specific agent
kvelmo pipe --agent codex "explain this function"

# With timeout
kvelmo pipe --timeout 5m "analyze the codebase"
```

## Agent Resolution

The agent is resolved in this order:

1. `--agent` flag
2. Project configuration
3. Global configuration
4. Auto-detect (finds installed CLI)

## Related

- [chat](/cli/chat.md) — Interactive agent conversation (requires server)
- [config](/cli/config.md) — Configure default agent
