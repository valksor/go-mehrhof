# Codex

The Codex agent uses OpenAI's Codex CLI for AI-assisted development.

## Prerequisites

Install the Codex CLI:

1. Visit the Codex documentation
2. Follow the installation instructions
3. Authenticate with your API key
4. Verify: `codex --version`

## Configuration

### Setting as Default

```bash
kvelmo config set default_agent codex
```

Or in `~/.valksor/kvelmo/kvelmo.yaml`:

```json
{
  "default_agent": "codex"
}
```

### Using for Specific Tasks

```bash
kvelmo start --from file:task.md --agent codex
```

## Connection Modes

### WebSocket (Primary)

WebSocket provides real-time streaming when available.

### CLI (Fallback)

Falls back to CLI mode:

- Spawns `codex` process
- Streams output via stdout

## Model Selection

Specify Codex model variants in configuration:

```json
{
  "agents": {
    "codex-4": {
      "extends": "codex",
      "args": ["--model", "codex-4"]
    }
  }
}
```

## Tool Support

Codex supports standard tools:

| Tool  | Description            |
| ----- | ---------------------- |
| Read  | Read file contents     |
| Write | Write file contents    |
| Edit  | Edit file with diff    |
| Glob  | Find files by pattern  |
| Grep  | Search file contents   |
| Bash  | Execute shell commands |

## Permissions

Configure auto-approval:

```json
{
  "agent": {
    "auto_approve": ["Read", "Glob", "Grep"]
  }
}
```

## Troubleshooting

### "codex: command not found"

Install the Codex CLI:

```bash
# Check if installed
which codex
```

### API Key Issues

Verify your API key is set:

```bash
codex auth status
```

## Related

- [Agents Overview](/agents/index.md)
- [Claude Agent](/agents/claude.md)
- [Custom Agents](/agents/custom.md)
