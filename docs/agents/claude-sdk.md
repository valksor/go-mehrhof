# claude-sdk Agent (WebSocket Agent SDK)

> ⚠️ **Broken on the official Anthropic `claude` CLI since version 2.1.121.** This adapter spawns `claude --print --sdk-url ws://127.0.0.1:<port>` and Anthropic rejects that:
>
> ```
> Error: --sdk-url rejected: host "127.0.0.1" is not an approved Anthropic
> endpoint. This flag is reserved for Remote Control worker processes
> connecting to Anthropic's backend.
> ```
>
> Whether the restriction will be lifted at any point is unknown. This is a separate issue from the 2026-06-15 Agent SDK credit-pool split — Anthropic has not announced any change to the `--sdk-url` restriction in connection with that billing reorganisation. Treat the SDK transport as broken for the foreseeable future on the official CLI.
>
> The adapter is **registered** in kvelmo (so you can select it explicitly) but will fail at `Connect()` against the official binary. The error surfaces with the rejection text above; kvelmo reports a clean failure.

## When this adapter still works

Some proxy setups still accept `--sdk-url`. The most common is **[Claude Code Router](https://github.com/musistudio/claude-code-router)**, which:

- Intercepts `claude` invocations
- Translates Agent SDK requests to a different provider's API (e.g. Z.ai's GLM)
- Accepts `--sdk-url` from the kvelmo side

If your custom agent points at such a proxy, set `extends: claude-sdk` in your `custom_agents` map.

## When NOT to use this adapter

For vanilla Anthropic claude on the official CLI, use one of:

| Adapter | Doc | Why |
|---|---|---|
| [`claude`](/agents/claude.md) | claude.md | Plain `claude --print` over stdin/stdout. Works today. Bills `claude -p` (so the $200 Agent SDK credit pool after 2026-06-15). |
| [`claude-mcp`](/agents/claude-mcp.md) | claude-mcp.md | **Default.** Interactive TUI under PTY + `--mcp-config`. Works today and preserves Max-subscription billing after 2026-06-15. |

## Historical note

Before claude CLI 2.1.121, `agent/claude/` was dual-mode: it tried WebSocket SDK first (this adapter's path) and silently fell back to plain CLI (`agent/claude/` after the split) when the WebSocket spawn failed. Once 2.1.121 made `--sdk-url` fail unconditionally on the official binary, every kvelmo startup ate one noisy `--sdk-url rejected` error before reaching the working CLI path. The split makes the two modes explicit so each adapter does one thing.

## Configuration

```yaml
agent:
  default: claude-sdk   # only if you have a proxy that accepts --sdk-url
```

Or use it as the base for a custom agent extension:

```yaml
custom_agents:
  glm-sdk:
    extends: claude-sdk
    env:
      ANTHROPIC_BASE_URL: "http://localhost:3456"   # your proxy
```

## Selecting per-task

```bash
kvelmo plan --agent claude-sdk
```

Will fail with the rejection error if pointed at the official Anthropic binary; will succeed if pointed at a proxy that accepts `--sdk-url`.
