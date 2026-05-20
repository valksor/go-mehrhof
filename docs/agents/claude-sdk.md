# claude-sdk

WebSocket Agent SDK transport: spawns `claude --print --sdk-url ws://127.0.0.1:<port>`.

> ⚠️ **Broken on the official Anthropic `claude` CLI.** Anthropic restricted `--sdk-url` to their own backend in version 2.1.121, so the spawn fails immediately with:
>
> ```
> Error: --sdk-url rejected: host "127.0.0.1" is not an approved Anthropic
> endpoint. This flag is reserved for Remote Control worker processes
> connecting to Anthropic's backend.
> ```
>
> Whether the restriction will be lifted is unknown. Independent of the 2026-06-15 Agent SDK billing-pool split.

## When this adapter still works

A proxy that accepts `--sdk-url` (e.g. [Claude Code Router](https://github.com/musistudio/claude-code-router)) — the proxy intercepts the connection before it reaches Anthropic's backend.

## When NOT to use it

For vanilla Anthropic claude, use one of:

- [`claude`](/agents/claude.md) — plain CLI, works today
- [`claude-mcp`](/agents/claude-mcp.md) — TUI + MCP, default, preserves sub billing after 2026-06-15

## Selection

```yaml
agent:
  default: claude-sdk # only if you have a proxy that accepts --sdk-url
```

Or as the base for a proxy-backed custom agent:

```yaml
custom_agents:
  glm-sdk:
    extends: claude-sdk
    env:
      ANTHROPIC_BASE_URL: "http://localhost:3456"
```

## Historical note

Before claude CLI 2.1.121, `agent/claude/` was dual-mode (WebSocket SDK first, CLI fallback). The split into `claude`, `claude-sdk`, and `claude-mcp` happened when 2.1.121 made the SDK path unconditionally fail — keeping the dual-mode meant every kvelmo startup ate one noisy `--sdk-url rejected` error before reaching the working CLI path.

## Related

- [Agents overview](/agents/index.md)
