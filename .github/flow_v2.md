# kvelmo: Consolidated Task Lifecycle Orchestrator

## Context

**Problem:** Three codebases exist with overlapping goals:
1. **mono-valksor** — Over-fragmented (38 repos) but good package-level patterns
2. **go-mehrhof** — Working but complex, CLI/Web have separate paths to conductor
3. **companion** — Clean WebSocket relay pattern for agent communication

**Goal:** Create a new consolidated tool that:
- Takes go-mehrhof's orchestration core
- Uses companion's socket-based communication pattern
- Applies mono-valksor's package-level granularity (not repo-level)
- Delivers an "app feel" developer tool (not a form-based website)

---

## Architecture: Socket-First Design

### Core Principle
**One conductor, one entrypoint.** The socket IS the conductor. Everything else is a client.

```
┌─────────────────────────────────────────────────────────────────────┐
│                         GLOBAL SOCKET                               │
│  (small, lightweight - settings, registry, status queries only)     │
│  • Does NOT manage workers or queue                                 │
│  • Can query project sockets for status                             │
└─────────────────────────────────────────────────────────────────────┘
         │ knows about
         ▼
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│ PROJECT SOCKET A│  │ PROJECT SOCKET B│  │ PROJECT SOCKET C│
│ (conductor)     │  │ (conductor)     │  │ (conductor)     │
│ • State machine │  │ • State machine │  │ • State machine │
│ • Git ops       │  │ • Git ops       │  │ • Git ops       │
│ • Task logic    │  │ • Task logic    │  │ • Task logic    │
│ • Submits jobs  │  │ • Submits jobs  │  │ • Submits jobs  │
└────────┬────────┘  └────────┬────────┘  └────────┬────────┘
         │                    │                    │
         └────────────────────┼────────────────────┘
                              │ submit jobs
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    GLOBAL WORKER QUEUE                              │
│  • Jobs from ALL projects                                           │
│  • Workers pick jobs, route output back to source project           │
│  • Project-aware (knows who called, what needs to be done)          │
└─────────────────────────────────────────────────────────────────────┘
                              │
       ┌──────────────────────┼──────────────────────┐
       │                      │                      │
    CLI cmd              Web serve              Tauri app
   (client)              (client)               (client)
```

### Socket Topology

```
~/.kvelmo/
├── global.sock                    # Management socket (settings, registry, status)
└── worktrees/
    ├── {worktree-hash-1}.sock     # Worktree A conductor
    └── {worktree-hash-2}.sock     # Worktree B conductor
```

**Two socket types:**

| Socket       | Purpose                                                                   | Lifecycle                                                        |
|--------------|---------------------------------------------------------------------------|------------------------------------------------------------------|
| **Global**   | Management: settings, project registry, **shared worker pool**, job queue | Starts on first `kvelmo` command or app/web launch. Stays alive. |
| **Worktree** | Task state machine, git ops for ONE worktree. Submits jobs to global.     | Starts on `init` or `start`. Stopped via `disconnect`.           |

**Critical: Worker pool is GLOBAL**
- LLM agents consume CPU/RAM — can't have unlimited
- **Max 5–6 workers total** across all projects
- Jobs from ANY project go into ONE shared queue
- Prevents resource exhaustion when working on multiple projects

**Socket-per-worktree logic:**
- Non-worktree project = 1 worktree socket (root is the "worktree")
- Git worktree = 1 socket per worktree
- Each worktree is independent for state but shares workers via global

**Startup sequences:**

```
CLI: kvelmo init
  └─► Start global socket (if not running) ← owns worker pool
  └─► Start worktree socket for current directory
  └─► Register worktree with global

CLI: kvelmo start --from github:...
  └─► Connect to global socket (start if needed)
  └─► Start worktree socket (if not running)
  └─► Begin task workflow

CLI: kvelmo plan
  └─► Worktree socket creates job
  └─► Job submitted to GLOBAL queue
  └─► Global assigns to available worker
  └─► Output streams: Worker → Global → Worktree → Client

Web/App launch (global mode):
  └─► Start global socket (if not running)
  └─► Show project picker (recent + picker)
  └─► List all registered worktrees with status
  └─► User selects worktree → connect to its socket
  └─► User can switch projects seamlessly

CLI: kvelmo disconnect
  └─► Stop worktree socket
  └─► Unregister from global
  └─► (Global keeps running, workers stay available)
```

