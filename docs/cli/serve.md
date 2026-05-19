# kvelmo serve

Start the global socket and web server.

## Usage

```bash
kvelmo serve
```

## Options

| Flag     | Description                 |
| -------- | --------------------------- |
| `--open` | Open browser automatically  |
| `--port` | Web UI port (default: 6337) |

## Examples

```bash
# Start server
kvelmo serve

# Start and open browser
kvelmo serve --open

# Custom port
kvelmo serve --port 8080
```

## What Happens

1. Global socket starts at `~/.valksor/kvelmo/global.sock`
2. Web UI starts at http://localhost:6337
3. Server runs until stopped (Ctrl+C)

## Output

```
Global socket listening at ~/.valksor/kvelmo/global.sock
Web UI available at http://localhost:6337
```

## Running in Background

```bash
# Background with nohup
nohup kvelmo serve &

# Or use a process manager
```

## Stopping

Press Ctrl+C in the terminal, or:

```bash
# Find and kill
pkill kvelmo
```

## Related

- [Web UI Guide](/web-ui/getting-started.md) — Using the Web UI
- [Sockets](/concepts/sockets.md) — Socket architecture
