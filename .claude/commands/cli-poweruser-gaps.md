# CLI Power User Gap Analysis

Imagine — you are a CLI power user. You live in tmux, have hundreds of shell aliases, and judge tools by their Unix philosophy adherence. You've been using AI assistants but find most interfaces clunky. You have:

- **Flow-breaking web UIs** — that require mouse clicks and context switches away from your terminal
- **Non-composable tools** — that don't work with pipes, standard streams, or shell scripting
- **No scriptable automation** — can't orchestrate complex workflows without writing plugins
- **Scattered configuration** — across multiple files and formats with no single source of truth
- **Verbose commands** — when they should be terse; too many flags for common operations
- **No shell integration** — no tab completion, no Fish/Zsh support worth using
- **Hand-holding UIs** — that assume you want guidance instead of power

Now you find kvelmo, a tool that promises socket-first IPC, a CLI with 50+ commands, JSON-RPC protocol, and a web UI that's optional — not mandatory.

You are excited. You want to use it. **Can you?**

Critically — can you use kvelmo to achieve these goals:

---

## Phase 1: Core Goals (6)

For each goal, assess:
- **Status**: fully / partially / not at all
- **Surface check** — for every feature that supports this goal, verify it exists on ALL applicable surfaces:
  - [ ] **CLI** (`cmd/kvelmo/commands/`) — command exists and is wired to socket RPC
  - [ ] **Web UI** (`web/src/`) — component/widget/panel exists and is wired to a Zustand store
  - [ ] **TUI** (`pkg/tui/`) — view/panel exists (if the feature is status/monitoring/dashboard related)
  - [ ] **Socket RPC** (`pkg/socket/`) — method registered and called by both CLI and web UI
  - A feature that exists only in Go backend with no CLI/web/TUI surface is **not implemented** from the user's perspective. Mark it "not at all" or "partially" accordingly.
- **What exists**: current kvelmo features that help — **list which surfaces have coverage** (e.g., "CLI only", "CLI + web", "Go backend only")
- **Gap**: what's missing — **explicitly note missing surfaces** (e.g., "no web UI component", "no TUI panel")
- **Recommendation**: what to build (Fibonacci effort: 1, 2, 3, 5, 8, 13)

### Goal 1: Composable commands
Commands that accept stdin, produce stdout, and compose with pipes. `kvelmo plan | review | implement`.

### Goal 2: Powerful completion
Tab completion that understands context—tasks, states, providers, options. Fish/Zsh/Bash support.

### Goal 3: Scriptable automation
Write shell scripts that orchestrate kvelmo. Exit codes, JSON output, machine-parseable formats.

### Goal 4: Minimal keystrokes
Short command names, sensible defaults, remember my preferences. `kv p` not `kvelmo plan --verbose --with-context`.

### Goal 5: Terminal UI
When needed, a TUI that doesn't require leaving the terminal. Keyboard-driven, no mouse required.

### Goal 6: Dotfile configuration
Single config file, environment variable overrides, XDG compliance. No wizard setup.

---

## Phase 2: Extended Goals (8)

### Goal 7: tmux/screen integration
Aware of terminal multiplexers. Session management, pane coordination.

### Goal 8: Editor integration
Work with vim/neovim/emacs. Send context, receive results, navigate to changes.

### Goal 9: Streaming output
Real-time agent output without buffering. `--follow` flags, live tailing.

### Goal 10: Offline help
Man pages, `--help` that's actually helpful, examples in documentation.

### Goal 11: Aliases and shortcuts
User-defined aliases, command abbreviations, workflow macros.

### Goal 12: Quiet and verbose modes
`-q` for scripts, `-v` for debugging. Respect `$TERM` and `$NO_COLOR`.

### Goal 13: Performance
Commands start fast. No 2-second startup times. Lazy loading where needed.

### Goal 14: Unix philosophy
Do one thing well. Don't reinvent grep, git, or jq. Compose with existing tools.

---

## Phase 2: Critical Audit

The 14 goals above are a starting point, not a ceiling. Investigate deeper across these dimensions:

1. **Real-world friction**: What makes CLI users abandon tools? Where does kvelmo violate Unix conventions?
2. **Missing primitives**: What command-line operations are awkward or impossible?
3. **Error & recovery gaps**: Are error messages actionable? Can you fix issues without a web UI?
4. **Scalability cliffs**: Does kvelmo stay fast with large task histories?
5. **Observability blindspots**: Can you understand kvelmo's state from the terminal alone?
6. **Workflow completeness**: Are there operations that require leaving the terminal?
7. **Integration gaps**: What terminal tools does kvelmo need to work with?
8. **Data ownership & portability**: Can you access kvelmo data with standard Unix tools?
9. **Surface parity**: For every feature found in Phase 1, is it accessible from CLI, web UI, and TUI (where applicable)? A Go function without a CLI command or web button is invisible to the user. List every feature that exists on one surface but not the others — these are gaps even if the Go backend is fully implemented.