**Global socket responsibilities:**
- Settings (global preferences)
- Worktree registry (list all registered projects/worktrees)
- **Worker pool** (5–6 max workers, shared)
- **Job queue** (jobs from all worktrees)
- Status aggregation (see all worktree states at once)

**Worktree socket responsibilities:**
- Task state machine (Task: None → Loaded → Planning → ...)
- Git operations (branches, checkpoints, undo/redo)
- Submit jobs to global queue
- Receive job results from global
- Does NOT own workers

### Protocol

| Type      | Protocol           | Use Case                                     |
|-----------|--------------------|----------------------------------------------|
| Commands  | JSON-RPC 2.0       | `start`, `plan`, `implement`, `submit`, etc. |
| Streaming | NDJSON passthrough | Agent output streams directly, no parsing    |

---

## Package Structure

```
github.com/valksor/kvelmo/
├── cmd/
│   └── kvelmo/
│       └── main.go              # Single binary entry point
│
├── internal/
│   ├── conductor/               # State machine, workflow orchestration
│   │   ├── conductor.go         # Core orchestrator
│   │   ├── state.go             # State definitions
│   │   └── transitions.go       # State transitions + guards
│   │
│   ├── socket/                  # Socket server + protocol
│   │   ├── server.go            # TCP socket server
│   │   ├── global.go            # Global socket (worker pool, registry)
│   │   ├── worktree.go          # Worktree socket (state machine, git)
│   │   ├── protocol.go          # JSON-RPC + NDJSON handling
│   │   └── client.go            # Client connection (for CLI/Web)
│   │
│   ├── worker/                  # Worker pool + job queue (owned by global)
│   │   ├── pool.go              # Worker pool management (max 5-6)
│   │   ├── worker.go            # Worker abstraction
│   │   ├── job.go               # Job queue + routing to worktrees
│   │   └── roles.go             # Planner, Implementer, Reviewer, Optimizer
│   │
│   ├── agent/                   # Agent connections (low-level)
│   │   ├── agent.go             # Interface + registry
│   │   ├── websocket.go         # WebSocket-first (primary)
│   │   ├── binary.go            # Binary fallback
│   │   ├── claude.go            # Claude implementation
│   │   └── codex.go             # Codex implementation
│   │
│   ├── provider/                # Task source interface
│   │   └── provider.go          # Interface definition
│   ├── file/                    # File provider
│   ├── github/                  # GitHub provider
│   ├── gitlab/                  # GitLab provider
│   └── wrike/                   # Wrike provider
│   │
│   ├── git/                     # Git operations
│   │   ├── branch.go            # Branch management
│   │   ├── checkpoint.go        # Checkpoints + undo/redo
│   │   └── worktree.go          # Worktree support
│   │
│   ├── security/                # Security scanning
│   │   └── scanner.go           # SAST, secrets, vulns
│   │
│   ├── quality/                 # Code quality
│   │   └── linter.go            # Linting integration
│   │
│   └── browser/                 # Browser automation
│       └── playwright.go        # Playwright integration
│
├── web/                         # React dashboard
│   ├── src/
│   │   ├── components/
│   │   │   ├── ui/              # shadcn/ui primitives (button, card, etc.)
│   │   │   ├── layout/
│   │   │   │   ├── AppShell.tsx       # Root layout with panels
│   │   │   │   ├── Sidebar.tsx        # Collapsible sidebar
│   │   │   │   └── CommandPalette.tsx # Cmd+K menu
│   │   │   ├── global/
│   │   │   │   ├── ProjectPicker.tsx  # Global mode project list
│   │   │   │   ├── WorkerPoolWidget.tsx
│   │   │   │   └── JobQueueWidget.tsx
│   │   │   ├── project/
│   │   │   │   ├── WorktreeList.tsx
│   │   │   │   ├── TaskWidget.tsx
│   │   │   │   ├── AgentOutput.tsx    # Streaming terminal output
│   │   │   │   ├── GitStatus.tsx
│   │   │   │   └── Checkpoints.tsx
│   │   │   └── shared/
│   │   │       └── Terminal.tsx       # Monospace output component
│   │   ├── hooks/
│   │   │   ├── useGlobalSocket.ts     # Connect to global socket
│   │   │   ├── useWorktreeSocket.ts   # Connect to worktree socket
│   │   │   └── useStream.ts           # NDJSON streaming
│   │   ├── stores/
│   │   │   ├── globalStore.ts         # Zustand: projects, workers
│   │   │   └── projectStore.ts        # Zustand: task, git, checkpoints
│   │   └── lib/
│   │       └── socket.ts              # WebSocket client wrapper
│   ├── components.json           # shadcn/ui config
│   └── package.json
│
├── desktop/                     # Tauri wrapper
│   └── src-tauri/
│       └── main.rs              # Just wraps web server
│
└── docs/
```

