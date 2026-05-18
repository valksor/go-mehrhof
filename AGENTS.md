This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

# IT IS YEAR 2026 !!! Please use 2026 in web searches!!!
## No time estimates. Never say "this will take 1 day" or "a few weeks" - these are always wrong. If you must indicate complexity, use Fibonacci numbers (1, 2, 3, 5, 8, 13) for relative effort.
## PROJECT USES BUN NOT NODE OR NPM! PLEASE USE BUN OR BUNX WHEN CALLING SCRIPTS!

## Project Overview

kvelmo is a socket-first task lifecycle orchestration system for AI-assisted development. It manages the complete lifecycle of development tasks from loading requirements through implementation to PR submission, using Unix domain sockets for inter-process communication between CLI, web UI, and AI agents.

**Note:** The `prototype/` directory contains a fully working prototype implementation of kvelmo. This code is **read-only reference material** - not in active development. Use it to understand patterns and approaches when needed.

## What kvelmo Does (Not What It Is)

kvelmo is a **development orchestrator** - it doesn't write code itself, it manages the lifecycle of AI agents writing code in OTHER projects.

**The flow:**
1. User loads a task (from GitHub issue, file, Linear, etc.)
2. kvelmo spawns an AI agent (Claude, Codex, custom) in a worktree
3. Agent plans → implements → simplifies → optimizes the code
4. kvelmo manages checkpoints, reviews, and PR submission

**When working on kvelmo itself:**
- You're modifying the orchestrator, not a target project
- Changes should affect how kvelmo manages workflows
- Don't confuse kvelmo's internal state with user project state

## Build & Development Commands

```bash
# Development
make dev                # Quality + test + run (full dev workflow)
make run                # Build and run (sockets + web UI)
make run-dev            # Run without rebuilding
make web-dev            # Vite dev server with hot reload (port 5173)

# Quality & Testing
make quality            # Full-stack: Go fmt/vet/lint + frontend lint/typecheck
make test               # Full-stack: Go tests + frontend unit tests
make test-e2e           # E2E tests (SUITE=provider|gitlab|workflow|cli|all)
make test-cover         # Go coverage report → coverage.html
make test-race          # Go tests with race detector
make web-test           # Frontend unit tests only
make web-e2e            # Frontend e2e tests (demo mode)
make web-e2e-ui         # Interactive Playwright e2e UI

# Build & Release
make build              # Full build: web + Go binary → ./build/kvelmo
make build-go           # Go-only build (faster, skip web)
make web-build          # Frontend build only (Vite production)
make release            # Release binaries for all platforms
make install            # Install to ~/.local/bin
make ci                 # quality + test + build
make version            # Show current version info

# Desktop (Tauri)
make desktop-dev        # Tauri dev mode with hot reload
make desktop-build      # Production desktop app build
make desktop-sidecar    # Prepare sidecar binary (current platform)
make desktop-sidecar-all # Prepare sidecar binaries (all platforms)
make desktop-clean      # Remove desktop build artifacts
make tauri-install      # Install Tauri CLI prerequisites

# Setup & Utilities
make deps               # Download Go module dependencies
make types              # Generate TypeScript types from Go (tygo)
make man-pages          # Generate man pages
make help               # List all available targets

# Cleanup
make clean              # Remove all build artifacts
make tidy               # Clean and tidy dependencies

# Prototype Management
make prototype-lock     # Lock prototype directory (read-only)
make prototype-unlock   # Unlock prototype directory
```

## Frontend

**Use bun, not npm/node** for all frontend operations in `web/`.

## Architecture

### Socket Paths
- Global: `~/.valksor/kvelmo/global.sock` (one per machine)
- Worktree: `<project>/.kvelmo/worktree.sock` (one per project)
- Protocol: JSON-RPC 2.0

### Task Lifecycle (`internal/conductor/`)

**The workflow kvelmo orchestrates:**

```
[External Task Source]
        ↓
    LOAD (start)
        ↓
    PLAN (plan) → Agent writes specification
        ↓
    IMPLEMENT (implement) → Agent writes code
        ↓
    SIMPLIFY (simplify) → Optional cleanup pass
        ↓
    OPTIMIZE (optimize) → Quality improvements
        ↓
    REVIEW (review) → Human review checkpoint
        ↓
    SUBMIT (submit) → Create PR
        ↓
    FINISH (finish) → Cleanup after merge
```

