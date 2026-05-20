# Claude

Plain binary use: spawns `claude --print` with stream-json over stdin/stdout.

|                              |                                                                                                     |
| ---------------------------- | --------------------------------------------------------------------------------------------------- |
| **Mechanism**                | `claude --print` (no `--sdk-url`, no `--mcp-config`, no PTY)                                        |
| **Works today**              | Yes                                                                                                 |
| **Billing today**            | Max subscription                                                                                    |
| **Billing after 2026-06-15** | $200/mo Agent SDK credit pool (Anthropic reclassifies `claude -p` usage; the spawn does not change) |

## The other two claude variants

| Adapter                               | What it does                               | Works today                                                | Billing                           |
| ------------------------------------- | ------------------------------------------ | ---------------------------------------------------------- | --------------------------------- |
| [`claude-mcp`](/agents/claude-mcp.md) | Interactive TUI + MCP server. **Default.** | Yes                                                        | Sub (across the 2026-06-15 split) |
| [`claude-sdk`](/agents/claude-sdk.md) | WebSocket Agent SDK (`--sdk-url`)          | **No** (Anthropic rejected the flag in claude CLI 2.1.121) | —                                 |

## Prerequisites

```bash
claude auth login
claude --version
```

## Selection

```yaml
# ~/.valksor/kvelmo/kvelmo.yaml or .valksor/kvelmo.yaml
agent:
  default: claude # if you want this adapter as default
```

```bash
kvelmo plan --agent claude   # per-task override
```

## Custom agents (`extends: claude`)

`extends: claude` is for proxy-backed custom agents that drive a proxy (e.g. [Claude Code Router](https://github.com/musistudio/claude-code-router)) routing to a non-Anthropic provider. The proxy expects stream-json over stdio, which is what this adapter speaks — so they keep working unchanged.

Do not switch a proxy-backed extension to `extends: claude-mcp` — the TUI/MCP architecture is incompatible with proxies expecting stream-json over stdio.

```yaml
custom_agents:
  opus:
    extends: claude
    args: ["--model", "opus"]
```

## Spawn detail

The adapter passes: `--print --verbose --output-format stream-json --input-format stream-json --permission-mode bypassPermissions`. Communication is NDJSON over the child's stdin/stdout.

## Troubleshooting

**`claude: command not found`** — install or update the CLI:

```bash
which claude
claude update
```

**Authentication issues:**

```bash
claude auth logout
claude auth login
```

## Related

- [Agents overview](/agents/index.md)
- [Custom agents](/agents/custom.md)
- [Codex](/agents/codex.md)
