# kvelmo autostart

Automatically start the worktree socket when needed.

## Description

The autostart mechanism is an internal feature that provides a seamless experience where users do not need to manually run `kvelmo start` before other commands. When a command needs the worktree socket and it is not running, autostart spawns `kvelmo start --foreground` in the background and waits up to 10 seconds for the socket to become available.

This is not a standalone CLI command but an internal helper used by other commands.

## How It Works

1. Command checks if the worktree socket exists
2. If missing, spawns `kvelmo start --foreground` as a background process
3. Polls for socket availability (up to 10 seconds)
4. Reports the PID of the auto-started process to stderr

## Safety

- Disabled during Go test execution to prevent fork bombs (test binaries would re-execute themselves)
- Falls back to a manual start message if auto-start fails

## Related

- [start](/cli/start.md) — Manually start a worktree socket
- [serve](/cli/serve.md) — Start the global socket server
- [cleanup](/cli/cleanup.md) — Remove stale socket files
