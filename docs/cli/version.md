# mehr version

Print version information.

## Synopsis

```bash
mehr version
```

## Description

The `version` command displays build information about the Mehrhof CLI:

- Version number
- Git commit hash
- Build timestamp
- Go runtime version

This is useful for troubleshooting, bug reports, and verifying installations.

## Flags

This command has no specific flags.

## Examples

### Show Version

```bash
mehr version
```

Output:

```
mehr v1.0.0
  Commit: a1b2c3d4e5f6
  Built:  2024-01-15T10:30:00Z
  Go:     go1.21.5
```

### Development Build

When running from source without ldflags:

```bash
mehr version
```

Output:

```
mehr dev
  Commit: none
  Built:  unknown
  Go:     go1.21.5
```

## Build Information

Version information is embedded at build time using Go ldflags:

```bash
go build -ldflags "-X main.Version=v1.0.0 -X main.Commit=$(git rev-parse HEAD) -X main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
```

The Makefile handles this automatically:

```bash
make build
make install
```

## See Also

- [Quickstart](../quickstart.md) - Installing Mehrhof
- [CLI Overview](index.md) - All available commands
