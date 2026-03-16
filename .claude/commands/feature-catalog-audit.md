Audit kvelmo's feature surface for completeness and parity across CLI, web UI, and socket RPC.

---

## Phase 1: Package-to-Surface Mapping

For each package in `pkg/`:

1. Read the package's exported API (service structs, public methods)
2. Check if it has:
   - **CLI command(s)** in `cmd/kvelmo/commands/`
   - **Socket RPC method(s)** registered in `pkg/socket/`
   - **Web UI coverage** in `web/src/`
3. Assess whether the package SHOULD have user-facing surface (some packages like `paths/` are internal-only)

Report any packages that have CLI but no web UI, or web UI but no CLI (violates parity rule in CLAUDE.md).

---

## Phase 2: Socket RPC Coverage

1. List all registered RPC methods in the socket server
2. For each method, verify:
   - A CLI command invokes it (or explain why not)
   - The web UI calls it via WebSocket (or explain why not)
3. Flag any "dead" RPC methods with no callers

---

## Phase 3: CLI/Web Parity Check

Per CLAUDE.md: "CLI and web UI must maintain feature parity; never ship one without the other."

1. List all CLI commands in `cmd/kvelmo/commands/`
2. For each command, check if the equivalent action exists in the web UI
3. List all web UI actions/buttons/workflows
4. For each action, check if the equivalent CLI command exists
5. Report gaps in either direction

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

Check that `docs/` stays in sync with the actual codebase. Stale or missing docs are a gap just like missing CLI/web parity.

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
- **Evidence**: [file:line or observation]
- **Impact**: [what breaks or is missing for which persona]
- **Recommendation**: [specific action]
- **Effort**: [Fibonacci 1-13]
```

---

## Summary Checklist

- [ ] All `pkg/` packages assessed for user-facing surface
- [ ] All socket RPC methods have CLI and/or web UI callers
- [ ] No CLI-only features (must have web UI equivalent)
- [ ] No web-UI-only features (must have CLI equivalent)
- [ ] Each persona's core goals have corresponding features
- [ ] No dead RPC methods or unreachable code paths
- [ ] Every CLI command has a corresponding doc page in `docs/cli/`
- [ ] No stale doc pages for removed commands/features
- [ ] CLAUDE.md package index, CLI commands, and Makefile targets match reality
- [ ] Provider and agent docs match implementations
- [ ] `docs/_sidebar.md` has no broken links or orphaned pages
