# Gemini Agent

> **⚠️ Third-Party Integration**: This integration depends on external APIs that may change. Not fully tested beyond unit tests. Behavior may vary depending on the third-party service. Manual validation recommended before production use.


Gemini provides access to Google's Gemini AI models via the official Gemini CLI.

## Prerequisites

- Gemini CLI installed (https://github.com/google-gemini/gemini-cli)
- Google AI API key

```bash
npm install -g @anthropic-ai/gemini-cli

export GEMINI_API_KEY="your-api-key"

gemini --version
```

## Key Features

- **Large context**: 1M token context window
- **Free tier**: 60 requests/min, 1000 requests/day
- **Streaming**: Full streaming support with JSON output
- **Tool use**: Built-in tool and function calling support
- **Google Search grounding**: Optional web search integration

**Default Model:** `gemini-2.5-pro`

## Configuration

Use as default:

```yaml
# .mehrhof/config.yaml
agent:
  default: gemini
```

Or specify via CLI:

```bash
mehr start --agent gemini file:task.md
```

## Available Models

| Model | Description |
|-------|-------------|
| `gemini-2.5-pro` | Latest pro model (default) |
| `gemini-2.5-flash` | Fast, cost-effective |
| `gemini-3-pro` | Experimental next-gen model |

## Aliases

Create custom configurations:

```yaml
# .mehrhof/config.yaml
agents:
  gemini-flash:
    extends: gemini
    description: "Gemini Flash for quick tasks"
    args: ["-m", "gemini-2.5-flash"]

  gemini-grounded:
    extends: gemini
    description: "Gemini with Google Search grounding"
    args: ["--grounding"]
```

## Troubleshooting

### "gemini CLI not found"

Ensure the Gemini CLI is installed and in your PATH:

```bash
which gemini
gemini --version
```

### "API key not configured"

Set the `GEMINI_API_KEY` environment variable or add it to `.mehrhof/.env`:

```
GEMINI_API_KEY=your-api-key
```
