# Contributing

Thank you for your interest in contributing to kvelmo! This document provides guidelines for contributing to the project.

## Development Setup

### Prerequisites

- **Go 1.25+** - Required for building from source
- **bun** – Required for the web frontend
- **Git** – Required for version control operations
- **make** – For build automation

### Getting Started

```bash
# Clone the repository
git clone https://github.com/valksor/kvelmo.git
cd kvelmo

# Build the binary
make build

# Build including web frontend
make build
```

### Running Tests

```bash
# Run all tests
make test

# Run tests with verbose output
make test-v

# Run tests with race detector
make test-race

# Generate HTML coverage report
make test-cover
```

### Code Quality

```bash
# Run linters and formatters (golangci-lint, gofmt, goimports, gofumpt, check-alias)
make quality

# Format code only
make fmt

# Build and run quality checks
make ci
```

**Run checks for what you changed:**
- Go code (`cmd/`, `internal/`, `*.go`): `make quality`
- Web frontend (`web/`): `cd web && bun run build`
- Docs only (`*.md`): No checks required

## Code Style

### Import Order

Imports should be grouped in the following order, with each group sorted alphabetically:

1. Standard library
2. Third-party packages
3. Local packages

```go
import (
    "fmt"           // standard library
    "os"            // standard library

    "github.com/spf13/cobra"  // third-party

    "github.com/valksor/kvelmo/internal/conductor"  // local
)
```

### Naming Conventions

- **Exported**: PascalCase (`MyFunction`, `MyType`)
- **Unexported**: camelCase (`myFunction`, `myType`)
- **Constants**: PascalCase (`MaxRetries`)
- **Interfaces**: Usually PascalCase, often ending with `-er` suffix (`Reader`, `Writer`)

### Error Handling

Always handle errors explicitly:

```go
if err != nil {
    return fmt.Errorf("operation failed: %w", err)
}
```

For multiple independent errors, use `errors.Join`:

```go
var errs []error
if err := op1(); err != nil {
    errs = append(errs, fmt.Errorf("op1: %w", err))
}
if err := op2(); err != nil {
    errs = append(errs, fmt.Errorf("op2: %w", err))
}
if len(errs) > 0 {
    return errors.Join(errs...)
}
```

### Modern Go Practices (Go 1.25+)

- Use `errors.Join()` for aggregating multiple errors
- Use `slices.Contains()`, `slices.Concat()`, `maps.Clone()` from stdlib
- Use `log/slog` for structured logging
- Always pass `context.Context` for cancelable operations

## Critical Rules

CI enforces these rules. Violating them will cause your PR to fail.

### Import Discipline

Import packages directly. **No type aliases, no wrapper functions, no re-exports.**

```go
// ✅ GOOD - Direct import
import "github.com/gorilla/websocket"
conn, _ := websocket.Upgrade(...)

// ❌ BAD - Type alias or wrapper
type Conn = websocket.Conn  // Don't do this
```

CI enforces this via `make check-alias`.

### No nolint Abuse

`//nolint` is a last resort. Never disable linters globally in `.golangci.yml`.

**Acceptable** (with justification):
```go
//nolint:unparam // Required by interface
//nolint:errcheck // String builder WriteString won't fail
```

**Never acceptable:**
- `//nolint:errcheck` without justification
- `//nolint:gosec` (fix the security issue)
- `//nolint:all` (never suppress all linters)

Always: specify a linter name, include justification, place on a specific line.

### File Size Limit

Keep all Go files under **500 lines**. Split by feature or responsibility:

```go
// Split handlers.go (1000 lines) into:
handlers_plan.go      // Planning handlers
handlers_implement.go // Implementation handlers
handlers_review.go    // Review handlers
```

## Testing

### Test-First Development

**Write tests FIRST (TDD).** This ensures your implementation meets requirements and catches regressions.

### Testing Strategy

**During development:** Run targeted tests for changed packages:
```bash
go test ./internal/socket/...           # Test a package
go test -run TestName ./internal/...    # Test specific function
```

**Before committing:** Run the full test suite:
```bash
make test  # Only after implementation is complete
```

### Conventions

- Use the standard `testing` package
- Prefer table-driven tests for multiple cases
- Target 80%+ code coverage
- Place test files next to the code they test (`foo_test.go`)

### Table-Driven Test Example

```go
func TestParse(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {"valid input", "test", "test", false},
        {"empty input", "", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Parse(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if got != tt.want {
                t.Errorf("Parse() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

## Pull Request Process

### Before Submitting

1. **Format your code**: `make fmt`
2. **Run linters**: `make quality` (fix any issues)
3. **Run tests**: `make test` (ensure all pass)
4. **Update documentation**: If adding features, update relevant docs

### Commit Messages

Follow clear, descriptive commit messages:

```
Add support for GitLab provider

- Implement GitLab API client
- Add issue parsing and MR creation
```

### Branch Naming

Use descriptive branch names:

- `feature/add-jira-provider`
- `fix/agent-session-restore`
- `docs/update-readme`

### Submitting a PR

1. Fork the repository
2. Create a feature branch from `master`
3. Make your changes and commit
4. Push to your fork
5. Open a pull request with a clear description

Your PR should:
- Pass all CI checks
- Include tests for new functionality
- Reference any related issues

### Review Process

Maintainers will review your PR and provide feedback. Please address review comments promptly. Once approved, your PR will be merged.

## Reporting Issues

### Bug Reports

Before submitting a bug report, please verify the issue against different versions:

1. **Check the latest release** – Confirm the bug exists in the latest stable release
2. **Check the nightly build** – Test against the latest nightly build to see if it's already fixed
3. **Decide whether to report**:
   - ✅ **Submit an issue** if: Bug exists in both release AND nightly
   - ✅ **Submit an issue** if: Bug exists ONLY in nightly (newly introduced)
   - ❌ **Do NOT submit** if: Bug exists only in release but is FIXED in nightly

### When Submitting Issues

1. **Search existing issues** first to avoid duplicates
2. **Provide clear details**:
   - Version tested (`kvelmo version`)
   - Steps to reproduce
   - Expected vs. actual behavior
   - Environment details (OS, Go version)
   - Relevant logs or error messages

### Feature Requests

For feature requests, please:
- Describe the use case clearly
- Explain why existing functionality doesn't meet your needs
- Consider if this would benefit most users or is specific to your workflow

## Getting Help

- **GitHub Issues**: For bugs and feature requests
- **GitHub Discussions**: For questions and general discussion

## Reference

For deeper context on project architecture and conventions:

- [CLAUDE.md](CLAUDE.md) — Complete project conventions and architecture overview
- [AGENTS.md](AGENTS.md) — Agent-specific guidance

## License

By contributing to kvelmo, you agree that your contributions will be licensed under the [BSD 3-Clause License](https://github.com/valksor/kvelmo/blob/master/LICENSE).
