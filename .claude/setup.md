# Claudify Global Setup

Instead of installing Claudify per-project (which dumps ~1,800 files into each repo), this setup installs it globally so all projects share one copy.

## Prerequisites

- Claude Code installed (`claude --version`)
- `jq` installed (`brew install jq`) — hooks need it for safety checks

## 1. Create shared directory

All Claudify assets live in `~/.shared/`. Both `~/.claude/` and `~/.plaude/` symlink here.

```bash
mkdir -p ~/.shared/agents ~/.shared/hooks
```

## 2. Copy Claudify source files into ~/.shared/

From your Claudify download:

```bash
CLAUDIFY=/path/to/claudify-download

# Agents (9 files)
cp "$CLAUDIFY/.claude/agents/"*.md ~/.shared/agents/

# Commands (21 files)
cp "$CLAUDIFY/.claude/commands/"*.md ~/.shared/commands/

# Hooks (9 files)
cp "$CLAUDIFY/.claude/hooks/"*.sh ~/.shared/hooks/
chmod +x ~/.shared/hooks/*.sh

# Command index
cp "$CLAUDIFY/.claude/command-index.md" ~/.shared/command-index.md
```

## 3. Create symlinks in ~/.claude/

```bash
ln -sfn ~/.shared/agents ~/.claude/agents
ln -sfn ~/.shared/hooks ~/.claude/hooks
ln -sfn ~/.shared/commands ~/.claude/commands      # if not already symlinked
ln -sfn ~/.shared/command-index.md ~/.claude/command-index.md
```

## 4. Create symlinks in ~/.plaude/

```bash
ln -sfn ~/.shared/agents ~/.plaude/agents
ln -sfn ~/.shared/hooks ~/.plaude/hooks
ln -sfn ~/.shared/commands ~/.plaude/commands      # if not already symlinked
ln -sfn ~/.shared/command-index.md ~/.plaude/command-index.md
```

## 5. Add hooks to global settings.json

Merge this into both `~/.claude/settings.json` and `~/.plaude/settings.json`.
Replace `/Users/YOU` with your actual home directory.

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "/Users/YOU/.shared/hooks/guard-bash.sh",
            "statusMessage": "Checking command safety..."
          }
        ]
      },
      {
        "matcher": "Write|Edit|NotebookEdit",
        "hooks": [
          {
            "type": "command",
            "command": "/Users/YOU/.shared/hooks/backup-before-write.sh",
            "async": true,
            "statusMessage": "Backing up before write..."
          },
          {
            "type": "command",
            "command": "/Users/YOU/.shared/hooks/completeness-gate.sh",
            "async": false,
            "statusMessage": "Validating content completeness..."
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Write|Edit|NotebookEdit",
        "hooks": [
          {
            "type": "command",
            "command": "/Users/YOU/.shared/hooks/log-changes.sh",
            "async": true,
            "statusMessage": "Logging change..."
          }
        ]
      }
    ],
    "PostToolUseFailure": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "/Users/YOU/.shared/hooks/log-failures.sh",
            "async": true,
            "statusMessage": "Logging failure..."
          }
        ]
      }
    ],
    "PreCompact": [
      {
        "matcher": "auto",
        "hooks": [
          {
            "type": "command",
            "command": "/Users/YOU/.shared/hooks/pre-compact-handoff.sh",
            "timeout": 5,
            "statusMessage": "Saving state before compaction..."
          }
        ]
      }
    ],
    "SessionStart": [
      {
        "matcher": "user",
        "hooks": [
          {
            "type": "command",
            "command": "/Users/YOU/.shared/hooks/session-reset.sh",
            "timeout": 3,
            "statusMessage": "Resetting session state..."
          }
        ]
      },
      {
        "matcher": "compact",
        "hooks": [
          {
            "type": "command",
            "command": "/Users/YOU/.shared/hooks/post-compact-resume.sh",
            "timeout": 10,
            "statusMessage": "Restoring context after compaction..."
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "prompt",
            "prompt": "You are a JSON-only response bot. Output raw JSON with no markdown, no code fences, no prose. Review this conversation and return exactly one JSON object: {\"decision\":\"allow\",\"learning\":null,\"task_type\":\"other\"} — decision is \"allow\" if all user-requested tasks were completed, \"block\" if something was missed (add \"reason\" field). learning is a one-sentence lesson if errors were resolved (root cause + fix), otherwise null. task_type is one of: build, debug, refactor, test, docs, research, deploy, admin, setup, other. RESPOND WITH ONLY THE JSON OBJECT. NO OTHER TEXT.",
            "model": "haiku",
            "timeout": 15
          },
          {
            "type": "command",
            "command": "/Users/YOU/.shared/hooks/log-stop-verdict.sh",
            "async": true,
            "statusMessage": "Logging session verdict..."
          }
        ]
      }
    ]
  }
}
```

## 6. Add MCP servers to both tools

Create `~/.claude/.mcp.json` and `~/.plaude/.mcp.json` with:

```json
{
  "mcpServers": {
    "context7": {
      "command": "npx",
      "args": ["-y", "@upstash/context7-mcp@latest"],
      "description": "Live library documentation (Next.js, React, any npm package)"
    },
    "memory": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-memory"],
      "description": "Persistent knowledge graph for cross-session facts"
    }
  }
}
```

## 7. Create global CLAUDE.md

Copy `~/.claude/CLAUDE.md` and `~/.plaude/CLAUDE.md` with the Claudify operational sections (Quick Start, Key Files, System Architecture, Memory Architecture, Command Awareness, Retrieval Map, Context Health, Maintenance). See the existing ones on your main machine as reference.

## What's NOT global

These are per-project state and stay in each project's `.claude/`:

- `memory.md` — session context
- `knowledge-base.md` — learned rules
- `knowledge-nominations.md` — candidate learnings
- `agent-memory/` — per-agent persistent knowledge
- `logs/` — audit trail
- `backups/` — file backups

These get created naturally by hooks and commands as you work.

## What we skipped from Claudify

- **skills/** (1,728 files) — generic industry templates (agriculture, healthcare, etc.). Not installed.
- **per-project .mcp.json** — moved to global
- **per-project .claude/settings.json** — hooks moved to global settings
- **Daily Notes/, Task Board.md, Scratchpad.md** — optional per-project workspace files, create if you want them

## Final structure

```
~/.shared/
  agents/          9 agent definitions
  commands/       21 slash commands
  hooks/           9 hook scripts
  command-index.md

~/.claude/
  agents          → ~/.shared/agents (symlink)
  commands        → ~/.shared/commands (symlink)
  hooks           → ~/.shared/hooks (symlink)
  command-index.md → ~/.shared/command-index.md (symlink)
  CLAUDE.md        global instructions
  settings.json    hooks config + permissions
  .mcp.json        MCP servers

~/.plaude/
  (same structure as ~/.claude/)
```
