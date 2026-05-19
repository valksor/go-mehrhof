# kvelmo rpc

Send a raw JSON-RPC call to the socket.

## Usage

```bash
kvelmo rpc <method> [params-json]
kvelmo rpc --global <method> [params-json]
```

## Description

A power-user tool for scripting and debugging. Sends an arbitrary JSON-RPC method call to the worktree or global socket and prints the raw JSON response to stdout.

## Options

| Flag       | Description                                      |
| ---------- | ------------------------------------------------ |
| `--global` | Send to global socket instead of worktree socket |

## Examples

```bash
# Call status on worktree socket
kvelmo rpc status

# Call plan with parameters
kvelmo rpc plan '{"force":true}'

# List workers on global socket
kvelmo rpc --global workers.list

# Get metrics from global socket
kvelmo rpc --global metrics
```

## Timeout

All RPC calls have a 30-second timeout.

## Socket Resolution

- Without `--global`: connects to the worktree socket for the current directory
- With `--global`: connects to the global socket at `~/.valksor/kvelmo/global.sock`

## Related

- [status](/cli/status.md) — Friendly status output
- [diagnose](/cli/diagnose.md) — System diagnostics
- [serve](/cli/serve.md) — Start the global socket server
