Audit kvelmo's feature surface for completeness and parity across all surfaces.

## Surface Parity Model

kvelmo has 5 user-facing surfaces with a **tiered parity model** (not flat 1:1):

| Surface | Scope | Notes |
|---------|-------|-------|
| **CLI** | **All** features | The superset. Some commands bypass socket (direct pkg calls). |
| **Web UI (panels/buttons)** | **Most** common actions | Dedicated buttons/panels for key workflows; user input/output flows through chat. |
| **Web Chat** | **Full parity** to CLI | Minus a few CLI-only commands (`completion`, `pipe`, `tui`, `serve`, `shutdown`). |
| **TUI** | **Full 1:1 parity** to web chat | Same chat interface, same capabilities. |
| **App (Tauri desktop)** | **Same as web** | Plus serves the kvelmo binary via Tauri sidecar. |

**Parity rules:**
- A feature in Go backend with no CLI path → **not implemented**
- A CLI feature missing from web chat → **gap** (unless inherently CLI-only)
- A web chat feature missing from TUI → **gap**
- A common workflow missing a web UI button/panel → **gap** (but not every CLI command needs a button)
- App should match web; additionally check sidecar binary serving

---

## Phase 1: Package-to-Surface Mapping

For each package in `pkg/`:

1. Read the package's exported API (service structs, public methods)
2. Check if it has:
   - **CLI command(s)** in `cmd/kvelmo/commands/`
   - **Socket RPC method(s)** registered in `pkg/socket/`
   - **Web Chat** coverage (can the action be invoked via chat?)
   - **Web UI** coverage in `web/src/` (dedicated panels/buttons for common actions)
   - **TUI** coverage in `pkg/tui/` (mirrors web chat)
3. Assess whether the package SHOULD have user-facing surface (some packages like `paths/` are internal-only)

Report gaps per the tiered parity model above — not every surface needs every feature, but the hierarchy must hold.

---

## Phase 2: Socket RPC Coverage

1. List all registered RPC methods in the socket server
2. For each method, verify:
   - A CLI command invokes it (or explain why not)
   - The web UI / web chat calls it via WebSocket (or explain why not)
3. Flag any "dead" RPC methods with no callers
4. Note: some CLI commands intentionally bypass socket (direct pkg calls) — this is acceptable, not a gap

---

## Phase 3: Surface Coverage Check

Apply the tiered parity model to find gaps:

### 3a. CLI Completeness (the superset)
1. List all CLI commands in `cmd/kvelmo/commands/`
2. Verify each has a working code path (socket RPC or direct pkg call)
3. Every user-facing feature must have a CLI path

### 3b. Web Chat ↔ CLI Parity
1. For each CLI command, check if the equivalent action can be performed via web chat
2. Known CLI-only exclusions (not gaps): `completion`, `pipe`, `tui`, `serve`, `shutdown`, `cleanup`
3. Flag any CLI command missing from web chat that isn't in the exclusion list

### 3c. TUI ↔ Web Chat Parity
1. For each web chat action, verify the TUI supports the same action
2. TUI should be full 1:1 with web chat — flag any divergence

### 3d. Web UI Panels/Buttons Coverage
1. List key workflows (task lifecycle, status, config, etc.)
2. Verify common actions have dedicated web UI buttons/panels (not just chat)
3. User input/output for these actions flows through chat — verify this wiring
4. Not every CLI command needs a button; focus on high-frequency workflows

### 3e. App (Tauri) Coverage
1. Verify the app wraps the full web surface
2. Check Tauri sidecar binary serving works
3. Flag any web features broken in the desktop app context

---

## Phase 4: Persona Alignment

Cross-reference the 8 persona gap analyses against actual features:

### Solo Developer
Should have: task loading from multiple sources, planning, implementation, review, undo/redo, PR submission
- Cross-check with `/solo-developer-gaps` goals

