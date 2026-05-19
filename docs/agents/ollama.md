# Ollama Agent

The Ollama agent connects to a local Ollama server for AI-assisted development using open-source models.

## Prerequisites

1. Install Ollama from [ollama.com](https://ollama.com)
2. Start the server: `ollama serve`
3. Verify: `curl http://localhost:11434/api/tags`

kvelmo automatically pulls missing models when connecting to the agent.

## Configuration

### Setting as Default

```bash
kvelmo config set agent.default ollama
```

Or in `~/.valksor/kvelmo/kvelmo.yaml`:

```yaml
agent:
  default: ollama
```

### Using for Specific Tasks

```bash
kvelmo start --from file:task.md --agent ollama
```

### Model Selection

The default model is `llama3.1`. Configure a different model:

```yaml
agent:
  ollama:
    model: qwen2.5-coder
```

### Custom Server URL

If Ollama runs on a non-default address:

```yaml
agent:
  ollama:
    base_url: http://192.168.1.100:11434
```

## How It Works

The Ollama provider uses Ollama's native `/api/chat` endpoint (not the OpenAI-compatible endpoint) for proper structured tool calling support. Responses are streamed via NDJSON.

## Tool Support

Ollama supports the same standard tools as other agents:

| Tool  | Description            |
| ----- | ---------------------- |
| Read  | Read file contents     |
| Write | Write file contents    |
| Edit  | Edit file with diff    |
| Glob  | Find files by pattern  |
| Grep  | Search file contents   |
| Bash  | Execute shell commands |

## Troubleshooting

### "server not reachable"

Ensure Ollama is running:

```bash
ollama serve
```

### Model not found

kvelmo auto-pulls missing models, but you can pre-pull manually:

```bash
ollama pull llama3.1
```

## Related

- [Agents Overview](/agents/index.md)
- [OpenAI Agent](/agents/openai.md)
- [Custom Agents](/agents/custom.md)
