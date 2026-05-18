# Claude Agent (legacy SDK adapter)

> ⚠️ **Broken against the current Anthropic `claude` CLI.** This adapter spawns `claude --print --sdk-url ws://127.0.0.1:<port>` and Anthropic now rejects that:
>
> ```
> Error: --sdk-url rejected: host "127.0.0.1" is not an approved Anthropic
> endpoint. This flag is reserved for Remote Control worker processes
> connecting to Anthropic's backend.
> ```
>
> **For vanilla Anthropic claude, use [`claude-mcp`](/agents/claude-mcp.md) instead.** That adapter is the new default — it spawns claude in interactive TUI mode under a PTY with `--mcp-config`, which (a) works on the current CLI and (b) preserves Max-subscription billing after the 2026-06-15 Agent SDK credit split.
>
> This adapter is retained only because custom agents (`extends: claude` in `custom_agents`) typically route through a proxy such as Claude Code Router that still accepts `--sdk-url`. If your custom agent uses a proxy, keep `extends: claude`. Do **not** switch a proxy-backed extension to `extends: claude-mcp` — the TUI/MCP architecture is incompatible with a proxy that expects Agent SDK calls.

The Claude agent uses Anthropic's Claude CLI for AI-assisted development.

## Prerequisites

Install the Claude CLI:

1. Visit https://claude.ai/code
2. Follow the installation instructions
3. Authenticate: `claude auth login`
4. Verify: `claude --version`

## Configuration

### Setting as Default

```bash
kvelmo config set default_agent claude
```

Or in `~/.valksor/kvelmo/kvelmo.yaml`:
```json
{
  "default_agent": "claude"
}
```

### Using for Specific Tasks

```bash
kvelmo start --from file:task.md --agent claude
```

## Connection Modes

The Claude agent supports two connection modes:

### WebSocket (Primary)

WebSocket provides real-time streaming:
- Lower latency
- Better for interactive use
- Used when available

### CLI (Fallback)

Falls back to CLI mode if WebSocket unavailable:
- Spawns `claude` process
- Streams output via stdout
- Works in all environments

## Model Selection

Specify Claude model variants:

```json
{
  "agents": {
    "claude-opus": {
      "extends": "claude",
      "args": ["--model", "claude-opus-4"]
    }
  }
}
```

Then use:
```bash
kvelmo plan --agent claude-opus
```

## Tool Support

Claude supports these tools during execution:

| Tool  | Description            |
|-------|------------------------|
| Read  | Read file contents     |
| Write | Write file contents    |
| Edit  | Edit file with diff    |
| Glob  | Find files by pattern  |
| Grep  | Search file contents   |
| Bash  | Execute shell commands |

## Permissions

By default:
- Read tools are auto-approved
- Write tools prompt for approval

Configure auto-approval in settings:
```json
{
  "agent": {
    "auto_approve": ["Read", "Glob", "Grep"]
  }
}
```

## Troubleshooting

### "claude: command not found"

Install or update the Claude CLI:
```bash
# Check if installed
which claude

# Update to latest
claude update
```

### Authentication Issues

Re-authenticate:
```bash
claude auth logout
claude auth login
```

### Model Not Available

Check your Claude subscription:
```bash
claude models list
```

## Related

- [Agents Overview](/agents/index.md)
- [Codex Agent](/agents/codex.md)
- [Custom Agents](/agents/custom.md)
