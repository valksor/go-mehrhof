# Ollama Agent

Ollama provides local AI inference for privacy and cost savings. Mehrhof wraps the `ollama run` command.

## Prerequisites

- Ollama installed and running (`ollama serve`)
- At least one model downloaded (`ollama pull codellama`)

```bash
# Verify Ollama works
ollama --version

# Pull a coding model
ollama pull codellama
```

## Key Features

- **Local inference**: No API calls, complete privacy
- **Free to use**: No per-token costs
- **Multiple models**: Support for various open-source models

**Default Model:** `codellama`

## Configuration

```yaml
# .mehrhof/config.yaml
agents:
  ollama-llama3:
    extends: ollama
    description: "Ollama with Llama 3"
    args: ["--model", "llama3:70b"]

  ollama-codellama:
    extends: ollama
    description: "Ollama with CodeLlama"
    args: ["--model", "codellama:34b"]

  ollama-deepseek:
    extends: ollama
    description: "Ollama with DeepSeek Coder"
    args: ["--model", "deepseek-coder:33b"]
```

## Popular Models for Coding

| Model | Size | Best For |
|-------|------|----------|
| `codellama` | 7B-34B | General code generation |
| `llama3` | 8B-70B | Reasoning and complex tasks |
| `deepseek-coder` | 1.3B-33B | Code completion |
| `mistral` | 7B | Fast inference |
| `mixtral` | 8x7B | High quality, larger context |
