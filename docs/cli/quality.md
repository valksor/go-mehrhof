# quality

Quality gate controls.

## Usage

```bash
kvelmo quality <subcommand>
```

## Description

Commands for interacting with the quality gate during task review. Quality gates allow human intervention when the agent encounters decisions requiring approval.

## Subcommands

### respond

Answer a pending quality gate prompt by providing a yes/no response.

```bash
kvelmo quality respond --prompt-id <ID> --yes|--no
```

| Flag          | Description                        |
| ------------- | ---------------------------------- |
| `--prompt-id` | Prompt ID to respond to (required) |
| `--yes`       | Answer yes                         |
| `--no`        | Answer no                          |

## Examples

```bash
# Check for pending prompts
kvelmo status

# Approve a quality gate prompt
kvelmo quality respond --prompt-id abc123 --yes

# Reject a quality gate prompt
kvelmo quality respond --prompt-id abc123 --no
```

## Finding Prompt IDs

The prompt ID is shown in `kvelmo status` when a quality gate question is waiting:

```
State: waiting
Quality gate pending: abc123
  "Should I delete the deprecated API endpoint?"
```

## Related

- [status](/cli/status.md) — Check for pending prompts
- [review](/cli/review.md) — Human review mode
