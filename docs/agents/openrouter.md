# OpenRouter Agent

> **⚠️ Third-Party Integration**: This integration depends on external APIs that may change. Not fully tested beyond unit tests. Behavior may vary depending on the third-party service. Manual validation recommended before production use.


OpenRouter provides unified access to 100+ AI models through a single API. Useful for accessing models from OpenAI, Anthropic, Google, Meta, and others.

## Prerequisites

- OpenRouter API key (get one at https://openrouter.ai/keys)

```bash
export OPENROUTER_API_KEY="sk-or-..."
```

## Key Features

- **Model variety**: Access to Claude, GPT-4, Gemini, Llama, and many more
- **Cost optimization**: Choose models based on price/performance
- **Fallback support**: Configure backup models
- **Streaming**: Full streaming support for real-time responses

**Default Model:** `anthropic/claude-3.5-sonnet`

## Configuration

```yaml
# .mehrhof/config.yaml
agents:
  openrouter-gpt4:
    extends: openrouter
    description: "OpenRouter with GPT-4"
    args: ["--model", "openai/gpt-4-turbo"]

  openrouter-gemini:
    extends: openrouter
    description: "OpenRouter with Gemini"
    args: ["--model", "google/gemini-pro-1.5"]

  openrouter-llama:
    extends: openrouter
    description: "OpenRouter with Llama 3.1"
    args: ["--model", "meta-llama/llama-3.1-405b-instruct"]
```

## Popular Models

| Model | Provider | Best For |
|-------|----------|----------|
| `anthropic/claude-3.5-sonnet` | Anthropic | General coding |
| `openai/gpt-4-turbo` | OpenAI | Complex reasoning |
| `google/gemini-pro-1.5` | Google | Large context |
| `meta-llama/llama-3.1-405b-instruct` | Meta | Open-source alternative |
