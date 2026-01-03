# GitHub Copilot Agent

> **⚠️ Third-Party Integration**: This integration depends on external APIs that may change. Not fully tested beyond unit tests. Behavior may vary depending on the third-party service. Manual validation recommended before production use.


GitHub Copilot agent wraps the `gh copilot` CLI extension for shell command suggestions and code explanations.

## Prerequisites

- GitHub CLI installed (`gh`)
- Copilot extension installed (`gh extension install github/gh-copilot`)
- Active GitHub Copilot subscription

```bash
gh copilot --version
```

## Key Features

- **Suggest mode**: Generate shell commands from natural language
- **Explain mode**: Explain what commands do
- **Target types**: Shell, Git, or GitHub CLI commands

## Configuration

```yaml
# .mehrhof/config.yaml
agents:
  copilot-shell:
    extends: copilot
    description: "Copilot for shell commands"
    args: ["--target", "shell"]

  copilot-git:
    extends: copilot
    description: "Copilot for git commands"
    args: ["--target", "git"]
```

## Limitations

- Cloud-only (requires GitHub subscription)
- Best for command-line tasks, not code generation
- Context sent to GitHub servers
