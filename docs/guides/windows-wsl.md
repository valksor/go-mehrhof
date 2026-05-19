# Windows WSL Setup

Complete guide to running kvelmo on Windows using WSL2.

## Prerequisites

- Windows 10 version 2004 or later, or Windows 11
- Administrator access

## Step 1: Enable WSL2

Open **PowerShell as Administrator** and run:

```powershell
wsl --install
```

Restart your computer when prompted.

## Step 2: Set Up Ubuntu

After restart:

1. Ubuntu opens automatically (or search "Ubuntu" in Start)
2. Wait for initial setup
3. Create a username (lowercase, no spaces)
4. Create a password (you'll need this for `sudo`)

**Note:** Password characters don't appear as you type - this is normal.

## Step 3: Update Ubuntu

In the Ubuntu terminal:

```bash
sudo apt update && sudo apt upgrade -y
```

## Step 4: Install kvelmo

```bash
curl -fsSL https://raw.githubusercontent.com/valksor/kvelmo/master/install.sh | bash
```

Verify:

```bash
kvelmo version
```

## Step 5: Install an Agent CLI

Install Claude CLI (or another agent):

```bash
# Follow instructions at https://claude.ai/code
claude --version
```

## Working with Windows Files

### Accessing Windows Files

Your Windows files are at `/mnt/c/`:

| Windows Path            | WSL Path                    |
| ----------------------- | --------------------------- |
| `C:\Users\Name\Desktop` | `/mnt/c/Users/Name/Desktop` |
| `C:\Projects`           | `/mnt/c/Projects`           |
| `D:\Work`               | `/mnt/d/Work`               |

### Navigate to Your Project

```bash
cd /mnt/c/Users/YourName/Projects/myproject
kvelmo serve
```

### Performance Note

For best performance, keep your code in the Linux filesystem:

```bash
# Create a projects folder in Linux
mkdir -p ~/projects
cd ~/projects

# Clone your repo here
git clone https://github.com/your/repo.git
cd repo
kvelmo serve
```

## Using VS Code

VS Code integrates well with WSL:

1. Install [VS Code](https://code.visualstudio.com/)
2. Install the "WSL" extension
3. Open a folder in WSL: `code .` from Ubuntu terminal

## Troubleshooting

### "Please enable Virtual Machine Platform"

1. Open **Turn Windows features on or off**
2. Enable:
   - Virtual Machine Platform
   - Windows Subsystem for Linux
3. Restart

### WSL is Slow

- Store code in Linux filesystem (`~/projects/`) not Windows (`/mnt/c/`)
- Close other programs using memory
- Check WSL memory limit in `%UserProfile%\.wslconfig`

### Cannot Connect to Server

Check if the server is running:

```bash
kvelmo serve
```

Check the port:

```bash
curl http://localhost:6337
```

### Permission Denied

Fix socket permissions:

```bash
chmod 600 ~/.valksor/kvelmo/global.sock
```

### Git Errors

Configure git:

```bash
git config --global user.name "Your Name"
git config --global user.email "your@email.com"
```

## Accessing Web UI from Windows

The Web UI runs in WSL but is accessible from Windows:

1. Start the server in WSL: `kvelmo serve`
2. Open browser in Windows: http://localhost:6337

## PowerShell Alternative

For Windows-native workflows, use the PowerShell installer:

```powershell
irm https://raw.githubusercontent.com/valksor/kvelmo/master/install.ps1 | iex
```

This installs kvelmo inside WSL from PowerShell.

## Next Steps

- [Quickstart](/quickstart.md) — Run your first task
- [Web UI Guide](/web-ui/getting-started.md) — Use the visual interface
- [CLI Reference](/cli/index.md) — Command reference
