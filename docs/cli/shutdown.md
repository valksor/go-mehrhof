# shutdown

Shutdown the worktree socket server.

## Usage

```bash
kvelmo shutdown
```

## Description

Shuts down the worktree socket server for the current directory. By default, sends a graceful shutdown request and waits for the socket to exit.

**Note:** This stops the kvelmo server, not the current operation. Use `kvelmo stop` to stop a running operation.

## Options

| Flag              | Description                                       |
| ----------------- | ------------------------------------------------- |
| `-t`, `--timeout` | Graceful shutdown timeout (default: 2s)           |
| `-f`, `--force`   | Skip graceful shutdown and unregister immediately |

## Examples

```bash
# Graceful shutdown
kvelmo shutdown

# Force shutdown
kvelmo shutdown --force

# Custom timeout
kvelmo shutdown --timeout 5s
```

## Related

- [stop](/cli/stop.md) — Stop running operation
- [serve](/cli/serve.md) — Start socket server
- [cleanup](/cli/cleanup.md) — Remove stale sockets