States: `none`, `loaded`, `planning`, `planned`, `implementing`, `implemented`, `simplifying`, `optimizing`, `reviewing`, `submitted`, `waiting`, `paused`, `failed`

Each transition creates a git checkpoint. `undo`/`redo` navigate between checkpoints.

### Package Layout

kvelmo splits its Go packages into a **public API surface** (hoisted to the module root, importable by external Go programs via `kvelmo pipe` and similar adapters) and **private orchestration** (under `internal/`, compiler-enforced private). The old `pkg/` directory no longer exists.

**Public API (module root) — reusable adapters, config, and metadata:**

| Package | Purpose |
|---------|---------|
| `agent/` | AI agent interface and adapters (`claude`, `claudesdk`, `claudemcp`, `codex`, `anthropic`, `apiagent`, `openai`, `ollama`, `custom`, `replay`); sub-packages: `permission` (tool approval), `recorder` (session recording), `strategy` (reasoning strategies), `agenttest` (test helpers). **Three-way split for the claude binary:** `claude` = plain `claude --print` over stdin/stdout (works today; billing classification is `claude -p` so the 2026-06-15 credit-pool split applies; what `glm extends: claude` uses). `claudesdk` = WebSocket Agent SDK (`--sdk-url`); **broken on the official Anthropic CLI since version 2.1.121** (Anthropic restricted the flag; lift unknown), retained for proxy setups like Claude Code Router. `claudemcp` = interactive TUI under PTY + `--mcp-config`; **the default**, preserves Max-subscription billing after the 2026-06-15 Agent SDK credit split. See `docs/agents/claude-mcp.md`. |
| `settings/` | Configuration management + drift detection |
| `metrics/` | Observability (counters, latency) |
| `meta/` | Build metadata (version, commit, docs URL) |
| `paths/` | Centralized path resolution |

**Private (`internal/`) — orchestration machinery, not importable outside the module:**

| Package | Purpose |
|---------|---------|
| `socket/` | Unix domain socket servers (global + per-worktree) |
| `conductor/` | Task state machine and lifecycle transitions |
| `worker/` | Concurrent job execution pool |
| `provider/` | Task sources (github, gitlab, linear, wrike, jira, azuredevops, file) |
| `storage/` | Persistence for tasks, plans, reviews, chat |
| `git/` | Repository operations and checkpoint management |
| `browser/` | Playwright automation for interactive testing |
| `web/` | HTTP server + WebSocket proxy to sockets |
| `memory/` | Vector store for semantic context search |
| `security/` | Security scanning |
| `screenshot/` | Screenshot capture and storage |
| `activitylog/` | RPC activity logging and querying |
| `backup/` | Backup and restore of kvelmo state |
| `catalog/` | Task template library (built-in + custom) |
| `changelog/` | Release changelog generation |
| `ciwatch/` | CI pipeline status monitoring |
| `cli/` | CLI framework utilities and output helpers |
| `codegraph/` | Code symbol and relationship indexing (SQLite-backed) |
| `discovery/` | Project command scanning (Makefile, package.json, Taskfile) |
| `eventlog/` | Append-only JSONL event log for task lifecycle replay |
| `filter/` | Generic type-safe in-memory filtering with predicates |
| `findings/` | Unified finding model with gate rules and phase-aware quality profiles |
| `graph/` | Dependency graph scheduling for parallel sub-tasks within phases |
| `notify/` | Webhook notifications (Slack, generic) |
| `page/` | Pagination primitives |
| `policy/` | Workflow policy checking and validation |
| `provision/` | Smart worktree provisioning (config copy, dependency symlinks) |
| `quality/` | Code quality gate execution + failure classification |
| `ratelimit/` | Rate limiting utilities |
| `report/` | Compliance report generation |
| `respcache/` | Semantic response cache for agent prompt deduplication |
| `retry/` | Retry logic with exponential backoff |
| `riskeval/` | Risk scoring for task-level auto-approval decisions |
| `search/` | Hybrid fuzzy + exact text search with RRF scoring |
| `taskgroup/` | Cross-repo task group coordination with synchronized submit |
| `testutil/` | Test helpers and fixtures |
| `timeline/` | Activity timeline service over activitylog |
| `trace/` | Distributed tracing |
| `tui/` | Terminal UI (Bubbletea-based dashboard) |
| `update/` | Self-update: GitHub release checker, minisign-verified downloader, atomic installer |
| `varpool/` | Variable pool for inter-node context sharing during graph execution |
| `watchdog/` | Background process monitoring |