---

## Data Flow: Task Lifecycle

### 1. Starting a Task

```
User: kvelmo start --from github:valksor/repo#123

CLI                          Socket Server                    GitHub
 │                                │                              │
 ├─[connect to project socket]───►│                              │
 │                                │                              │
 ├─[JSON-RPC: start]─────────────►│                              │
 │                                ├─[fetch issue]───────────────►│
 │                                │◄─[issue data]─────────────────┤
 │                                │                              │
 │                                ├─[create branch]              │
 │                                ├─[set state: Planning]        │
 │◄─[OK: task started]────────────┤                              │
```

### 2. Planning (Agent Interaction via Global Pool)

```
CLI            Worktree Socket        Global Socket           Worker (Claude)
 │                   │                      │                       │
 ├─[plan]───────────►│                      │                       │
 │                   ├─[submit job]────────►│                       │
 │                   │                      ├─[assign to worker]───►│
 │                   │                      │                       │
 │                   │                      │◄─[NDJSON stream]──────┤
 │                   │◄─[route output]──────┤                       │
 │◄─[NDJSON]─────────┤                      │                       │
 │◄─[NDJSON]─────────┤                      │                       │
 │                   │                      │                       │
 │                   │                      │◄─[complete]───────────┤
 │                   │◄─[job done]──────────┤                       │
 │                   ├─[checkpoint git]     │                       │
 │                   ├─[set state: Planned] │                       │
 │◄─[OK]─────────────┤                      │                       │
```

**Key insight:** Worktree socket doesn't talk to agents directly. It submits jobs to global, global assigns to workers, output routes back.

### 3. Web UI Connection

```
Browser                      Web Server                    Socket Server
 │                              │                               │
 ├─[HTTP GET /]────────────────►│                               │
 │◄─[Dashboard HTML]────────────┤                               │
 │                              │                               │
 ├─[WebSocket connect]─────────►│                               │
 │                              ├─[connect to project socket]──►│
 │                              │                               │
 │                              │◄─[state snapshot]─────────────┤
 │◄─[state via WS]──────────────┤                               │
 │                              │                               │
 │  (agent streaming)           │                               │
 │◄─[NDJSON via WS]◄────────────┤◄─[NDJSON passthrough]─────────┤
```

---

## State Machine

```
                    ┌────────────────────────────────────────────┐
                    │                                            │
                    ▼                                            │
        ┌───────────────┐  start  ┌──────────────┐  plan  ┌──────────────┐
 ──────►│ Task: None    │────────►│ Task: Loaded │───────►│Task: Planning│
        └───────────────┘         └──────────────┘        └──────┬───────┘
                 ▲                                               │
                 │                                               │ complete
                 │                                               ▼
        ┌────────────────┐ submit ┌───────────────┐ impl  ┌──────────────┐
        │Task: Submitted │◄───────│Task: Reviewing│◄──────│Task: Planned │
        └────────────────┘        └───────────────┘       └──────┬───────┘
                                         │                       │
                                         │ reject                │ implement
                                         ▼                       ▼
                                  ┌──────────────┐        ┌─────────────────┐
                                  │Task: Planning│◄───────│Task: Implemented│
                                  └──────────────┘  revise└─────────────────┘
```

