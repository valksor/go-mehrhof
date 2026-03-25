# Regulated Developer Gap Analysis

Imagine — you are a developer in a large organization with strict engineering standards. Your team ships enterprise software, and every commit, branch, and PR follows enforced conventions. You have:

- **Mandatory commit formats** — every commit must match a pattern like `feat(auth): add SSO login [PROJ-1234]` or it gets rejected by hooks
- **Branch naming rules** — branches must follow `feature/PROJ-123-short-description` or CI won't run
- **Specs live in the repo** — architecture decisions, implementation plans, and task specs must be checked into `docs/` so they survive team turnover
- **PR templates are non-negotiable** — every PR fills a template with sections for "What changed", "How to test", "Related tickets", and "Rollback plan"
- **Approval gates everywhere** — nothing ships without sign-off; some transitions need explicit manager approval
- **Audit everything** — compliance requires knowing who did what, when, with which AI model, and why

Now you find kvelmo, a tool that orchestrates AI-assisted development — but your organization's rules are non-negotiable. The tool must adapt to your process, not the other way around.

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

### Goal 1: Enforced commit message format
Configure a commit message pattern (e.g., `type(scope): message [TICKET-ID]`) and kvelmo follows it for every commit it creates. Invalid formats are rejected before they reach git hooks.

### Goal 2: Branch naming rules
Auto-generate branch names from task metadata following org conventions. Loading a task from PROJ-1234 creates `feature/PROJ-1234-task-title-slug` automatically. Configurable pattern per project.

### Goal 3: In-repo specs and plans
When kvelmo generates a plan or spec, write it to the repo (e.g., `docs/specs/PROJ-1234.md`) — not just internal storage. These files are version-controlled, reviewable, and survive kvelmo being uninstalled.

### Goal 4: PR template compliance
Detect the repo's PR template (`.github/PULL_REQUEST_TEMPLATE.md`), parse its sections, and auto-fill them from task context. Required sections that can't be auto-filled are flagged before submission.

### Goal 5: Approval gates
Configure transitions that require explicit human approval. "Plan → Implement" might be auto-approved, but "Review → Submit" requires a human to type `kvelmo approve`. Configurable per project.

### Goal 6: Audit trail
Log every kvelmo action with timestamp, user identity, action taken, files affected, and AI model used. Exportable in standard formats (JSON, CSV) for compliance review.

---

## Phase 2: Extended Goals (8)

### Goal 7: Configurable workflow steps
Add, remove, or reorder lifecycle stages per project. Some teams need a "security review" step between implement and review. Others skip simplify entirely. The state machine should be configurable.

### Goal 8: Ticket system sync
Sync task status back to Jira, Linear, GitHub Issues, or Azure DevOps as the task progresses through kvelmo's lifecycle. "In Progress" when implementing, "In Review" when reviewing, "Done" when merged.

### Goal 9: Pre-commit hook compatibility
Kvelmo's commits must pass the project's existing pre-commit hooks (linters, formatters, secret scanners). If a hook fails, kvelmo fixes the issue and retries — not bypasses with `--no-verify`.

### Goal 10: Code review checklist enforcement
Define a review checklist (security, performance, accessibility, documentation) and require each item to be explicitly checked before the review step completes.

### Goal 11: Documentation requirements
Block PR submission unless documentation is updated when certain files change. Modifying an API endpoint requires updating the API docs. Configurable rules per file pattern.

### Goal 12: Changelog generation
Auto-generate CHANGELOG entries from task metadata, commit messages, and PR descriptions. Follow Keep a Changelog format or a custom template.

### Goal 13: Environment-specific configs
Different rules per project or repo within a monorepo. The backend team uses conventional commits, the frontend team uses a different format. Kvelmo respects per-directory configuration.

### Goal 14: Compliance reports
Generate reports of all AI-assisted development activity: tasks completed, AI models used, human approval points, files modified, time spent. For quarterly compliance reviews and stakeholder updates.

---

## Phase 2: Critical Audit

The 14 goals above are a starting point, not a ceiling. Investigate deeper across these dimensions:

1. **Real-world friction**: Where does kvelmo conflict with common org-enforced workflows? What makes regulated teams reject the tool?
2. **Missing primitives**: What corporate development operations are awkward or impossible in kvelmo?
3. **Error & recovery gaps**: When kvelmo creates a commit that fails hooks, is recovery clean or does it leave broken state?
4. **Scalability cliffs**: At what team/project size does kvelmo's configuration management become unmanageable?
5. **Observability blindspots**: Can compliance officers verify what kvelmo did without developer assistance?
6. **Workflow completeness**: Are there mandatory corporate workflows (sign-offs, gates, approvals) that kvelmo can't model?
7. **Integration gaps**: What enterprise tools (Jira, Azure DevOps, ServiceNow, Confluence) does kvelmo need to connect to?
8. **Data ownership & portability**: Can the organization export all kvelmo data if they switch tools? Are audit logs complete?
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
- `/pragmatic-dev-gaps`
- `/regulated-dev-gaps` (this command)

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
4. **Full-stack implementation** — every recommended feature MUST be wired per the tiered parity model. The architecture is hub-and-spoke: Go package implements logic, Socket RPC exposes it, then surfaces consume it. For each new feature, specify:
   - **Go package** (`pkg/<feature>/`) + handler wiring
   - **Socket RPC method** registered in socket server (the hub that surfaces call into)
   - **CLI command** in `cmd/kvelmo/commands/` — calls socket RPC (or bypasses it for some commands); required for all features (the superset)
   - **Web chat** — calls socket RPC via WebSocket; required unless inherently CLI-only
   - **Web UI panel/button** — calls socket RPC via WebSocket; for common workflows; input/output flows through chat
   - **Web UI store** update in `web/src/stores/` (if panel/button needed)
   - **TUI** in `pkg/tui/` — must match web chat 1:1
   - **App** — wraps web surface; verify works in Tauri desktop context
   - A feature without a CLI path is not complete. A feature in CLI but not web chat is a gap (unless CLI-only by nature).
5. **Name by function, not domain** — packages, RPC methods, CLI commands, and frontend components must be named for what they DO, not which persona inspired them. Litmus test: "Would a user from a DIFFERENT persona find this name sensible?" Domain-specific terminology belongs in help text and documentation, NOT in code identifiers.
