# claude-mcp agent

The `claude-mcp` agent runs an **interactive Claude Code session** under a pseudo-terminal, wired to a kvelmo-served MCP server. The orchestration logic that other adapters drive through `claude --print` lives in MCP tools that the running Claude Code session calls into.

## Why it's the default

`claude-mcp` is the only claude adapter that **(a)** works against the current Anthropic CLI **and** **(b)** preserves Max-subscription billing after the 2026-06-15 policy change. The other two are partial solutions:

- The plain [`claude`](/agents/claude.md) adapter works today but its billing classification (`claude -p`) means it falls into Anthropic's new $200/mo Agent SDK credit pool after 2026-06-15.
- The [`claude-sdk`](/agents/claude-sdk.md) adapter is broken on the current CLI — Anthropic restricted `--sdk-url` to their own backend in claude 2.1.121. This is a separate issue from the 2026-06-15 billing change.

`claude-mcp` sidesteps both by keeping the spawned `claude` in **interactive TUI mode** (which Anthropic still bills against the subscription) and giving the model the kvelmo workflow as MCP tools it can call.

> Anthropic has not contractually committed to keeping third-party-launched TUI sessions on the subscription. They could fingerprint and reclassify. Treat this as a best-effort path, not a guarantee.

## Default agent

As of this release, `agent.default` defaults to `claude-mcp`. A fresh kvelmo install will use this adapter automatically — no yaml override required.

Set `agent.default` to something other than `claude-mcp` only if:

- You want the plain CLI path → `agent.default: claude` (works on current CLI; bills credit pool post-June-15)
- You have a custom agent extending the WebSocket SDK path via a proxy (Claude Code Router etc.) → `agent.default: claude-sdk` or `extends: claude-sdk` on the custom agent