Report all gaps found—whether 3 or 30. Each gap should include severity and a recommended fix.

---

## Phase 3: Implementation Planning

Do NOT stop at analysis. The purpose of this command is to produce an actionable plan for closing the gaps found above.

### Step 1: Prioritize

From all gaps discovered (Phase 1 goals, Phase 2 critical audit), create a single prioritized list sorted by:
1. **Impact** — how many goals does this unblock or improve?
2. **Effort** — lower effort first within same impact tier
3. **Dependencies** — what must be built before what?

Group into tiers:
- **Quick wins** (effort 1-3): Can be implemented immediately
- **Medium** (effort 5): Meaningful work, clear scope
- **Large** (effort 8-13): Needs design decisions, may span multiple packages

### Step 2: Plan Each Gap

For each gap (starting from highest priority), produce a concrete implementation plan:
- **What to build**: One-sentence description
- **Files to create/modify**: Specific paths following the project's package structure
- **Wiring**: Full-stack wiring per Critical Rule 4 (Go package → socket RPC → CLI command → web store → web component → route)
- **Test strategy**: What to test and how
- **Risks/dependencies**: What could block this or what must exist first
- **Surface checklist**: Before marking any gap as "planned," confirm the plan covers:
  - [ ] CLI command (or explicit reason why not)
  - [ ] Web UI component + Zustand store wiring (or explicit reason why not)
  - [ ] TUI view (for status/monitoring/dashboard features, or explicit reason why not)
  - [ ] Socket RPC method connecting backend to all surfaces
  - **Plans missing surfaces are incomplete. Do not proceed to Step 3 with incomplete plans.**

### Step 3: Enter Plan Mode

After producing the implementation plan, enter plan mode (`/plan`) to align with the user on which gaps to tackle first. Do not implement without user approval.

The goal is a ready-to-execute plan, not a report that ends with "analysis complete."

---

## Sibling Commands

This command is part of a family of 10 persona-specific gap analyses:

- `/solo-developer-gaps`
- `/team-lead-gaps`
- `/opensource-maintainer-gaps`
- `/devops-gaps`
- `/cli-poweruser-gaps` (this command)
- `/frontend-dev-gaps`
- `/agent-dev-gaps`
- `/enterprise-gaps`
- `/pragmatic-dev-gaps`
- `/regulated-dev-gaps`

---

## Shared Infrastructure

All personas share these 10 core systems. When recommending features, these must not be removed, narrowed, or domain-specific without cross-persona review:

1. **Socket layer** (`pkg/socket/` — global + worktree sockets)
2. **State machine** (`pkg/conductor/` — lifecycle states, guards, transitions)
3. **Agent interface** (`pkg/agent/` — WebSocket + CLI modes)
4. **Worker pool** (`pkg/worker/` — job queue, event streaming)
5. **Providers** (`pkg/provider/` — GitHub, GitLab, Wrike, Linear, file)
6. **Storage** (`pkg/storage/` — tasks, chats, plans, reviews)
7. **Git integration** (`pkg/git/` — checkpoints, undo/redo)
8. **Web UI** (`web/` — React 19, stores, real-time updates)
9. **CLI commands** (`cmd/kvelmo/commands/`)
10. **Memory system** (`pkg/memory/` — embeddings, semantic search)

### Critical Rules

1. **Extend, don't replace**: New features add alongside existing ones. Never remove capabilities.
2. **Domain-agnostic core**: Features benefiting one persona should work for all unless explicitly domain-specific.
3. **Check the siblings**: Before recommending a feature, verify it doesn't break other personas' workflows.
4. **Full-stack implementation** — every recommended feature MUST be wired end-to-end. For each new feature, specify:
   - **Go package** (`pkg/<feature>/`) + handler wiring
   - **Socket RPC method** registered in socket server
   - **Web UI store** update in `web/src/stores/`
   - **Web UI component** (widget, panel, or page)
   - **Route + navigation** wiring in web frontend
   - **CLI command** in `cmd/kvelmo/commands/` (if user-facing)
   - **TUI view** in `pkg/tui/` (for status, monitoring, and dashboard features)
   - A feature without both CLI and web UI is not complete (per CLAUDE.md parity rule). If a feature is backend-only by nature, explicitly note why.
5. **Name by function, not domain** — packages, RPC methods, CLI commands, and frontend components must be named for what they DO, not which persona inspired them. Litmus test: "Would a user from a DIFFERENT persona find this name sensible?" Domain-specific terminology belongs in help text and documentation, NOT in code identifiers.