### Web Frontend (`web/`)

- React 19 + TypeScript 6 + Vite 8
- UI: Tailwind CSS 4 + DaisyUI 5
- Views: `GlobalView` (project picker) ↔ `ProjectView` (active project dashboard)

**Stores (Zustand):**
- `globalStore` - Projects, workers, agent status across all worktrees
- `projectStore` - Active worktree state, task lifecycle, file changes
- `chatStore` - Message history, streaming, subagent status
- `browserStore` - Playwright session state
- `screenshotStore` - Screenshot selection and attachments
- `themeStore` - Light/dark mode
- `layoutStore` - Panels, widgets, tabs (15 tab types)
- `viewModeStore` - View mode (grid, list, etc.)
- `debugStore` - Debug mode state and diagnostic helpers

## Key Patterns

### Error Handling
Go: Return errors, wrap with context (`fmt.Errorf("action: %w", err)`)

### Configuration
- Global config: `~/.valksor/kvelmo/kvelmo.yaml` (managed by `settings/`)
- CLI: `kvelmo config show|init|set`
- Environment: `KVELMO_HOME` (overrides `~/.valksor/kvelmo`), `GITHUB_TOKEN`, etc.

### Testing
- Table-driven tests using `testing.T`
- Benchmark tests in `internal/socket/bench_test.go`
- Frontend: Add `?demo` URL param for UI testing without backend
- **Never accept test failures.** If a test fails, fix it. No exceptions. Never rationalize failures as "pre-existing" or "not my problem."

### Quality Gate Rules

When running `make quality`, `make test`, or `make ci`:
- **Fix ALL errors and failures in the output, not just ones you introduced.** Pre-existing failures are your responsibility too.
- Do not skip, ignore, or dismiss errors you didn't cause. The codebase must be clean after your work.
- If `make quality` reports 10 lint errors and you caused 2, fix all 10.
- If `make test` has 3 failing tests and you wrote 1, fix all 3.
- Run the quality/test command again after fixing to confirm zero errors remain.

### Dead Code vs Unimplemented Code

Before deleting any logic flagged as "dead code," verify whether it is truly unused or simply not yet implemented. Reviewers and automated tools may mark code as dead when the actual issue is missing implementation, not unnecessary code. If the code is scaffolding for a planned feature, implement it — don't delete it.

## CLI Commands

Commands in `cmd/kvelmo/commands/`. Entry point: `serve` (global socket + web server, port 6337).

<!-- Surface parity note: these CLI commands are intentionally excluded from web chat -->
<!-- completion, pipe, tui, serve, shutdown, cleanup, rpc, prompt, upgrade, tutorial, autostart -->
<!-- start: complex input with file upload — handled by TaskPanel widget in web UI -->

**Workflow progression:**
- `start` - Load task and initialize worktree (accepts positional text arg; `--skip` for phase skipping; `--file`/`--symbol`/`--commit` for context attachment)
- `plan` - Have agent write specification
- `implement` - Have agent write code
- `simplify` - Optional code cleanup pass
- `optimize` - Quality improvements
- `review` - Enter human review mode
- `submit` - Create pull request (`--dry-run` to preview, `--section` for custom sections)
- `finish` - Cleanup after PR merge
- `quick` - Quick-fix mode: load, implement, submit in one step (accepts positional text arg; `--skip`; `--file`/`--symbol`/`--commit`)

**Workflow control:**
- `undo`/`redo` - Navigate checkpoints
- `status` - Show current state (`--all` for multi-project, `--blocked`/`--failed` to filter)
- `watch` - Stream progress
- `retry` - Re-run failed phases (phase commands accept `--wait` flag to block until completion)
- `stop`/`abort`/`reset` - Interrupt operations
- `abandon` - Abandon current task
- `delete` - Delete task permanently
- `update` - Refresh task from source
- `checklist` - Manage review checklist items (check, uncheck, list)
- `fork` - Fork task into parallel alternatives (create, list, compare, select)

**Governance & quality:**
- `approve` - Approve workflow transitions
- `audit` - Audit trail inspection
- `policy` - Check workflow policy compliance
- `quality` - Run code quality gates
- `ci` - CI pipeline status

**Task organization:**
- `tag` - Add/remove/list task tags
- `queue` - Task queue management (add, remove, list, reorder)
- `batch` - Run actions across all active projects
- `catalog` - Browse and use task templates
- `group` - Cross-repo task group management (create, add, list, status, submit, remove)

