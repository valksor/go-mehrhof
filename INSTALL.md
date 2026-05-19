# Installation Guide

> **Last verified:** February 2026

Complete guide to install kvelmo and all prerequisites from scratch.

---

## Before You Begin (First-Time Users)

**New to command lines?** This section is for you. If you've used a terminal before, skip to [Quick Start](#quick-start-experienced-users).

### What is a Terminal?

A **terminal** (also called "command line", "console", or "shell") is a text-based way to control your computer. Instead of clicking buttons and icons, you type commands.

It looks like a window with text, usually with a **prompt** like this:

```
username@computer:~$
```

or on Mac:

```
username@MacBook ~ %
```

The `$` or `%` at the end means the computer is waiting for you to type something.

### How to Open a Terminal

**macOS:**

1. Press **Cmd + Space** to open Spotlight search
2. Type **Terminal**
3. Press **Enter**

**Windows (after WSL is installed - see [Windows section](#windows-wsl-setup)):**

1. Click the **Start** button
2. Type **Ubuntu**
3. Click to open

**Ubuntu/Linux:**

1. Press the **Super** key (the Windows logo key on your keyboard)
2. Type **Terminal**
3. Click to open

### Running Commands

When you see a command in this guide like:

```bash
sudo apt update
```

Here's what to do:

1. **Copy** the command (highlight it, then Ctrl+C or Cmd+C)
2. **Paste** it into your terminal (Ctrl+Shift+V on Linux, Cmd+V on Mac, or right-click → Paste)
3. Press **Enter** to run it

**What to expect:** You'll see text scroll by - this is normal! Commands often print information as they work.

### Passwords in the Terminal

Some commands start with `sudo` (like `sudo apt update`). This runs the command with administrator privileges.

When you run a `sudo` command:

- You'll be asked for your password
- **The characters won't appear as you type** - this is a security feature, not a bug
- Just type your password and press Enter

On Linux/WSL, this is your Linux password (the one you created during setup), not your Windows password.

---

## Which Computer Do I Have?

Some downloads depend on your computer's **architecture** (the type of processor inside). Here's how to check:

**Mac:**

1. Click the **Apple menu** (top-left corner)
2. Click **About This Mac**
3. Look for **Chip** or **Processor**:
   - **Apple M1, M2, M3, or M4** → You have **Apple Silicon (arm64)**
   - **Intel** → You have **Intel (amd64)**

**Linux:**
Open a terminal and run:

```bash
uname -m
```

- `x86_64` → You have **amd64**
- `aarch64` → You have **arm64**

**Windows:**
Don't worry about this - WSL handles it automatically.

---

## Key Terms Reference

| Term                 | What It Means                                               |
| -------------------- | ----------------------------------------------------------- |
| **Terminal**         | A text-based window where you type commands                 |
| **Command**          | Text you type to tell the computer what to do               |
| **Directory/Folder** | A container for files (same thing, different names)         |
| **Path**             | The location of a file or folder, like `/home/user/task.md` |
| **sudo**             | "Super user do" - runs a command with admin privileges      |
| **Package manager**  | Software that installs other software (like an app store)   |

---

## Quick Start (Experienced Users)

If you're familiar with terminals, here's the fast path:

```bash
# Install kvelmo
curl -fsSL https://raw.githubusercontent.com/valksor/kvelmo/master/install.sh | bash

# Verify installation
kvelmo version

# Make sure you have an agent CLI (Claude recommended)
claude --version

# Start the server
kvelmo serve
```

Done! Skip to [Verification](#verification).

---

## macOS Installation

### Step 1: Install kvelmo

Open Terminal and run:

```bash
curl -fsSL https://raw.githubusercontent.com/valksor/kvelmo/master/install.sh | bash
```

The script will:

- Detect your Mac type (Intel or Apple Silicon)
- Download the correct binary
- Install to `/usr/local/bin/kvelmo`
- Verify the download with checksums

### Step 2: Verify Installation

```bash
kvelmo version
```

You should see the version number.

### Step 3: Install an Agent CLI

kvelmo orchestrates AI agents. You need at least one agent CLI:

**Claude** (recommended):

1. Visit https://claude.ai/code
2. Follow the installation instructions
3. Verify: `claude --version`

**Codex**:

1. Visit the Codex documentation
2. Follow the installation instructions
3. Verify: `codex --version`

---

## Linux Installation

### Step 1: Install kvelmo

Open a terminal and run:

```bash
curl -fsSL https://raw.githubusercontent.com/valksor/kvelmo/master/install.sh | bash
```

### Step 2: Verify Installation

```bash
kvelmo version
```

### Step 3: Install an Agent CLI

See [macOS Step 3](#step-3-install-an-agent-cli) - the process is the same.

---

## Windows (WSL) Setup

kvelmo runs on Windows through WSL2 (Windows Subsystem for Linux). This gives you a full Linux environment inside Windows.

### Prerequisites

- Windows 10 version 2004+ or Windows 11
- Virtualization enabled in BIOS (usually enabled by default)

### Step 1: Install WSL2

Open **PowerShell as Administrator**:

1. Right-click the Start button
2. Click **Terminal (Admin)** or **PowerShell (Admin)**
3. If prompted, click **Yes**

Run this command:

```powershell
wsl --install
```

This installs WSL2 with Ubuntu. **Restart your computer** when prompted.

### Step 2: Set Up Ubuntu

After restart:

1. Ubuntu will open automatically (or search for "Ubuntu" in Start)
2. Wait for the installation to complete
3. Create a username and password when prompted

**Important:** Remember this password - you'll need it for `sudo` commands.

### Step 3: Install kvelmo

In the Ubuntu terminal:

```bash
curl -fsSL https://raw.githubusercontent.com/valksor/kvelmo/master/install.sh | bash
```

### Step 4: Verify and Install Agent

```bash
kvelmo version
# Install Claude or Codex CLI
```

### Accessing Windows Files

Your Windows files are at `/mnt/c/`:

- Desktop: `/mnt/c/Users/YourName/Desktop`
- Documents: `/mnt/c/Users/YourName/Documents`

Example:

```bash
cd /mnt/c/Users/YourName/Documents/myproject
kvelmo serve
```

---

## Build from Source

For developers who want to build from source:

### Prerequisites

- Go 1.23+
- Make
- Node.js 20+ (for web UI)
- bun (recommended) or npm

### Build

```bash
git clone https://github.com/valksor/kvelmo.git
cd kvelmo
make build
```

The binary is at `./build/kvelmo`.

### Install Locally

```bash
sudo mv ./build/kvelmo /usr/local/bin/
```

---

## Verification

After installation, verify everything works:

```bash
# Check kvelmo
kvelmo version

# Check agent CLI
claude --version  # or codex --version

# Start the server
kvelmo serve

# In another terminal, check status
kvelmo status
```

Open http://localhost:6337 to see the Web UI.

---

## Troubleshooting

### "command not found: kvelmo"

The binary isn't in your PATH. Try:

```bash
# Check if it exists
ls -la /usr/local/bin/kvelmo

# If not, reinstall
curl -fsSL https://raw.githubusercontent.com/valksor/kvelmo/master/install.sh | bash
```

### "permission denied"

You may need to make the binary executable:

```bash
chmod +x /usr/local/bin/kvelmo
```

### WSL: "Please enable Virtual Machine Platform"

1. Open **Turn Windows features on or off**
2. Enable **Virtual Machine Platform**
3. Restart your computer

### Agent CLI Not Working

Make sure your agent CLI is properly authenticated:

```bash
# For Claude
claude auth status

# For Codex
codex auth status
```

### Still Having Issues?

- Check [GitHub Issues](https://github.com/valksor/kvelmo/issues)
- Ask in discussions
- Review the [FAQ](/docs/faq.md)
