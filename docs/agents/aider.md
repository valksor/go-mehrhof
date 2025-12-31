# Aider Agent

Aider is a Git-aware AI pair programming assistant. Mehrhof wraps the `aider` CLI for code generation tasks.

## Prerequisites

- Aider CLI installed (`pip install aider-chat`)
- API key configured (supports OpenAI, Anthropic, and other providers)

```bash
# Verify Aider works
aider --version
```

## Key Features

- **Git-aware**: Understands repository structure and history
- **Multi-file editing**: Can modify multiple files in a single session
- **Auto-commits disabled**: Changes are applied without automatic commits (Mehrhof manages commits)

## Configuration

```yaml
# .mehrhof/config.yaml
agents:
  aider-gpt4:
    extends: aider
    description: "Aider with GPT-4"
    args: ["--model", "gpt-4"]

  aider-claude:
    extends: aider
    description: "Aider with Claude"
    args: ["--model", "claude-3-opus-20240229"]
```