**Context & debugging:**
- `chat` - Interactive agent conversation
- `checkpoints` - List/manage git checkpoints
- `memory` - View/manage context store
- `logs` - View operation logs
- `prompt` - Shell prompt integration (PS1)
- `recap` - Summarize current task state for resuming work
- `explain` - Ask agent to explain last action
- `diagnose` - System diagnostics
- `show` - Display specification, plan, or review content
- `diff` - Show file changes from agent work
- `list` - List tasks (active, history, queue)
- `jobs` - View job queue and status
- `cache` - Response cache management (stats, clear)
- `codegraph` - Code symbol graph (stats, index, search, callers, deps)
- `eventlog` - View task lifecycle events (phase transitions, checkpoints)
- `tui` - Terminal UI dashboard (Bubbletea)
- `rpc` - Raw JSON-RPC calls to sockets

**Management:**
- `config` - Configuration (show, get, set, init, edit, diff, path, validate, check)
- `workers` - Worker pool (list, add, remove, stats)
- `projects` - Project registry
- `agent` - Agent status and health checks
- `strategy` - List available agent reasoning strategies
- `serve` - Start global socket + web server
- `shutdown` - Gracefully stop the server
- `cleanup` - Remove stale socket files
- `autostart` - Auto-start worktree sockets when needed
- `github`/`gitlab`/`linear`/`wrike`/`jira`/`azuredevops` - Provider commands (each has `login` subcommand)
- `config test-provider` - Test a provider connection (verify token and reachability)

**Data & reporting:**
- `export` - Export task history and metrics (JSON/CSV)
- `report` - Generate compliance reports
- `changelog` - Generate changelog between two git refs (`--full` for descriptions)
- `stats` - Show real-time metrics
- `activity` - View RPC activity log

**Infrastructure:**
- `backup`/`restore` - State backup and restore
- `security` - Security scanning (secrets, dependencies)
- `notify` - Webhook notification testing
- `hooks` - List configured workflow hooks
- `recordings` - View agent session recordings

**Utilities:**
- `browse` - Open URLs in browser
- `browser` - Playwright browser automation
- `screenshots` - Capture and manage screenshots
- `files` - List/search project files
- `git` - Git operations
- `completion` - Shell completion
- `tutorial` - Interactive walkthrough
- `pipe` - One-shot agent prompt (stdin/stdout, no server required)
- `remote` - Remote PR operations (approve, merge)
- `discover` - List available project commands (Makefile, package.json, Taskfile)
- `upgrade` - Self-update binary from GitHub releases (`--nightly`, `--version`, `--check`)

## Code Style

- **Imports**: stdlib → third-party → local (alphabetical within groups)
- **Naming**: PascalCase exported, camelCase unexported
- **Errors**: `fmt.Errorf("prefix: %w", err)`
- **Logging**: `log/slog`

### Import Discipline

Import packages directly. No type aliases, no wrapper functions, no re-exports.

```go
// ✅ GOOD - Direct import
import "github.com/gorilla/websocket"
conn, _ := websocket.Upgrade(...)

// ❌ BAD - Type alias or wrapper
type Conn = websocket.Conn  // Don't do this
func NewConn(...) *Conn { return websocket.Upgrade(...) }  // Don't do this
```

### Modern Go (1.23+)

- Use `slices.Contains()`, `maps.Clone()` instead of manual loops
- Always pass `context.Context` for cancelable operations

## Linting Guidelines

`//nolint` is a last resort. Always specify linter name and include justification.

**Acceptable**:
- `//nolint:unparam // Required by interface`
- `//nolint:errcheck // String builder WriteString won't fail`

**Never acceptable**:
- `//nolint:errcheck` without justification
- `//nolint:gosec` (fix the security issue)
- `//nolint:all`

## File Organization

Keep Go files under 500 lines. Split by feature or responsibility:

```go
// Split handlers.go (800 lines) into:
handlers_plan.go      // Planning handlers
handlers_implement.go // Implementation handlers
handlers_review.go    // Review handlers
```

### Commit Style

Plain imperative sentences. No conventional commits prefix (`feat:`, `fix:`, `chore:`, etc.).

```text
✅ Add project selector to GlobalView
✅ Fix socket reconnect race condition
✅ Improve TaskWidget keyboard navigation

❌ feat(web): add project selector
❌ fix(socket): reconnect race condition
```
