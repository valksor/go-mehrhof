# Installation

## Requirements

- **Go 1.25+** (for building from source)
- **Git** (for version control integration)
- **Claude CLI** (Mehrhof delegates AI operations to Claude)

## Build from Source

### Clone and Build

```bash
git clone <repository-url>
cd go-mehrhof

# Build to ./build/mehr
make build

# Or install to $GOPATH/bin
make install
```

### Verify Installation

```bash
mehr version
```

Output:

```
mehr v1.0.0
Commit: abc1234
Built:  2025-01-15T10:30:00Z
Go:     go1.25.0
```

## Configuration

### Claude CLI Setup

Mehrhof calls Claude CLI for all AI operations. Ensure Claude is installed and configured:

```bash
# Verify Claude works
claude --version

# If not set up, follow Claude's installation guide
```

Whatever configuration you have for Claude (API keys, settings, etc.) will be used by Mehrhof.

### Project Initialization

Initialize Mehrhof in your project:

```bash
cd your-project
mehr init
```

This creates:

- `.mehrhof/` directory for task storage
- Updates `.gitignore` to exclude task data

## Makefile Commands

The project includes a Makefile with useful targets:

| Command        | Description                        |
| -------------- | ---------------------------------- |
| `make build`   | Build binary to `./build/mehr`     |
| `make install` | Build and install to `$GOPATH/bin` |
| `make test`    | Run all tests with coverage        |
| `make lint`    | Run golangci-lint                  |
| `make fmt`     | Format code with go fmt            |
| `make tidy`    | Tidy go.mod dependencies           |

## Development Setup

For contributing to Mehrhof:

```bash
# Install dependencies
go mod download

# Run tests
make test

# Run with arguments (builds first)
make run-args ARGS="start task.md"
```

## Updating

To update to the latest version:

```bash
cd go-mehrhof
git pull
make install
```

## Uninstalling

Remove the binary:

```bash
rm $(which mehr)
```

Clean up configuration (optional):

```bash
rm -rf ~/.mehrhof
```

## Troubleshooting

### "command not found: mehr"

Ensure `$GOPATH/bin` is in your PATH:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

### Claude Not Working

Ensure Claude CLI is installed and configured:

```bash
claude --version
```

If Claude isn't set up, follow Claude's installation and configuration guide.

### Permission Denied

Ensure the binary is executable:

```bash
chmod +x $(which mehr)
```

See [Troubleshooting](troubleshooting/index.md) for more solutions.
