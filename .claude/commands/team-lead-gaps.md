# Team Lead Gap Analysis

Imagine — you are a team lead managing 4-8 engineers on a product team. You've adopted AI-assisted development and now face new coordination challenges. You have:

- **Inconsistent outputs** — engineers using different AI tools with no standard approach
- **No visibility** — what AI agents are actually doing across the team is a black box
- **Review bottlenecks** — AI-generated code needs extra scrutiny, slowing your pipeline
- **Sprint chaos** — planning doesn't account for AI's impact on velocity
- **Onboarding friction** — new engineers take longer than expected to learn AI-assisted workflows
- **Knowledge silos** — which prompts work best for your codebase lives in individual heads
- **Quality uncertainty** — hard to tell if AI is helping or creating tech debt faster

Now you find kvelmo, a tool that promises to orchestrate AI-assisted development with full lifecycle visibility, consistent workflows, and checkpoint-based safety.

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

### Goal 1: Team-wide visibility
See all active kvelmo tasks across the team. Know who's working on what, what state they're in, what agents are running.

### Goal 2: Consistent workflows
Enforce team standards—same planning depth, same review requirements, same commit practices. Configurable guardrails.

### Goal 3: Code review integration
Surface AI-generated changes with appropriate context. Help reviewers understand what the AI did and why.

### Goal 4: Progress tracking
Track task completion rates, agent success rates, and common failure modes. Data for retrospectives.

### Goal 5: Knowledge sharing
Share effective prompts, task templates, and patterns across the team. Collective intelligence.

### Goal 6: Risk visibility
Flag high-risk AI operations before they happen. Security-sensitive code, critical paths, production systems.

---

## Phase 2: Extended Goals (8)

### Goal 7: Onboarding workflows
Guided paths for new team members learning kvelmo. Progressive disclosure of capabilities.

### Goal 8: Workload distribution
See which engineers are overloaded with active tasks. Balance AI-assisted work.

### Goal 9: Quality metrics
Track code quality trends—are AI-assisted PRs getting approved faster? Creating more bugs? Better test coverage?

### Goal 10: Audit trail
Complete history of who ran what agents with what prompts. Compliance and debugging.

### Goal 11: Permission management
Control who can run which agents, access which providers, modify which configurations.

### Goal 12: Integration with project management
Sync with Jira, Linear, Asana. Tasks flow in, status flows out.

### Goal 13: Team templates
Shared task templates, prompt libraries, and workflow presets. Standardization without rigidity.

### Goal 14: Cross-project coordination
When the team works on multiple repos, maintain coherent views and shared context.

---

## Phase 2: Critical Audit

The 14 goals above are a starting point, not a ceiling. Investigate deeper across these dimensions:

1. **Real-world friction**: What makes team leads abandon tools? Where does kvelmo create management overhead?
2. **Missing primitives**: What coordination operations are awkward or impossible?
3. **Error & recovery gaps**: When one engineer's kvelmo fails, how does it affect the team?
4. **Scalability cliffs**: At what team size does kvelmo's coordination break down?
5. **Observability blindspots**: Can leads understand team-wide AI activity patterns?
6. **Workflow completeness**: Are there gaps between kvelmo and existing team tools (CI, PM, chat)?
7. **Integration gaps**: What team infrastructure does kvelmo need to connect to?
8. **Data ownership & portability**: Can the team migrate away from kvelmo without losing history?
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
- `/team-lead-gaps` (this command)
- `/opensource-maintainer-gaps`
- `/devops-gaps`
- `/cli-poweruser-gaps`
- `/frontend-dev-gaps`
- `/agent-dev-gaps`
- `/enterprise-gaps`
- `/pragmatic-dev-gaps`
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
