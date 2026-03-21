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
# Build (includes web frontend)
make build              # Full build: web + Go binary → ./build/kvelmo
make build-go           # Go-only build (faster, skip web)

# Run
make run                # Build and run (sockets + web UI)
make run-dev            # Run without rebuilding (uses existing binary)

# Tests
make test               # Run all tests
make test-v             # Verbose test output
make test-cover         # Coverage report → coverage.html
make test-race          # Tests with race detector
go test ./pkg/socket/...  # Run tests for specific package

# Quality
make quality            # fmt + vet + lint + alias check
make ci                 # quality + test + build

# Frontend
make web-dev            # Vite dev server with hot reload (port 5173)
make web-build          # Production build → web/dist/

# Desktop (Tauri)
make desktop-dev        # Tauri dev mode with hot reload
make desktop-build      # Production desktop app build
make desktop-sidecar    # Prepare sidecar binary (current platform)
make desktop-clean      # Clean desktop build artifacts

# Frontend testing
make web-test           # Frontend unit tests
make web-test-coverage  # Frontend test coverage report
make web-e2e            # Web end-to-end tests (demo mode)

# Type generation
make types              # Generate TypeScript types from Go structs (tygo)

# E2E tests
make test-e2e           # Full E2E tests (requires GITHUB_TOKEN + E2E_GITHUB_REPO)
make test-e2e-cli       # CLI E2E cycle test
make test-e2e-provider  # E2E provider tests only
make test-e2e-gitlab    # E2E GitLab tests only
make test-e2e-workflow  # E2E workflow tests only

