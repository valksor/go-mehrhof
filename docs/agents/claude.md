# Claude Agent (plain CLI)

The `claude` agent spawns `claude --print` with stream-json over stdin/stdout. No `--sdk-url`, no `--mcp-config`, no PTY — just the binary in print mode.

**Status:** works on the current Anthropic `claude` CLI. Billing classification is `claude -p` (Agent SDK by Anthropic's terminology), so the 2026-06-15 billing change — Agent SDK usage moves off Max subscription limits onto a new $200/mo credit pool — applies to this path. Mechanically the spawn is unchanged either way; only the upstream billing classification shifts.

**Sibling adapters:**

| Adapter | Doc | Use case |
|---|---|---|
| [`claude-mcp`](/agents/claude-mcp.md) | claude-mcp.md | **Default.** Interactive TUI + MCP. Preserves Max-subscription billing after 2026-06-15. |
| [`claude-sdk`](/agents/claude-sdk.md) | claude-sdk.md | WebSocket Agent SDK (`--sdk-url`). **Broken on the official Anthropic CLI since version 2.1.121** (Anthropic restricted the flag to their own backend; whether that will be lifted is unknown). Retained for proxy setups (Claude Code Router etc.) that still accept the flag. |

**Custom agents that `extends: claude`** (e.g. `glm` routed through Claude Code Router) inherit this CLI behaviour. Before the dual-mode split they silently rode a WebSocket-then-CLI fallback on every spawn; they now use CLI directly with no startup noise. Keep `extends: claude` for proxy-backed extensions — do not switch to `extends: claude-mcp` (the TUI/MCP architecture is incompatible with a proxy that expects stream-json over stdio).

---

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

## Connection Mode

Single-mode CLI: spawns `claude --print --verbose --output-format stream-json --input-format stream-json --permission-mode bypassPermissions`. Communication is NDJSON over the child's stdin/stdout — no WebSocket, no `--sdk-url`, no PTY.

The legacy adapter (pre-2026-Q2 split) was dual-mode: it tried WebSocket Agent SDK first (`--sdk-url ws://...`) and silently fell back to CLI when the WebSocket handshake failed. Claude CLI 2.1.121 restricted `--sdk-url` to Anthropic's own backend, making the WebSocket attempt fail unconditionally. The adapter was split so each path is explicit: this page describes the CLI variant; [`claude-sdk`](/agents/claude-sdk.md) is the WebSocket variant (broken against the official CLI, retained for proxy setups).

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
