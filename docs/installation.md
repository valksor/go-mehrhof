# Installation

## Requirements

- **Git** (for version control integration)
- **Claude CLI** (Mehrhof delegates AI operations to Claude)

> **Note:** Go 1.25+ is only required if building from source.

---

## Option 1: Pre-built Binary (Recommended)

The fastest way to get started is to download a pre-built binary for your platform.

### Available Platforms

| Platform | Architecture | Binary Name |
|----------|--------------|-------------|
| Linux | AMD64 | `mehr-linux-amd64` |
| Linux | ARM64 | `mehr-linux-arm64` |
| macOS | AMD64 (Intel) | `mehr-darwin-amd64` |
| macOS | ARM64 (Apple Silicon) | `mehr-darwin-arm64` |

### Installation

```bash
# 1. Download the binary for your platform (example: macOS ARM64)
curl -L https://github.com/valksor/go-mehrhof/releases/latest/download/mehr-darwin-arm64 -o mehr

# 2. Make it executable
chmod +x mehr

# 3. Move to a directory in your PATH
sudo mv mehr /usr/local/bin/

# 4. Verify installation
mehr version
```

### Using a Different Installation Directory

If you prefer not to use `/usr/local/bin`:

```bash
# Create a bin directory in your home
mkdir -p ~/bin
mv mehr ~/bin/

# Add to PATH (add this to your ~/.bashrc, ~/.zshrc, etc.)
export PATH="$PATH:$HOME/bin"
```

### Windows Users

For Windows, use [WSL](https://learn.microsoft.com/en-us/windows/wsl/install) or build from source.

---

## Option 2: Nightly Build (Pre-release)

Nightly builds are created from the latest commit on the main branch. They are always available but should be used with caution as they may contain untested changes.

```bash
# Download latest nightly (example: macOS ARM64)
curl -L https://github.com/valksor/go-mehrhof/releases/download/nightly/mehr-darwin-arm64 -o mehr
chmod +x mehr
sudo mv mehr /usr/local/bin/
```

**Important:** Nightly builds are pre-release. If a stable release exists, prefer that over nightly.

### View Nightly Release Notes

To see what's included in a nightly build:

```bash
curl -L https://github.com/valksor/go-mehrhof/releases/tag/nightly
```

---

## Option 3: Build from Source

If you need to build from source or contribute to Mehrhof:

### Prerequisites

- **Go 1.25+** - [Install Go](https://go.dev/dl/)

### Clone and Build

```bash
# Clone the repository
git clone https://github.com/valksor/go-mehrhof.git
cd go-mehrhof

# Build to ./build/mehr
make build

# Or install to $GOPATH/bin
make install

# Verify installation
mehr version
```

### Verify Installation

Running `mehr version` should show output like:

```
mehr v1.0.0
Commit: abc1234
Built:  2025-01-15T10:30:00Z
Go:     go1.25.0
```

---

## Updating Mehrhof

### Self-Update Command

Mehrhof can update itself automatically:

```bash
# Check for updates
mehr update --check

# Update to latest release
mehr update

# Update to latest pre-release
mehr update --pre-release

# Skip confirmation
mehr update -y
```

### Manual Update

To update manually, simply download the latest binary and replace the existing one:

```bash
# Re-download the latest binary
curl -L https://github.com/valksor/go-mehrhof/releases/latest/download/mehr-darwin-arm64 -o mehr
chmod +x mehr
sudo mv mehr /usr/local/bin/
```

---

## Claude CLI Setup

Mehrhof calls Claude CLI for all AI operations. Ensure Claude is installed and configured:

```bash
# Verify Claude works
claude --version

# Test Claude
claude "hello world"
```

Whatever configuration you have for Claude (API keys, settings, etc.) will be used by Mehrhof.

If Claude isn't set up, follow the [Claude CLI installation guide](https://github.com/anthropics/claude-cli).

---

## Project Initialization

Once Mehrhof is installed, initialize it in your project:

```bash
cd your-project
mehr init
```

This creates:

- `.mehrhof/` directory for task storage
- Updates `.gitignore` to exclude task data

---

## Makefile Commands

The project includes a Makefile with useful targets for development:

| Command        | Description                        |
| -------------- | ---------------------------------- |
| `make build`   | Build binary to `./build/mehr`     |
| `make install` | Build and install to `$GOPATH/bin` |
| `make test`    | Run all tests with coverage        |
| `make lint`    | Run golangci-lint                  |
| `make fmt`     | Format code with go fmt            |
| `make tidy`    | Tidy go.mod dependencies           |

---

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

---

## Uninstalling

Remove the binary:

```bash
rm $(which mehr)
```

Clean up configuration (optional):

```bash
rm -rf ~/.mehrhof
```

---

## Troubleshooting

### "command not found: mehr"

Ensure the binary location is in your PATH:

```bash
# Check where mehr is installed
which mehr

# If not found, add installation directory to PATH
export PATH="$PATH:/usr/local/bin"
```

### Claude Not Working

Ensure Claude CLI is installed and configured:

```bash
claude --version
```

If Claude isn't set up, follow the [Claude CLI installation guide](https://github.com/anthropics/claude-cli).

### Permission Denied

Ensure the binary is executable:

```bash
chmod +x $(which mehr)
```

### Self-Update Fails

If `mehr update` fails, download manually:

```bash
curl -L https://github.com/valksor/go-mehrhof/releases/latest/download/mehr-darwin-arm64 -o mehr
chmod +x mehr
sudo mv mehr /usr/local/bin/
```

For more troubleshooting help, see [Troubleshooting](troubleshooting/index.md).