`extends: claude` is for proxy-backed custom agents that speak stream-json over stdio — e.g. a custom agent routing through [Claude Code Router](https://github.com/musistudio/claude-code-router) to a non-Anthropic provider (Z.ai's GLM, OpenAI, etc.).

## How it works

```
kvelmo CLI ──► claudemcp adapter ──► PTY ──► claude (interactive TUI)
                  │                          │
                  │ rendezvous Unix sock     │ --mcp-config
                  ◄──── signal_complete ─────│
                                             │ stdio MCP
                                             ▼
                                       kvelmo mcp --stdio  ──► worktree socket
                                       (separate subprocess     (conductor)
                                        spawned by claude)
```

The adapter:

1. Writes a per-session `mcp-config.json` and `system-prompt.md` under `~/.valksor/kvelmo/work/<task-id>/claudemcp-<rand>/`.
2. Binds a Unix-domain socket (`adapter.sock`, mode 0600) for rendezvous.
3. Spawns `claude --strict-mcp-config --mcp-config … --append-system-prompt … --permission-mode acceptEdits [model] [seed prompt]` under a PTY.
4. Pumps PTY output as opaque transcript events and waits for `signal_complete` / `signal_failure` on the rendezvous socket.

`claude` itself spawns the `kvelmo mcp --stdio` subprocess (named in the MCP config). That subprocess connects to the per-worktree kvelmo socket and exposes seven MCP tools.

## MCP tools exposed to the model

| Tool | Purpose |
|---|---|
| `kvelmo_get_task` | Current task metadata (id, title, state). |
| `kvelmo_get_specifications` | List prior spec files, optionally with content (≤10 MiB per file). |
| `kvelmo_read_file` | Read a single file from the worktree (≤10 MiB). |
| `kvelmo_save_artifact` | Persist a spec/plan/implementation_summary. |
| `kvelmo_create_checkpoint` | Create a git checkpoint commit. |
| `kvelmo_signal_complete` | Acknowledge phase completion (acknowledgment only; the existing worker-pool/graph path drives the actual state advance). MUST be called before exit. |
| `kvelmo_signal_failure` | Acknowledge phase failure (same pattern: the pump emits EventError that drives the failure path). |

> **No `list_files` / `search` MCP tool today.** The model uses its own built-in file-discovery tools (Glob, Grep, Read) for code exploration inside the worktree. The kvelmo MCP surface is intentionally narrow — it covers only the kvelmo-specific orchestration primitives that claude cannot infer on its own.

## Selecting this adapter

```bash
kvelmo config set agent.default claude-mcp
```

The existing `claude` adapter remains untouched and selectable; any custom agent that `extends: claude` continues to work via the original adapter.

## Configuration

```yaml
agent:
  default: claude-mcp
  claude_mcp:
    permission_mode: acceptEdits   # acceptEdits | auto | bypassPermissions | dontAsk | default
    model: ""                       # e.g. "sonnet", "opus", or full model ID
    mcp_server_command: ["kvelmo", "mcp", "--stdio"]
    # macOS: mcp_server_command: ["/Users/you/.local/bin/kvelmo", "mcp", "--stdio"]
    # Linux: mcp_server_command: ["/home/you/.local/bin/kvelmo", "mcp", "--stdio"]
    # ^ explicit absolute path; only needed if you want to pin to a specific
    #   binary other than the currently-running one. The default rewrites
    #   the bare "kvelmo" to the running binary's absolute path via
    #   os.Executable(), so the spawned claude finds it regardless of PATH.
    system_prompt_override: ""      # if set, replaces the default kvelmo orchestration prompt;
                                    # whitespace-only values are treated as empty (default applies)
    strict_mcp_config: true         # adds --strict-mcp-config (ignore user defaults)
```

## Authentication

The spawned `claude` is expected to use its own saved login (the same one a human session uses) — that fallback is what keeps the session billed against the Max subscription.

**The adapter actively strips** `ANTHROPIC_API_KEY`, `ANTHROPIC_AUTH_TOKEN`, and `ANTHROPIC_BEARER_TOKEN` from the child environment before spawning `claude`. If you have any of these set in your shell (e.g. for other tools), the adapter logs a warning and removes them from the child env only — your parent shell is untouched. Leaving the key in place would silently route the session onto API billing and defeat the entire purpose of this adapter.

If you want the session to actually use an API key (for whatever reason), use the `agent/anthropic` adapter instead.

## Subprocess PATH

`claude` spawns the `kvelmo mcp --stdio` subprocess as its MCP server. When the default `mcp_server_command: ["kvelmo", "mcp", "--stdio"]` is in effect, the adapter resolves `kvelmo` to the **absolute path of the currently-running kvelmo binary** via `os.Executable()` before writing the MCP config. This means `claude` doesn't need to find `kvelmo` on its PATH — which matters because GUI-launched `claude` instances often have a minimal PATH that excludes `~/.local/bin`.

If you set `mcp_server_command` explicitly to a non-`kvelmo` first element (e.g. a custom launcher or a different absolute path), the adapter passes it through unchanged.

## Risks (read before adopting)

1. **Anthropic may reclassify.** No contractual protection that third-party-launched TUI sessions stay on the sub.
2. **Sub rate limits apply.** Max 20x rate limits are sized for human-paced typing. Autonomous loops burn the per-5h limit faster than `--print` did. If the session stalls with no output for several minutes, check the Max subscription usage page — you may have hit a rate limit. The pump emits an `EventError` when claude exits, but the failure reason in that case won't include "rate limited" specifically.
3. **`permission_mode` controls how much the LLM can do unsupervised.** Default is `acceptEdits` — claude can write files in the worktree without prompting but still gates dangerous shell commands. `bypassPermissions` removes all guardrails (functionally equivalent to the older `--dangerously-skip-permissions` flag); opt in only when you know what you're doing.
4. **PTY/TUI output is opaque.** Treated as transcript only. Never parsed for control flow — all control flows through MCP.
5. **`claude --help` may change.** Adapter pre-flight assumes `--mcp-config`, `--permission-mode`, `--append-system-prompt`, `--model`, and positional prompt in TUI mode. If Anthropic renames these, the adapter breaks.
6. **PID-recycle in the watchdog.** `kvelmo mcp` watches the spawning `claude` process via `kill(pid, 0)`. On a long-running system where PIDs wrap, a recycled PID owned by the same user will keep the MCP subprocess alive past the real parent's exit. Polling cadence is 5s, so the practical window is narrow.

## Verifying the billing path

After 2026-06-15: run a single `plan` phase with this adapter on a throwaway worktree, then inspect the Anthropic billing dashboard at https://console.anthropic.com/. Two things to look for:

1. **Where did the usage land?** Anthropic distinguishes "Claude subscription usage" from "Agent SDK credit" in the usage breakdown. If the session shows up under subscription usage → success. If under Agent SDK credit → Anthropic has fingerprinted PTY-spawned sessions and the adapter no longer preserves sub billing.
2. **Dashboard latency.** Usage can take **up to 24 hours** to appear in the dashboard. Don't conclude failure from a single check immediately after the test run; wait a day and look again.

If the path is closed, the fallbacks are: (a) switch back to the `claude` adapter and accept the $200 credit, (b) switch to a non-Anthropic adapter (`openai`, `ollama`), or (c) use a direct Anthropic API key via the `anthropic` adapter (separate from the subscription).
