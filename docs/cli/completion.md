# completion

Shell completion setup.

## Usage

```bash
kvelmo completion <shell>
```

## Supported Shells

| Shell      | Command                        |
| ---------- | ------------------------------ |
| bash       | `kvelmo completion bash`       |
| zsh        | `kvelmo completion zsh`        |
| fish       | `kvelmo completion fish`       |
| powershell | `kvelmo completion powershell` |

## Installation

### Bash

```bash
# Add to ~/.bashrc
source <(kvelmo completion bash)
```

### Zsh

```bash
# Add to ~/.zshrc
kvelmo completion zsh > "${fpath[1]}/_kvelmo"
```

### Fish

```bash
kvelmo completion fish > ~/.config/fish/completions/kvelmo.fish
```

### PowerShell

```powershell
kvelmo completion powershell | Out-String | Invoke-Expression
```

## What It Does

Enables tab completion for kvelmo commands and flags.

## Examples

After setup:

```bash
kvelmo st<TAB>  # Completes to: kvelmo status
kvelmo start --<TAB>  # Shows available flags
```
