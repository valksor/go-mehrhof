# kvelmo discover

List available project commands.

## Usage

```bash
kvelmo discover [flags]
```

## Description

Scans the project directory for available commands from common task runners and build tools. Detected commands can be used by agents to understand what build, test, and lint commands are available.

## Sources

| Source           | Detection                            | Format             |
|------------------|--------------------------------------|-------------------|
| Makefile         | Targets matching `^[a-zA-Z0-9][a-zA-Z0-9_-]*:` | `make <target>`    |
| package.json     | Scripts section                      | `bun run <script>` or `npm run <script>` |
| Taskfile.yml     | Task names                           | `task <name>`      |
| bin/ directory   | Executable files                     | `./bin/<name>`     |

Package manager detection: uses `bun` if `bun.lockb` exists, otherwise `npm`.

## Options

| Flag     | Description              |
|----------|--------------------------|
| `--json` | Output as JSON           |

## Examples

```bash
# List all discovered commands
kvelmo discover

# Machine-readable output
kvelmo discover --json
```

## Output

```
Discovered commands (6)

  make build
  make test
  make quality
  bun run dev
  bun run lint
  task deploy
```

## Related

- [files](/cli/files.md) — List project files
- [status](/cli/status.md) — Show current state
