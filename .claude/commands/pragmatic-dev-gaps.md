# Pragmatic Developer Gap Analysis

Imagine — you are an experienced developer who's been shipping software for years. You're equally comfortable in the terminal and the browser, switching between CLI and web UI without thinking. You have:

- **No patience for ceremony** — if a tool makes you click through 5 dialogs to start work, you'll close it and use the terminal
- **Multiple projects in flight** — you context-switch daily and need to know instantly where each project stands
- **Strong opinions, loosely held** — you'll skip planning when the task is obvious, but want it available when it's not
- **CLI and web interchangeably** — whichever is faster in the moment wins; you expect them to show the same state
- **Speed over polish** — you'd rather ship a rough PR fast and iterate than perfect it before submitting
- **Zero tolerance for friction** — extra prompts, confirmation dialogs, and mandatory steps that add no value make you abandon tools

Now you find kvelmo, a tool that orchestrates your development workflow — task loading, planning, implementation, review, and PR submission.

You are excited. You want to use it. **Can you?**

Critically — can you use kvelmo to achieve these goals:

---

## Phase 1: Core Goals (6)

For each goal, assess:
- **Status**: fully / partially / not at all
- **Surface check**: See Critical Rule 4 below for the tiered parity model (CLI ⊇ web chat ⊇ TUI ⊇ web UI)
- **What exists**: current kvelmo features that help — **list which surfaces have coverage** (e.g., "CLI only", "CLI + web", "Go backend only")
- **Gap**: what's missing — **explicitly note missing surfaces** (e.g., "no web UI component", "no TUI panel")
- **Recommendation**: what to build (Fibonacci effort: 1, 2, 3, 5, 8, 13)

### Goal 1: Zero-friction task start
Load a task from a GitHub issue URL, a file path, or inline text in under 5 seconds. One command, no wizards, no follow-up prompts.

### Goal 2: Switch freely between CLI and web
Start a task in the terminal, check progress in the browser, intervene from either. Same state, no drift, no sync issues.

### Goal 3: Skip optional steps
Jump straight to implement when planning is unnecessary. Skip simplify and optimize when the code is already clean. The workflow should be flexible, not a forced march.

### Goal 4: Instant status at a glance
One command (`kvelmo status`) or one screen (web dashboard) tells you exactly where every active task stands — state, progress, blockers. No digging.

### Goal 5: Fast undo without thinking
Undo the last action immediately. No "are you sure?" dialogs. No selecting from a list of checkpoints. Just undo. If you want more control, checkpoints are there, but the default is fast.

### Goal 6: Ship PR in one command
`kvelmo submit` generates the PR description from task context, fills in what was planned vs. implemented, and creates the PR. Zero manual editing needed for routine PRs.

---

## Phase 2: Extended Goals (8)

### Goal 7: Batch operations
Act on multiple tasks at once — submit all reviewed tasks, pause everything, reset failed tasks. Bulk actions for bulk workflows.

### Goal 8: Keyboard shortcuts in web UI
Navigate the web dashboard without touching the mouse. Vim-style or customizable shortcuts for common actions — next task, approve, submit, undo.

### Goal 9: Customizable defaults
Set preferred agent, default skip steps, auto-approve patterns. Per-project or global. `kvelmo config set default-agent claude` and never think about it again.

### Goal 10: Quick context dump
Export the current task state — plan, changes, chat history — as a shareable artifact. Useful for debugging, handoffs, or "what did I do yesterday?"

### Goal 11: Aliases and shortcuts
`kvelmo i` = `kvelmo implement`. `kvelmo s` = `kvelmo status`. Custom user-defined aliases that match your muscle memory.

### Goal 12: Notification preferences
Only alert on failures and blockers, not routine progress. Configurable per-project. Silent mode for when you're in flow.

### Goal 13: Template tasks
Reuse common task shapes — "bug fix", "feature", "refactor" — with pre-filled fields and default settings. Skip repetitive setup.

### Goal 14: History search
Find past tasks by keyword, file touched, or date range. Not by ID. "What did I work on in the auth module last week?"

---

## Phase 2: Critical Audit

The 14 goals above are a starting point, not a ceiling. Investigate deeper across these dimensions:

1. **Real-world friction**: Where does kvelmo add steps that a pragmatic dev would skip? What makes power users abandon the tool?
2. **Missing primitives**: What basic operations require multiple commands when one would do?
3. **Error & recovery gaps**: When something breaks mid-workflow, can you recover without losing progress?
4. **Scalability cliffs**: At what point does managing many concurrent tasks become unwieldy?
5. **Observability blindspots**: Can you tell what kvelmo is doing without reading logs?
6. **Workflow completeness**: Are there "last mile" gaps between kvelmo and actually shipping code?
7. **Integration gaps**: Does kvelmo work with the tools pragmatic devs already use (editors, terminals, browsers)?
8. **Data ownership & portability**: Can you get your task history out if you stop using kvelmo?
9. **Surface parity**: For every feature found in Phase 1, does it exist on the appropriate surfaces per the tiered parity model? CLI is the superset, web chat should match CLI (minus CLI-only commands), TUI should match web chat 1:1, and common workflows need web UI buttons/panels. A Go function without a CLI path is not implemented. List features that violate the tier expectations — these are gaps even if the Go backend is fully implemented.

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
- **Wiring**: Full-stack wiring per Critical Rule 4 (Go package → Socket RPC → {CLI, web chat, web UI, TUI, app})
- **Test strategy**: What to test and how
- **Risks/dependencies**: What could block this or what must exist first
- **Surface checklist**: Before marking any gap as "planned," confirm the plan covers the appropriate surfaces per the tiered parity model:
  - [ ] CLI command — required for all features (the superset)
  - [ ] Socket RPC method (or explicit reason why the CLI command bypasses socket)
  - [ ] Web chat support — required unless inherently CLI-only (`completion`, `pipe`, `tui`, `serve`, `shutdown`)
  - [ ] Web UI panel/button — required for common workflows (not every action needs a button)
  - [ ] TUI support — must match web chat 1:1
  - [ ] App — verify works in Tauri desktop context
  - **Plans missing required surfaces are incomplete. Do not proceed to Step 3 with incomplete plans.**

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
- `/cli-poweruser-gaps`
- `/frontend-dev-gaps`
- `/agent-dev-gaps`
- `/enterprise-gaps`
- `/pragmatic-dev-gaps` (this command)
- `/regulated-dev-gaps`

---

## Shared Infrastructure

All personas share these 10 core systems. When recommending features, these must not be removed, narrowed, or domain-specific without cross-persona review:

1. **Socket layer** (`internal/socket/` — global + worktree sockets)
2. **State machine** (`internal/conductor/` — lifecycle states, guards, transitions)
3. **Agent interface** (`agent/` — WebSocket + CLI modes)
4. **Worker pool** (`internal/worker/` — job queue, event streaming)
5. **Providers** (`internal/provider/` — GitHub, GitLab, Wrike, Linear, file)
6. **Storage** (`internal/storage/` — tasks, chats, plans, reviews)
7. **Git integration** (`internal/git/` — checkpoints, undo/redo)
8. **Web UI** (`web/` — React 19, stores, real-time updates)
9. **CLI commands** (`cmd/kvelmo/commands/`)
10. **Memory system** (`internal/memory/` — embeddings, semantic search)

### Critical Rules

1. **Extend, don't replace**: New features add alongside existing ones. Never remove capabilities.
2. **Domain-agnostic core**: Features benefiting one persona should work for all unless explicitly domain-specific.
3. **Check the siblings**: Before recommending a feature, verify it doesn't break other personas' workflows.
4. **Full-stack implementation** — every recommended feature MUST be wired per the tiered parity model. The architecture is hub-and-spoke: Go package implements logic, Socket RPC exposes it, then surfaces consume it. For each new feature, specify:
   - **Go package** (`internal/<feature>/`) + handler wiring
   - **Socket RPC method** registered in socket server (the hub that surfaces call into)
   - **CLI command** in `cmd/kvelmo/commands/` — calls socket RPC (or bypasses it for some commands); required for all features (the superset)
   - **Web chat** — calls socket RPC via WebSocket; required unless inherently CLI-only
   - **Web UI panel/button** — calls socket RPC via WebSocket; for common workflows; input/output flows through chat
   - **Web UI store** update in `web/src/stores/` (if panel/button needed)
   - **TUI** in `internal/tui/` — must match web chat 1:1
   - **App** — wraps web surface; verify works in Tauri desktop context
   - A feature without a CLI path is not complete. A feature in CLI but not web chat is a gap (unless CLI-only by nature).
5. **Name by function, not domain** — packages, RPC methods, CLI commands, and frontend components must be named for what they DO, not which persona inspired them. Litmus test: "Would a user from a DIFFERENT persona find this name sensible?" Domain-specific terminology belongs in help text and documentation, NOT in code identifiers.