# Additional targets
make dev                # quality + test + run (full dev workflow)
make install            # Install binary to $GOPATH/bin
make man-pages          # Generate man pages for all commands
make release            # Build release binaries for all platforms
make clean              # Remove all build artifacts
make deps               # Download Go module dependencies
make tidy               # Clean and tidy Go module dependencies
```

## Frontend

**Use bun, not npm/node** for all frontend operations in `web/`.

## Architecture

### Socket Paths
- Global: `~/.valksor/kvelmo/global.sock` (one per machine)
- Worktree: `<project>/.kvelmo/worktree.sock` (one per project)
- Protocol: JSON-RPC 2.0

### Task Lifecycle (`pkg/conductor/`)

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

### Package Index (`pkg/`)

| Package | Purpose |
|---------|---------|
| `socket/` | Unix domain socket servers (global + per-worktree) |
| `conductor/` | Task state machine and lifecycle transitions |
| `agent/` | AI agent interface (claude, codex, custom); includes permission, recorder, and strategy sub-packages |
| `worker/` | Concurrent job execution pool |
| `provider/` | Task sources (github, gitlab, linear, wrike, jira, azuredevops, file) |
| `storage/` | Persistence for tasks, plans, reviews, chat |
| `git/` | Repository operations and checkpoint management |
| `browser/` | Playwright automation for interactive testing |
| `web/` | HTTP server + WebSocket proxy to sockets |
| `memory/` | Vector store for semantic context search |
| `settings/` | Configuration management |
| `paths/` | Centralized path resolution |
| `metrics/` | Observability (counters, latency) |
| `security/` | Security scanning |
| `screenshot/` | Screenshot capture and storage |
| `access/` | Socket access token management |
| `activitylog/` | RPC activity logging and querying |
| `backup/` | Backup and restore of kvelmo state |
| `catalog/` | Task template library (built-in + custom) |
| `changelog/` | Release changelog generation |
| `changeset/` | Internal changeset tracking |
| `ciwatch/` | CI pipeline status monitoring |
| `cli/` | CLI framework utilities and output helpers |
| `configcheck/` | Configuration drift detection |
| `discovery/` | Project command scanning (Makefile, package.json, Taskfile) |
| `filter/` | Filtering utilities |
| `findings/` | Unified finding model with gate rules and phase-aware quality profiles |
| `graph/` | Dependency graph scheduling for parallel sub-tasks within phases |
| `log/` | Structured logging (slog wrappers) |
| `meta/` | Build metadata (version, commit, docs URL) |
| `notify/` | Webhook notifications (Slack, generic) |
| `onboarding/` | User onboarding workflows |
| `page/` | Pagination utilities |
| `policy/` | Workflow policy checking and validation |
| `quality/` | Code quality gate execution |
| `ratelimit/` | Rate limiting utilities |
| `report/` | Compliance report generation |
| `retry/` | Retry logic with exponential backoff |
| `search/` | File and content search utilities |
| `testutil/` | Test helpers and fixtures |
| `timeline/` | Event timeline utilities |
| `trace/` | Distributed tracing |
| `tui/` | Terminal UI (Bubbletea-based dashboard) |
| `varpool/` | Variable pool for inter-node context sharing during graph execution |
| `watchdog/` | Background process monitoring |

### Web Frontend (`web/`)

- React 19 + TypeScript + Vite 8
- UI: Tailwind CSS 4 + DaisyUI 5
- Views: `GlobalView` (project picker) ↔ `ProjectView` (active project dashboard)

**Stores (Zustand):**
- `globalStore` - Projects, workers, agent status across all worktrees
- `projectStore` - Active worktree state, task lifecycle, file changes
- `chatStore` - Message history, streaming, subagent status
- `browserStore` - Playwright session state
- `screenshotStore` - Screenshot selection and attachments
- `themeStore` - Light/dark mode
- `layoutStore` - Panels, widgets, tabs (13 tab types)
- `debugStore` - Debug mode state and diagnostic helpers

## Key Patterns

### Error Handling
Go: Return errors, wrap with context (`fmt.Errorf("action: %w", err)`)

### Configuration
- Global config: `~/.valksor/kvelmo/kvelmo.yaml` (managed by `pkg/settings/`)
- CLI: `kvelmo config show|init|set`
- Environment: `KVELMO_SOCKET_DIR`, `GITHUB_TOKEN`, etc.

### Testing
- Table-driven tests using `testing.T`
- Benchmark tests in `pkg/socket/bench_test.go`
- Frontend: Add `?demo` URL param for UI testing without backend
- **Never accept test failures.** If a test fails, fix it. No exceptions. Never rationalize failures as "pre-existing" or "not my problem."

### Quality Gate Rules

When running `make quality`, `make test`, or `make ci`:
- **Fix ALL errors and failures in the output, not just ones you introduced.** Pre-existing failures are your responsibility too.
- Do not skip, ignore, or dismiss errors you didn't cause. The codebase must be clean after your work.
- If `make quality` reports 10 lint errors and you caused 2, fix all 10.
- If `make test` has 3 failing tests and you wrote 1, fix all 3.
- Run the quality/test command again after fixing to confirm zero errors remain.

## CLI Commands

Commands in `cmd/kvelmo/commands/`. Entry point: `serve` (global socket + web server, port 6337).

**Workflow progression:**
- `start` - Load task and initialize worktree
- `plan` - Have agent write specification
- `implement` - Have agent write code
- `simplify` - Optional code cleanup pass
- `optimize` - Quality improvements
- `review` - Enter human review mode
- `submit` - Create pull request
- `finish` - Cleanup after PR merge
- `quick` - Quick-fix mode: load, implement, submit in one step

**Workflow control:**
- `undo`/`redo` - Navigate checkpoints
- `status` - Show current state
- `watch` - Stream progress
- `retry` - Re-run failed phases (phase commands accept `--wait` flag to block until completion)
- `stop`/`abort`/`reset` - Interrupt operations
- `abandon` - Abandon current task
- `delete` - Delete task permanently
- `update` - Refresh task from source
- `autostart` - Auto-start task phases on load

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

**Context & debugging:**
- `chat` - Interactive agent conversation
- `checkpoints` - List/manage git checkpoints
- `memory` - View/manage context store
- `logs` - View operation logs
- `prompt` - Shell prompt integration (PS1)
- `explain` - Ask agent to explain last action
- `diagnose` - System diagnostics
- `show` - Display specification, plan, or review content
- `diff` - Show file changes from agent work
- `list` - List tasks (active, history, queue)
- `jobs` - View job queue and status
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
- `login` - Authenticate with task providers (subcommands: `github login`, `gitlab login`, etc.)

**Data & reporting:**
- `export` - Export task history and metrics (JSON/CSV)
- `report` - Generate compliance reports
- `stats` - Show real-time metrics
- `activity` - View RPC activity log

**Infrastructure:**
- `backup`/`restore` - State backup and restore
- `access` - Socket access token management
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
