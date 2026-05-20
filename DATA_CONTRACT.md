# Data Contract

This document is kvelmo's promise about your data. It describes what kvelmo stores, where it lives on disk, what is and is not transmitted off your machine, and what survives an upgrade.

If anything below ever drifts from how kvelmo actually behaves, treat that as a bug and open a [security advisory](https://github.com/valksor/kvelmo/security/advisories).

## Key Principle

**Your data is yours, and it stays on your machine.**

kvelmo is a local orchestrator. It does not run a hosted service. It does not collect usage analytics. It does not phone home. The only network traffic kvelmo originates is traffic you configured — to a task provider, a webhook URL, or an agent endpoint.

User-generated artifacts (configuration, tasks, recordings, logs) are partitioned cleanly from kvelmo's own files. Upgrading kvelmo replaces the binary and bundled assets; it never touches your user layer.

## Protected User Layer

These artifacts are yours. kvelmo creates and reads them but treats them as user-owned data. Backups, restores, and exports operate on this layer.

| Path                            | Contents                                                                                                              |
| ------------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| `~/.valksor/kvelmo/kvelmo.yaml` | Global configuration (providers, agent settings, defaults)                                                            |
| `~/.valksor/kvelmo/.env`        | Local environment overrides (tokens, API keys)                                                                        |
| `~/.valksor/kvelmo/recordings/` | Agent session recordings (JSONL, rotated by line count)                                                               |
| `~/.valksor/kvelmo/activity/`   | RPC activity log (daily JSONL, SHA256-chained for tamper detection, 30-file retention by default)                     |
| `~/.valksor/kvelmo/memory/`     | Vector store entries used for semantic context recall                                                                 |
| `~/.valksor/kvelmo/worktrees/`  | Per-worktree socket files (recreated on demand)                                                                       |
| `<project>/.kvelmo/`            | Per-project state: tasks, plans, reviews, chat history, screenshots, event log (`events.jsonl`), checkpoints metadata |

The base path is `~/.valksor/kvelmo/` by default, or whatever `KVELMO_HOME` is set to.

### Recording sanitization

Recordings under `~/.valksor/kvelmo/recordings/` capture agent prompts and responses. Before any record is written to disk, `agent/recorder/sanitizer.go` applies two layers of redaction:

1. **Known token masking** — values from `kvelmo.yaml` providers and well-known environment variables (`GITHUB_TOKEN`, `GITLAB_TOKEN`, `WRIKE_TOKEN`, `LINEAR_TOKEN`, `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`) are replaced with a masked form preserving the first four and last four characters.
2. **Regex redaction** — common secret formats are replaced with `[REDACTED:<name>]` placeholders. Currently covered: AWS access and secret keys, GitHub tokens, Anthropic API keys, OpenAI API keys, JWT tokens, generic API key assignments, RSA/EC/DSA/OpenSSH private key blocks, and generic secret/password/credential assignments.

Sanitization happens before disk write, not after. Even so, recordings can contain other sensitive content (file paths, code snippets) — treat them like any other working file.

### Activity log integrity

`internal/activitylog/` writes RPC entries as JSONL with daily rotation. Each entry carries a `prev_hash` field containing the SHA256 of the previous entry, forming a tamper-evident chain. The log is local-only by default; webhook forwarding to an external receiver only happens if you explicitly configure a `Forwarder`.

### In-memory metrics

`metrics/` keeps counters (job submissions, RPC requests, token consumption, agent latencies) in process memory via atomics and `sync.Map`. Metrics are never persisted unless you explicitly export them via `kvelmo export` or scrape the Prometheus endpoint.

## System Layer

These artifacts are kvelmo's. Upgrades replace them; you should not edit them by hand.

- The `kvelmo` binary itself (replaced atomically by `internal/update/` via `os.Rename`)
- Bundled web UI assets (compiled and embedded at build time)
- Built-in catalog templates (`internal/catalog/`)
- Schemas, default policies, and generated TypeScript types

## What Gets Transmitted

kvelmo only sends data over the network when you have explicitly configured a destination:

- **Provider API calls** — to the providers you configured (GitHub, GitLab, Linear, Wrike, Jira, Azure DevOps) when fetching or updating tasks
- **Agent endpoints** — prompts sent to the agent you selected (Claude CLI, Codex CLI, Anthropic, OpenAI, Ollama, or a custom endpoint)
- **Webhook notifications** — to the Slack or generic webhook URL you configured under `internal/notify/`
- **Activity log forwarding** — only if you configured a forwarder under `internal/activitylog/`

There is no telemetry. There is no usage analytics. There is no automatic crash reporting. Disabling network access entirely still leaves the core workflow functional for local-only providers (file-based tasks, local Ollama agents).

## Update Guarantee

`kvelmo upgrade` and the install script only touch the system layer:

1. The new binary is downloaded next to the current executable (same filesystem, so `os.Rename` is atomic on POSIX).
2. The downloaded file is `chmod 0755`'d and renamed into place.
3. The running process keeps the old binary open until it exits; new invocations pick up the new binary.

Nothing under `~/.valksor/kvelmo/` or any project's `.kvelmo/` directory is touched. If an upgrade ever does corrupt user-layer data, that is a bug — report it.

## Destructive Operations Name What They Touch

Commands that remove data state the artifacts they touch in their output and confirmation prompts. The user-affecting ones today:

- `kvelmo delete <task>` — removes the task's entry in `<project>/.kvelmo/` (plan, review, chat, checkpoints)
- `kvelmo abandon` — drops the current task's working state but preserves the event log entry
- `kvelmo cleanup` — removes stale socket files under `~/.valksor/kvelmo/worktrees/`
- `kvelmo backup` / `restore` — operate on the full user layer; restore is destructive to the current state

If a destructive command ever removes more than it advertises, that is a bug.

## Where to Verify

The behavior described above is implemented in:

- `paths/resolver.go` — canonical disk locations
- `agent/recorder/sanitizer.go` — secret redaction in recordings
- `internal/activitylog/log.go` — SHA256-chained activity log
- `internal/eventlog/eventlog.go` — per-task event log
- `internal/update/installer.go` — atomic binary replacement
- `metrics/metrics.go` — in-memory counters

If you have questions about how a specific kvelmo command handles your data, the source is the authoritative answer. If you find a gap between this document and the code, open an issue.

## Related

- [`LICENSE`](LICENSE) — BSD-3-Clause
- [`SECURITY.md`](SECURITY.md) — vulnerability disclosure
- [`LEGAL_DISCLAIMER.md`](LEGAL_DISCLAIMER.md) — limitations, AI risks, third-party compliance
- [`TRADEMARK.md`](TRADEMARK.md) — brand usage