### Team Lead
Should have: multi-project visibility, worker pool monitoring, metrics, audit trail
- Cross-check with `/team-lead-gaps` goals

### Open Source Maintainer
Should have: GitHub provider, PR workflows, issue integration
- Cross-check with `/opensource-maintainer-gaps` goals

### DevOps/SRE
Should have: metrics, security scanning, configuration management, deployment configs
- Cross-check with `/devops-gaps` goals

### CLI Power User
Should have: composable commands, JSON output, shell completion, streaming
- Cross-check with `/cli-poweruser-gaps` goals

### Frontend Developer
Should have: full web UI coverage, real-time updates, visual diff, dashboard
- Cross-check with `/frontend-dev-gaps` goals

### Agent Developer
Should have: agent interface docs, event streaming, permission system, testing
- Cross-check with `/agent-dev-gaps` goals

### Enterprise
Should have: configuration management, access control, audit logging, backup
- Cross-check with `/enterprise-gaps` goals

---

## Phase 5: Documentation Freshness

Check that `docs/` stays in sync with the actual codebase. Stale or missing docs are a gap just like missing surface coverage.

### 5a. CLI Docs Coverage

1. List all command `.go` files in `cmd/kvelmo/commands/` (exclude `_test.go`)
2. List all doc pages in `docs/cli/`
3. Flag:
   - Commands that have no corresponding doc page in `docs/cli/`
   - Doc pages in `docs/cli/` for commands that no longer exist

### 5b. CLAUDE.md Accuracy

Verify these sections in `CLAUDE.md` match reality:

1. **Package Index table** — every row in the `pkg/` table should correspond to an actual directory in `pkg/`. Flag packages listed but missing, or packages that exist but aren't listed
2. **CLI Commands section** — commands mentioned should exist in `cmd/kvelmo/commands/`. Flag stale or missing entries
3. **Build & Development Commands** — verify `Makefile` targets listed actually exist in the `Makefile`

### 5c. Web UI Docs Coverage

1. List doc pages in `docs/web-ui/`
2. Cross-reference with actual views/components in `web/src/`
3. Flag doc pages describing features that no longer exist, or major web UI features with no docs

### 5d. Provider & Agent Docs

1. Compare `docs/providers/` pages against implementations in `pkg/provider/`
2. Compare `docs/agents/` pages against implementations in `pkg/agent/`
3. Flag doc pages for unimplemented providers/agents, or implementations without docs

### 5e. Sidebar Sync

1. Parse `docs/_sidebar.md` for all linked pages
2. Verify every linked page actually exists as a file in `docs/`
3. Verify every doc page in `docs/` is linked from the sidebar
4. Flag broken links and orphaned pages

---

## Output Format

For each gap found:

```
## [Category] Issue Title
- **Type**: Parity gap / Missing surface / Dead code / Persona gap
- **Surface**: Which surface(s) are affected
- **Evidence**: [file:line or observation]
- **Impact**: [what breaks or is missing for which persona]
- **Recommendation**: [specific action]
- **Effort**: [Fibonacci 1-13]
```

---

## Summary Checklist

- [ ] All `pkg/` packages assessed for user-facing surface
- [ ] All socket RPC methods have callers (CLI, web chat, or web UI)
- [ ] CLI covers all features (the superset)
- [ ] Web chat has full parity to CLI (minus known CLI-only exclusions)
- [ ] TUI has full 1:1 parity to web chat
- [ ] Web UI panels/buttons cover key workflows (input/output flows through chat)
- [ ] App wraps full web surface and serves binary via sidecar
- [ ] Each persona's core goals have corresponding features
- [ ] No dead RPC methods or unreachable code paths
- [ ] Every CLI command has a corresponding doc page in `docs/cli/`
- [ ] No stale doc pages for removed commands/features
- [ ] CLAUDE.md package index, CLI commands, and Makefile targets match reality
- [ ] Provider and agent docs match implementations
- [ ] `docs/_sidebar.md` has no broken links or orphaned pages
