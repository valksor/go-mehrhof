# Contributing

Thank you for your interest in contributing to Mehrhof! This document provides guidelines for contributing to the project.

## Development Setup

### Prerequisites

- **Go 1.25+** - Required for building from source
- **Git** - Required for version control operations
- **make** - For build automation

### Getting Started

```bash
# Clone the repository
git clone https://github.com/valksor/go-mehrhof.git
cd go-mehrhof

# Download dependencies
make deps

# Build the binary
make build

# Install to $GOPATH/bin (optional)
make install
```

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage and race detection
make coverage

# Generate HTML coverage report
make coverage-html
```

### Code Quality

```bash
# Run linters (golangci-lint, govulncheck)
make quality

# Format code
make fmt

# Tidy dependencies
make tidy
```

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

    "github.com/valksor/go-mehrhof/internal/conductor"  // local
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
- Use `wg.Go(func() { ... })` instead of `wg.Add(1); go func() { defer wg.Done() }()`

## Testing

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
- Update documentation with examples
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
- Update documentation if needed
- Reference any related issues

### Review Process

Maintainers will review your PR and provide feedback. Please address review comments promptly. Once approved, your PR will be merged.

## Reporting Issues

When reporting bugs or requesting features:

1. **Search existing issues** first to avoid duplicates
2. **Use the issue templates** if available
3. **Provide clear details**:
   - Steps to reproduce (for bugs)
   - Expected vs actual behavior
   - Environment details (OS, Go version)
   - Relevant logs or error messages

## Getting Help

- **GitHub Issues**: For bugs and feature requests
- **GitHub Discussions**: For questions and general discussion
- **Documentation**: See the [docs/](../) directory for detailed guides

## Documentation

### Adding Diagrams

For diagrams in documentation, use **Mermaid** syntax but export to static images to avoid loading a 3MB JavaScript library.

**Generate a PNG from Mermaid:**

```bash
cat <<'EOF' | npx -p @mermaid-js/mermaid-cli mmdc -i - -o docs/_media/img/diagram.png -b transparent
stateDiagram-v2
    [*] --> StateA
    StateA --> StateB
EOF
```

**Reference in markdown:**

```markdown
![Diagram Title](../_media/img/diagram.png)
```

**Why PNG instead of SVG?**
- Mermaid's SVG output uses `<foreignObject>` for text labels
- Browsers block `foreignObject` in SVGs loaded via `<img>` tags (security policy)
- PNG rasterization bypasses this limitation and renders consistently

**Alternative: Use Mermaid Live Editor**
- Visit [mermaid.live](https://mermaid.live)
- Write your diagram
- Export as PNG/SVG manually

## License

By contributing to Mehrhof, you agree that your contributions will be licensed under the [BSD 3-Clause License](license.md).

The full contributing guide is also available in the [CONTRIBUTING.md](https://github.com/valksor/go-mehrhof/blob/master/CONTRIBUTING.md) file in the repository root.
