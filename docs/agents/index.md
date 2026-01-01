# AI Agents


> **⚠️ Integration Testing Notice**: Agent and provider implementations depend on third-party APIs that may change. While unit-tested, these integrations may have edge cases not covered by automated tests. Manual testing in your environment is recommended before production use.

Mehrhof orchestrates AI agents to help with planning and implementation. It delegates AI operations to external CLI tools.

## How It Works

Mehrhof doesn't connect to AI APIs directly. Instead, it calls agent CLIs as subprocesses. Whatever configuration you have for the agent will be used automatically.

```
User → mehr plan → Agent CLI → AI Response → Mehrhof processes output
```

## Available Agents

- [Claude](claude.md) (Default) - Anthropic's Claude AI
- [Codex](codex.md) - Alternative AI agent
- [Aider](aider.md) - Git-aware AI pair programming assistant
- [Gemini](gemini.md) - Google Gemini AI with 1M token context
- [Ollama](ollama.md) - Local AI inference
- [Copilot](copilot.md) - GitHub Copilot integration
- [OpenRouter](openrouter.md) - Access to 100+ AI models

## Configuration

See [Aliases](aliases.md) for creating custom agent configurations and [Per-Step Configuration](per-step.md) for using different agents per workflow step.

## How Agents Work

### Planning Phase

During `mehr plan`, the agent:

1. Receives the task source content
2. Reads any existing notes
3. Analyzes the requirements
4. Generates SPEC files with implementation details

### Implementation Phase

During `mehr implement`, the agent:

1. Reads all SPEC files
2. Considers notes and context
3. Generates or modifies code files
4. Provides a summary of changes

## Agent Output

Agents produce structured output:

```
<<FILE:path/to/file.go>>
package main

func main() {
    // Generated code
}
<<END FILE>>

<<SUMMARY>>
Created main.go with basic structure.
<<END SUMMARY>>
```

This format allows Mehrhof to:

- Extract file changes
- Apply modifications safely
- Track what was generated

## Session Logging

All agent interactions are logged:

```
.mehrhof/work/<id>/sessions/
├── 2025-01-15T10-30-00-planning.yaml
├── 2025-01-15T11-00-00-talk.yaml
└── 2025-01-15T11-30-00-implementing.yaml
```

Each session includes:

- Timestamps
- Message history
- Token usage
- Cost tracking

## Troubleshooting

### Claude Not Working

Ensure Claude CLI is properly installed and configured:

```bash
# Check Claude is available
claude --version

# Test Claude works
claude "Hello"
```

If Claude has issues, fix them in your Claude CLI configuration first.

### "Agent timeout"

Increase the timeout in `.mehrhof/config.yaml`:

```yaml
agent:
  timeout: 600 # 10 minutes
```

### "Rate limited"

The agent will retry automatically up to `agent.max_retries` times. If issues persist, wait before retrying.

### Verbose Output

See agent interactions in real-time:

```bash
mehr plan --verbose
mehr implement --verbose
```
