# explain

Ask the agent to explain its last action.

## Usage

```bash
kvelmo explain
```

## Description

Sends a message asking the agent to explain what it did, why it made those choices, and any assumptions or constraints it encountered.

## Options

| Flag             | Description                                   |
| ---------------- | --------------------------------------------- |
| `-p`, `--prompt` | Custom prompt to override the default request |

## Examples

```bash
# Ask for explanation with default prompt
kvelmo explain

# Custom explanation prompt
kvelmo explain --prompt "Why did you choose that data structure?"
```

## Default Prompt

> Explain what you did in the last action, why you made those choices, and any assumptions or constraints you encountered.

## Output

```
Explain request sent (job: abc123)
Use 'kvelmo status' to check progress
```

## Related

- [chat](/cli/chat.md) — Interactive agent conversation
- [watch](/cli/watch.md) — Stream live output
