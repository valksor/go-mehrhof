# Installation Issues

Solutions for problems during installation and setup.

> **Tip:** If you're having trouble building from source, consider downloading a [pre-built binary](../installation.md) instead.

## "command not found: mehr"

**Cause:** Binary not in PATH.

**Solution:**

```bash
# Check if installed
ls $(go env GOPATH)/bin/mehr

# Add to PATH
export PATH="$PATH:$(go env GOPATH)/bin"

# Make permanent (add to ~/.bashrc or ~/.zshrc)
echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.bashrc
```

## "go: command not found"

**Cause:** Go not installed.

**Solution:**

```bash
# macOS
brew install go

# Linux
sudo apt install golang-go

# Verify
go version
```

## Build Fails

**Cause:** Missing dependencies or old Go version.

**Solution:**

```bash
# Check Go version (need 1.21+)
go version

# Update dependencies
go mod tidy

# Rebuild
make build
```