### States (descriptive, not ambiguous)
- **Task: None** — No active task (not "Idle" — that's ambiguous)
- **Task: Loaded** — Task fetched from provider, branch created
- **Task: Planning** — Agent generating specification (in progress)
- **Task: Planned** — Specification complete, ready for implementation
- **Task: Implementing** — Agent executing specification (in progress)
- **Task: Implemented** — Implementation complete, ready for review
- **Task: Reviewing** — Human review plus security scan (in progress)
- **Task: Submitted** — Task submitted to provider (PR created, etc.)

### Key Transitions
- `start` → Creates branch, loads task from provider
- `plan` → Runs agent for planning
- `implement` → Runs agent for implementation
- `optimize` → Optional agent pass for improvements
- `review` → Human approval gate
- `submit` → Push to provider (PR, issue update, etc.)
- `undo/redo` → Navigate git checkpoints

---

## Agent Communication

### Worker Pool Architecture (Augment-inspired)

**Problem with 1:1 prompt passing:** Conductor blocks waiting for agent response. No parallelism. Linear execution.

**Problem with per-project workers:** Multiple projects × multiple workers = resource explosion. 5 projects × 3 workers = 15 agents eating CPU/RAM.

**Solution:** Global worker pool with shared job queue. All worktrees submit to one queue. Max 5–6 workers total.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         GLOBAL SOCKET                                       │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │                      Shared Job Queue                                  │ │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐                       │ │
│  │  │Project A│ │Project B│ │Project A│ │Project C│  ← Jobs from ANY      │ │
│  │  │ Plan    │ │ Impl    │ │ Impl    │ │ Plan    │    worktree           │ │
│  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘                       │ │
│  └───────────────────────────┬────────────────────────────────────────────┘ │
│                              │                                              │
│           ┌──────────────────┼──────────────────┐                           │
│           ▼                  ▼                  ▼                           │
│     ┌──────────┐       ┌──────────┐       ┌──────────┐                      │
│     │ Worker 1 │       │ Worker 2 │       │ Worker 3 │   MAX 5-6 TOTAL      │
│     │ (Opus)   │       │ (Sonnet) │       │ (Sonnet) │                      │
│     │ Proj A   │       │ Proj B   │       │ Available│                      │
│     └──────────┘       └──────────┘       └──────────┘                      │
│          │                  │                                               │
└──────────┼──────────────────┼───────────────────────────────────────────────┘
           │                  │
           ▼                  ▼
      Claude WS          Claude WS
           │                  │
           ▼                  ▼
    ┌─────────────┐    ┌─────────────┐
    │ Worktree A  │    │ Worktree B  │  ← Results routed back to source
    │   Socket    │    │   Socket    │
    └─────────────┘    └─────────────┘
```

### Worker Types (personas)

| Worker          | Model  | Purpose                 | Tools                      |
|-----------------|--------|-------------------------|----------------------------|
| **Planner**     | Opus   | Generate specifications | Read, search, analyze      |
| **Implementer** | Sonnet | Write code, execute     | Read, write, git, terminal |
| **Reviewer**    | Sonnet | Review, suggest fixes   | Read, analyze, comment     |
| **Optimizer**   | Sonnet | Refactor, improve       | Read, write, refactor      |

### Job Queue Flow (Global)

```
1. User in Project A: kvelmo plan
2. Worktree socket A creates job: {worktree: "A", type: "plan", model: "opus"}
3. Job submitted to GLOBAL socket's queue
4. Global assigns job to available worker
5. Worker streams output → Global → Worktree A → Client
6. Worker completes → Global marks job "done"
7. Global notifies Worktree A of completion
8. Meanwhile: Project B can submit jobs to same queue
```

**Job routing:**
- Jobs tagged with source worktree ID
- Global routes output back to the correct worktree
- Worktree doesn't know/care about other projects' jobs

### Parallelism Examples

**Sequential (old way):**
```
[Plan component A] → wait → [Implement A] → wait → [Plan B] → wait → ...
```

**Parallel (worker pool):**
```
Conductor splits task: "Plan each component separately"
  └─► Job 1: Plan component A → Worker 1 picks
  └─► Job 2: Plan component B → Worker 2 picks
  └─► Job 3: Plan component C → Worker 3 picks

All three run simultaneously!
```

**Task splitting strategy:**
- By file/component: Each major file gets an own job
- By concern: UI vs. backend vs. tests
- By phase: Multiple planning alternatives, pick the best

### WebSocket-First Architecture

```go
// internal/worker/worker.go
type Worker struct {
    ID       string
    Model    string        // "opus", "sonnet", etc.
    Role     WorkerRole    // Planner, Implementer, Reviewer, Optimizer
    Status   WorkerStatus  // Available, Working, Disconnected
    conn     *websocket.Conn
}

type Job struct {
    ID        string
    Type      JobType       // Plan, Implement, Review, Optimize
    Prompt    string
    Model     string        // Required model
    Status    JobStatus     // Queued, InProgress, Done, Failed
    WorkerID  string        // Which worker picked it
    Output    chan Event    // Streaming output
}

// internal/worker/pool.go
type Pool struct {
    workers  []*Worker
    queue    chan *Job
    results  map[string]*Job
}

func (p *Pool) Submit(job *Job) error  // Add a job to the queue
func (p *Pool) Stream(jobID string) <-chan Event  // Get job output stream
```

### WebSocket Protocol (based on companion reverse-engineering)

**We act as WebSocket SERVER, Claude CLI connects to us.**

```bash
# Worker launches Claude with --sdk-url pointing to our server
claude --sdk-url ws://localhost:8765 --print \
  --output-format stream-json --input-format stream-json
```

**Message Types We Receive (CLI → Server):**

| Type              | Purpose                     | Key Fields                        |
|-------------------|-----------------------------|-----------------------------------|
| `system/init`     | Session start, capabilities | `session_id`, `tools[]`           |
| `stream_event`    | Token-by-token streaming    | `content`, `delta`                |
| `assistant`       | Full LLM response           | `message.role`, `message.content` |
| `control_request` | Permission request          | `id`, `tool`, `input`             |
| `result`          | Task complete               | `success`                         |
| `keep_alive`      | Heartbeat (10s)             | —                                 |

**Message Types We Send (Server → CLI):**

| Type               | Purpose                 | Key Fields                       |
|--------------------|-------------------------|----------------------------------|
| `user`             | Send prompt             | `message.content`, `session_id`  |
| `control_response` | Approve/deny permission | `control_request_id`, `approved` |

**Protocol: NDJSON over WebSocket** — one JSON object per line.

**Worker Implementation:**

```go
// internal/worker/websocket.go
type WebSocketWorker struct {
    port      int
    server    *http.Server
    conn      *websocket.Conn  // Claude's connection
    sessionID string
    messages  chan Message
}

func (w *WebSocketWorker) Start() error {
    // 1. Start WebSocket server
    w.server = startWSServer(w.port, w.handleConnection)

    // 2. Launch Claude CLI
    cmd := exec.Command("claude",
        "--sdk-url", fmt.Sprintf("ws://localhost:%d", w.port),
        "--print",
        "--output-format", "stream-json",
        "--input-format", "stream-json",
    )
    cmd.Start()

    // 3. Wait for connection + system/init
    initMsg := <-w.messages
    w.sessionID = initMsg.SessionID

    return nil  // Worker ready
}

func (w *WebSocketWorker) SendPrompt(prompt string) error {
    return w.send(Message{
        Type: "user",
        Message: UserMessage{Role: "user", Content: prompt},
        SessionID: w.sessionID,
    })
}

func (w *WebSocketWorker) HandlePermission(req ControlRequest) error {
    // Route to conductor for decision
    approved := w.conductor.EvaluatePermission(req)
    return w.send(Message{
        Type: "control_response",
        ControlRequestID: req.ID,
        Approved: approved,
    })
}
```

### Fallback Strategy

```
Claude supports --sdk-url? ──► WebSocket mode (structured messages)
        │
        No (older Claude version)
        ▼
Spawn as subprocess ──► Binary mode (parse stdout NDJSON)
        │
        Both fail
        ▼
Error: No agent available
```

**Binary fallback** uses the same NDJSON format, just over stdin/stdout instead of WebSocket. Message parsing is identical.

---

## Web UI: Dashboard Design

### Tech Stack

| Layer          | Technology             | Why                                                      |
|----------------|------------------------|----------------------------------------------------------|
| **Framework**  | React 19               | Stable, already used in go-mehrhof                       |
| **Styling**    | TailwindCSS 4          | Utility-first, fast iteration                            |
| **Components** | shadcn/ui              | Copy-paste primitives, full control, Radix accessibility |
| **State**      | Zustand                | Simple, already used in go-mehrhof                       |
| **Data**       | TanStack Query         | Caching, background refresh                              |
| **Layout**     | react-resizable-panels | VS Code-like resizable splits                            |
| **Build**      | Vite                   | Fast dev server, good HMR                                |

### Design Elements

**Layout Patterns:**
- Resizable panels (like VS Code) — sidebar, main, output
- Command palette (Cmd+K) for quick actions
- Collapsible drawers for secondary info
- Tabs plus split views for multiple worktrees

**Visual Style:**
- Dark mode first (developer tool aesthetic)
- Monospace for code/output, sans-serif for UI
- Minimal chrome, content-focused
- Subtle status colors (not heavy badges)
- Terminal-inspired output panels

**Interactions:**
- Keyboard-first (arrow keys, vim bindings optional)
- Context menus on right-click
- Drag-drop for file/project selection
- Toast notifications for background completion

### Philosophy
- **App feel, not website feel**
- Widgets, drawers, unified view
- See all active states at once
- Developer tool aesthetic
- **Global mode by default** — project picker on launch

### Global Mode (Default View)

```
┌─────────────────────────────────────────────────────────────────┐
│  kvelmo                                             [Settings]  │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │  Recent Projects                              [+ Open]   │   │
│  │  ──────────────────────────────────────────────────────  │   │
│  │  ● ~/workspace/project-a     Task: Implementing  ▶       │   │
│  │  ○ ~/workspace/project-b     Task: None                  │   │
│  │  ○ ~/workspace/mono/feat-x   Task: Planned      ▶        │   │
│  │  ○ ~/workspace/mono/fix-y    Task: Reviewing             │   │
│  │  ──────────────────────────────────────────────────────  │   │
│  │  [Browse...] or drag folder here                         │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                 │
│  ┌─────────────────────────┐  ┌─────────────────────────────┐   │
│  │  Worker Pool (Global)   │  │  Job Queue                  │   │
│  │  ────────────────────   │  │  ────────────────────────   │   │
│  │  ● Worker 1: Proj A     │  │  1. project-a: Implement    │   │
│  │  ● Worker 2: Proj B     │  │  2. feat-x: Review          │   │
│  │  ○ Worker 3: Available  │  │  3. project-a: Optimize     │   │
│  │  ○ Worker 4: Available  │  │  ────────────────────────   │   │
│  │  ────────────────────   │  │  3 jobs queued              │   │
│  │  2/5 active             │  └─────────────────────────────┘   │
│  └─────────────────────────┘                                    │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Project View (After Selection)

```
┌─────────────────────────────────────────────────────────────────┐
│  kvelmo                 [← All Projects] [Project ▾] [Settings] │
├───────────┬─────────────────────────────────────────────────────┤
│           │                                                     │
│ Worktrees │  ┌─────────────────────┐  ┌─────────────────────┐   │
│           │  │   Task: #123        │  │   Agent Output      │   │
│ ● main    │  │   State: Planning   │  │                     │   │
│ ○ feat-x  │  │   ────────────────  │  │   > Analyzing...    │   │
│ ○ fix-y   │  │   [Plan] [Impl]     │  │   > Found 3 files   │   │
│           │  │   [Review] [Submit] │  │   > Generating...   │   │
│           │  └─────────────────────┘  └─────────────────────┘   │
│───────────│                                                     │
│           │  ┌─────────────────────┐  ┌─────────────────────┐   │
│ Workers   │  │   Git Status        │  │   Checkpoints       │   │
│           │  │                     │  │                     │   │
│ ● 1 busy  │  │   branch: feat-123  │  │   ◉ Plan v2         │   │
│ ○ 4 avail │  │   +3 -1 modified    │  │   ○ Plan v1         │   │
│           │  │   ✓ clean           │  │   ○ Started         │   │
│           │  └─────────────────────┘  └─────────────────────┘   │
│           │                                                     │
└───────────┴─────────────────────────────────────────────────────┘
```

### Widget Architecture

**Level 1: Global View**
| Widget | Purpose | Conductor Handler |
|--------|---------|-------------------|
| `ProjectList` | Recent projects, add new | `global.ListProjects()`, `global.RegisterProject()` |
| `GlobalStats` | Total costs, jobs today, uptime | `global.GetStats()` |
| `GlobalQueue` | All pending jobs across projects | `global.ListQueue()` |
| `WorkerPool` | Worker status (all workers) | `global.ListWorkers()` |

**Level 2: Project View**
| Widget | Purpose | Conductor Handler |
|--------|---------|-------------------|
| `TaskList` | Active tasks in a project | `project.ListTasks()` |
| `ProjectStats` | Costs for this project, job count | `project.GetStats()` |
| `ProjectQueue` | Jobs from this project only | `project.ListQueue()` |
| `WorktreeList` | Worktrees in this project | `project.ListWorktrees()` |
| `QuickActions` | Start task, import, settings | (multiple handlers) |

**Level 3: Task View**
| Widget | Purpose | Conductor Handler |
|--------|---------|-------------------|
| `TaskInfo` | Task title, state, source, metadata | `task.GetInfo()` |
| `AgentOutput` | Streaming terminal output | `task.StreamOutput()` |
| `AgentChat` | Send messages, answer questions | `task.SendMessage()`, `task.AnswerQuestion()` |
| `TaskActions` | Plan, Implement, Stop, Undo/Redo | `task.Plan()`, `task.Implement()`, `task.Stop()` |
| `Specification` | Current spec (collapsible) | `task.GetSpec()` |
| `FileChanges` | Files modified, diff preview | `task.ListChanges()` |
| `Checkpoints` | Undo/redo navigation | `task.ListCheckpoints()` |
| `Mentions` | Reference files, other tasks | `task.Search()` |

**AgentChat Features:**
- `@file.ts` — mention/reference a file
- `@task-123` — reference another task
- History of previous messages
- Answer agent questions inline
- Stop + send correction

---

## CLI Commands

```bash
# Project initialization
kvelmo init                    # Initialize project

# Task lifecycle
kvelmo start [--from SOURCE]   # Start task (auto-starts socket)
kvelmo plan                    # Run planning
kvelmo implement               # Run implementation
kvelmo optimize                # Run optimization pass
kvelmo review                  # Enter review state
kvelmo submit                  # Submit to provider

# Navigation
kvelmo undo                    # Revert to previous checkpoint
kvelmo redo                    # Restore next checkpoint
kvelmo status                  # Show current state

# Server
kvelmo serve                   # Start web server (connects to socket)

# Management (via global socket)
kvelmo projects                # List all registered worktrees + their status
kvelmo workers                 # Show worker pool status (available/busy)
kvelmo disconnect              # Stop current worktree socket, unregister from global
```

---

## Implementation Phases

### Phase 1: Core Socket Infrastructure
1. internal/socket - Server, client, protocol (JSON-RPC + NDJSON)
2. Global socket plus worktree socket lifecycle
3. cmd/kvelmo - Binary with socket discovery
4. Minimal CLI: `init`, `status`, `disconnect`

### Phase 2: Worker Pool + Agent Integration
1. internal/worker - Pool, workers, job queue
2. agent - WebSocket connection layer
3. Claude agent (WebSocket-first, binary fallback)
4. Worker roles: Planner (Opus), Implementer (Sonnet)
5. Planning workflow: `plan` command

### Phase 2.5: Conductor + State Machine
1. internal/conductor - State machine (Task: None → Loaded → Planning → ...)
2. Integration with the worker pool (submit jobs, receive results)
3. State transitions trigger git operations

### Phase 3: Git Operations
1. internal/git - Branch, checkpoint, worktree
2. Undo/redo navigation
3. Integration with state transitions

### Phase 4: Provider Integration
1. internal/provider - Interface
2. internal/file, internal/github, internal/gitlab, internal/wrike
3. `start --from` sources

### Phase 5: Web UI
1. web/ - React dashboard
2. Socket client (WebSocket bridge to project socket)
3. Widget-based layout
4. Real-time streaming

### Phase 6: Desktop + Polish
1. desktop/ - Tauri wrapper
2. Security scanning (internal/security)
3. Quality checks (internal/quality)
4. Browser automation (internal/browser with Playwright)

---

## Migration Strategy

**Completed:** go-mehrhof → kvelmo migration using filter-repo

```
github.com/valksor/kvelmo/
├── prototype/                   # go-mehrhof (ALL 1000+ commits, prototype/ prefix in history)
│   ├── cmd/mehr/
│   ├── internal/
│   ├── web/
│   └── ...
│
├── cmd/kvelmo/                # NEW entry point
├── internal/                         # NEW architecture
└── web/                         # NEW web UI
```

**Reference approach:**
- Old code lives in `prototype/` for reference during development
- `git log prototype/internal/conductor/` shows full history
- New code at root, clean separation
- When done: optionally `git rm -r prototype/` or keep for posterity

---

## Key Files to Port/Reference

From **prototype/** (go-mehrhof):
- `prototype/internal/conductor/conductor.go` → Simplified into `internal/conductor/`
- `prototype/internal/workflow/machine.go` → State machine logic
- `prototype/internal/agent/claude/claude.go` → WebSocket implementation reference
- `prototype/internal/vcs/git.go` → Git operations

From **companion**:
- WebSocket relay pattern → Apply to socket server design
- NDJSON streaming → Protocol layer

From **mono-valksor/crea-pipe**:
- Protocol definitions → Reference for Claude/Codex message types

---

## Verification

### Phase 1 Test (Sockets)
```bash
cd ~/test-project
kvelmo init
# Check: global.sock exists in ~/.kvelmo/
# Check: worktree socket exists in ~/.kvelmo/worktrees/

kvelmo status  # Should show: "Task: None"

# In another terminal, same directory:
kvelmo status  # Should connect to SAME worktree socket

# Check global awareness:
kvelmo projects  # Should list this worktree
```

### Phase 2 Test (Workers)
```bash
kvelmo workers   # Should show: 2-3 workers, all "Available"

kvelmo start --from file:task.md
kvelmo plan      # Worker picks job, streams output

kvelmo workers   # Should show: 1 worker "Working", others "Available"
# After completion: all workers back to "Available"
```

### Parallelism Test
```bash
# Task with multiple components
kvelmo start --from file:multi-component-task.md
kvelmo plan --parallel   # Split into multiple jobs

kvelmo workers   # Should show: multiple workers "Working"
# Output streams from multiple workers simultaneously
```

### Phase 5 Test (Web)
```bash
kvelmo serve  # Starts web server, connects to global socket

# Open browser → should see:
# - Dashboard with worktree list (from global)
# - Select worktree → see task state
# - Worker pool widget showing availability

# In CLI: kvelmo plan
# In browser: should see live streaming output
```

---

## Success Criteria

1. **Socket per worktree** — Each worktree has its own state machine socket
2. **Global socket owns workers** — Shared pool (max 5–6), shared queue, cross-project
3. **Global mode UI** — Web/App starts with a project picker, not project-specific
4. **Seamless project switching** — Work on multiple projects, see all in one view
5. **Worker pool architecture** — Queue-based, parallel execution, not 1:1 blocking
6. **Model-per-role** — Opus for planning, Sonnet for implementation
7. **Descriptive states** — "Task: Planned" not "Idle"
8. **WebSocket-first agents** — Streaming, binary fallback
9. **Package-level granularity** — Clean boundaries, single repo
10. **App feel UI** — Widgets, not forms
11. **Minimal config** — Sensible defaults, just works
