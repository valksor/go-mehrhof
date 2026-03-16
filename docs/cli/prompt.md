# kvelmo prompt

Output task status for shell prompt integration.

## Usage

```bash
kvelmo prompt
```

## Output

Outputs a short status string suitable for shell prompt (PS1) integration. Outputs nothing if no task is active.

Format: `[kvelmo:STATE]`

## Shell Integration

### Bash

```bash
PS1='$(kvelmo prompt 2>/dev/null)\$ '
```

### Fish

```fish
function fish_prompt
  set -l kv (kvelmo prompt 2>/dev/null)
  echo -n "$kv \$ "
end
```

## Related

- [status](/cli/status.md) — Full task status
- [completion](/cli/completion.md) — Shell completion
