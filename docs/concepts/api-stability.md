# API Stability & Compatibility

This page is the compatibility contract for kvelmo. Starting at **v1.0**, kvelmo
follows [Semantic Versioning](https://semver.org): a **major** bump signals a
breaking change, a **minor** bump adds backward-compatible functionality, and a
**patch** bump is fixes only. The sections below define exactly what is covered.

## Versioned surfaces

| Surface                     | Versioned by                       | Policy                                                                                                                                                                                                                                     |
| --------------------------- | ---------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Socket JSON-RPC protocol    | `protocol_version` (major int)     | See [Sockets](/concepts/sockets.md). Cross-version handshake via `system.capabilities`.                                                                                                                                                    |
| Config file (`kvelmo.yaml`) | `version` field + migration chain  | Old configs are migrated forward on load; a config written by a newer kvelmo is rejected with an error telling the user to upgrade.                                                                                                        |
| Task state (`task.yaml`)    | `format_version` field             | A binary reads its own and older state; newer state is rejected with a clear error.                                                                                                                                                        |
| Event log (`events.jsonl`)  | per-entry `v` field                | Append-only; readers accept both older and newer entries (JSON is additive, so unknown fields are ignored) — it is the one surface with no hard newer-than rejection, because a single log can mix entries written across kvelmo versions. |
| Backup archives (`.tar.gz`) | embedded manifest `format_version` | Restore validates the manifest before extracting; newer archives are rejected.                                                                                                                                                             |
| Public Go API               | Go module SemVer                   | See below.                                                                                                                                                                                                                                 |
| CLI commands & flags        | Go module SemVer                   | See below.                                                                                                                                                                                                                                 |

Config, task state, and backups follow a **forward-read policy**: version _N_ of
kvelmo reads data written by version _N_ and earlier, and refuses data written by
a newer version rather than silently dropping fields it does not understand. The
append-only event log is the deliberate exception noted above.

## Public Go API

These module-root packages are importable by external programs (e.g. via
`kvelmo pipe` adapters) and are covered by the SemVer promise:

- `agent/` — the `Agent` interface, `Event` model, and registry
- `settings/` — `Settings` and config loading/migration
- `metrics/` — observability counters
- `meta/` — build metadata
- `paths/` — path resolution

Everything under `internal/` is **not** part of the public API and may change in
any release.

### The `agent.Agent` interface is frozen

`agent.Agent` is a stable extension point: third parties implement it to add
custom agents. To keep those implementations compiling across releases, the
interface is **frozen** — it does not gain methods within a major version.

**New capabilities are added as separate optional interfaces**, detected by type
assertion, never by growing `Agent`:

```go
// A future optional capability — agents that don't implement it keep working.
type Resumable interface {
    Resume(ctx context.Context, sessionID string) error
}

// Callers opt in by asserting:
if r, ok := someAgent.(Resumable); ok {
    _ = r.Resume(ctx, id)
}
```

This is the same pattern the standard library uses (`io.Reader` vs
`io.ReaderFrom`). It means adding a capability is a **minor** change, not a
breaking one.

## CLI commands & flags

Command names and flags are part of the contract. After v1.0:

- A command or flag is **never removed** without a deprecation cycle: it is first
  marked deprecated (still functional, with a warning) for at least one minor
  release, and only removed in a later major release.
- New commands and optional flags are backward-compatible (minor releases).
- Output formats intended for scripting (`--json`, `export`) are stable; the
  human-readable text output is not a contract and may be reworded.

## Deprecation policy

Nothing is deprecated at v1.0 — pre-v1, surfaces could still change freely. After
v1.0, a deprecation is announced in the release notes, kept working for the
deprecation window, and removed only in a subsequent major release. Go code uses
the standard `// Deprecated:` comment so tooling can flag it.

## What is not covered

- Anything under `internal/`.
- The web UI's internal component structure and Zustand stores.
- Exact wording of human-readable CLI/log output.
- The on-disk layout of caches and other regenerable, throwaway data.
