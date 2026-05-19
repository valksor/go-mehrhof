# OpenAI Agent

The OpenAI agent uses OpenAI's chat completions API for AI-assisted development.

## Prerequisites

Add your API key to the global `.env` file (`~/.valksor/kvelmo/.env`):

```
OPENAI_API_KEY=sk-...
```

## Configuration

### Setting as Default

```bash
kvelmo config set agent.default openai
```

Or in `~/.valksor/kvelmo/kvelmo.yaml`:

```yaml
agent:
  default: openai
```

### Using for Specific Tasks

```bash
kvelmo start --from file:task.md --agent openai
```

### Model Selection

The default model is `gpt-4o`. Configure a different model:

```yaml
agent:
  openai:
    model: gpt-4.1
```

### Custom Base URL

For OpenAI-compatible APIs (Azure, local proxies):

```yaml
agent:
  openai:
    base_url: https://my-proxy.example.com
```

## How It Works

The OpenAI provider uses the `/v1/chat/completions` endpoint with SSE streaming. Tool calls are accumulated across streaming chunks and flushed when the response completes.

## Tool Support

OpenAI supports the same standard tools as other agents:

| Tool  | Description            |
| ----- | ---------------------- |
| Read  | Read file contents     |
| Write | Write file contents    |
| Edit  | Edit file with diff    |
| Glob  | Find files by pattern  |
| Grep  | Search file contents   |
| Bash  | Execute shell commands |

## Troubleshooting

### "OPENAI_API_KEY not configured"

Add the key to your `.env` file:

```
# ~/.valksor/kvelmo/.env
OPENAI_API_KEY=sk-...
```

The API key is loaded from `.env` files, not from shell environment variables or `kvelmo.yaml`.

## Related

- [Agents Overview](/agents/index.md)
- [Ollama Agent](/agents/ollama.md)
- [Codex Agent](/agents/codex.md)
