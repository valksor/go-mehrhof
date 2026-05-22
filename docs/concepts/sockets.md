# Socket Architecture

kvelmo uses Unix domain sockets for inter-process communication. This enables real-time coordination between the CLI, Web UI, and AI agents.

## Overview

```
┌─────────────────────────────────────────────────────────────┐
│                      CLI / Web UI                           │
└─────────────────────────┬───────────────────────────────────┘
                          │ JSON-RPC
┌─────────────────────────┴───────────────────────────────────┐
│                    Socket Layer                             │
│  ┌─────────────────┐    ┌─────────────────────────────────┐ │
│  │  Global Socket  │    │  Worktree Sockets (per-project) │ │
│  │  - Registry     │    │  - State Machine                │ │
│  │  - Worker Pool  │    │  - Git Operations               │ │
│  │  - Job Queue    │    │  - Streaming Events             │ │
│  └─────────────────┘    └─────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

## Two Socket Types

### Global Socket

A single instance at `~/.valksor/kvelmo/global.sock`.

**Responsibilities:**

- Project registry management
- Worker pool coordination
- Job queue management
- Cross-project operations

**Started by:** `kvelmo serve`

### Worktree Socket

One per project at `<project>/.kvelmo/worktree.sock`.

**Responsibilities:**

- Task state machine
- Git operations (branches, checkpoints)
- Specification management
- Real-time event streaming

**Created automatically** when you work on a project.

## Protocol

Communication uses JSON-RPC 2.0 over Unix domain sockets.

### Request Format

```json
{
  "jsonrpc": "2.0",
  "protocol_version": "1",
  "id": 1,
  "method": "plan",
  "params": {}
}
```

`protocol_version` is the kvelmo socket protocol major version. First-party
clients stamp it automatically. It is optional — a request that omits it is
treated as a legacy client and accepted.

### Response Format

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "state": "planned",
    "specification": "/path/to/spec.md"
  }
}
```

### Error Format

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32600,
    "message": "Invalid request"
  }
}
```

## Protocol Versioning & Compatibility

The socket protocol is versioned by a single major number (`protocol_version`,
currently `"1"`). The compatibility contract:

- **Same major version** — fully compatible.
- **Absent version** — treated as a legacy client and accepted, so older or
  third-party clients keep working.
- **Different major version** — every method except the cross-version control
  surface is rejected with `-32600` (invalid request) and a message pointing the
  caller to the handshake or to upgrade.

### Cross-version methods

These three methods are always reachable, regardless of protocol version, so a
mismatched client can still discover the server version and an upgrade can hand
off cleanly:

- `system.capabilities` — the handshake (below)
- `ping` — liveness + build info
- `shutdown` — graceful stop

### Handshake: `system.capabilities`

Call it to learn the server's protocol version and the methods it serves:

```json
{
  "protocol_version": "1",
  "kvelmo_version": "v0.10.0",
  "methods": ["agent.status", "ping", "plan", "system.capabilities", "..."]
}
```

(`kvelmo_version` is whatever the running build reports — `meta.Version` — so the
value above is illustrative.)

A client should call `system.capabilities` first when talking to a server of
unknown age, compare `protocol_version`, and only issue version-gated methods
when the major versions match.

### Stability

Starting at kvelmo v1.0, the protocol major version is a compatibility promise:
the major number is only incremented for a breaking change, and any such bump is
documented in the release notes. Method additions are backward-compatible and do
not bump the major version.

## Event Streaming

The worktree socket supports event streaming for real-time updates:

### Agent Events

- `token` — Agent output token
- `tool_call` — Agent called a tool
- `tool_result` — Tool returned a result
- `permission` — Permission requested
- `completion` — Agent completed

### State Events

- `state_change` — Task state changed
- `checkpoint` — Checkpoint created

## Why Sockets?

### Benefits

1. **Real-time coordination** — CLI, Web UI, and agents share state instantly
2. **Multi-client** — Multiple CLI instances can monitor the same task
3. **Efficient** — No HTTP overhead for local communication
4. **Streaming** — Events flow in real-time, not polling

### Compared to HTTP

| Aspect     | Sockets    | HTTP                   |
| ---------- | ---------- | ---------------------- |
| Latency    | ~1ms       | ~10ms                  |
| Connection | Persistent | Per-request            |
| Streaming  | Native     | Requires SSE/WebSocket |
| Local-only | Yes        | Needs CORS, auth       |

## Socket Locations

| Socket   | Location                          | Purpose           |
| -------- | --------------------------------- | ----------------- |
| Global   | `~/.valksor/kvelmo/global.sock`   | Registry, workers |
| Worktree | `<project>/.kvelmo/worktree.sock` | Per-project state |

## CLI and Sockets

The CLI is a thin client that sends requests to sockets:

```bash
kvelmo status    # Queries worktree socket
kvelmo workers   # Queries global socket
kvelmo plan      # Sends plan request to worktree socket
```

## Web UI and Sockets

The Web UI connects via WebSocket to the server started by `kvelmo serve`. The server bridges WebSocket to Unix domain sockets.

```
Browser ──WebSocket──► Server ──Unix Socket──► Global/Worktree
```

## Troubleshooting

### "socket not found"

The server isn't running:

```bash
kvelmo serve
```

### "permission denied"

Socket file has wrong permissions:

```bash
chmod 600 ~/.valksor/kvelmo/global.sock
```

### Stale socket file

If kvelmo crashed, remove the stale socket:

```bash
rm ~/.valksor/kvelmo/global.sock
kvelmo serve
```

## Related

- [Workflow](/concepts/workflow.md) — Task lifecycle
- [State Machine](/concepts/state-machine.md) — States managed by sockets
