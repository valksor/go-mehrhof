# Dashboard

The dashboard is your central view for managing tasks in kvelmo.

## Opening the Dashboard

Start the server and open your browser:

```bash
kvelmo serve --open
```

Or navigate to http://localhost:6337.

## Dashboard Layout

The dashboard consists of several panels:

### Project Selector

At the top, select which project to work on. kvelmo can manage multiple projects simultaneously.

### Task Status

Shows the current task state and progress:
- Task title and description
- Current state (loaded, planning, implemented, etc.)
- Time elapsed
- Agent activity

### Actions Panel

Workflow buttons for the current state:
- **Plan** — Generate specification
- **Implement** — Execute specification
- **Simplify** — Optional code cleanup
- **Optimize** — Optional optimization
- **Review** — Start review phase
- **Submit** — Create PR

Buttons are enabled/disabled based on the current state.

### Output Panel

Real-time output from the agent:
- Agent thoughts and reasoning
- Tool calls and results
- Errors and warnings

### Sidebar

Access to additional panels:
- Files — Browse project files
- Changes — View file diffs
- Checkpoints — Navigate undo/redo history
- Workers — Monitor worker pool
- Memory — Semantic memory
- Screenshots — Screenshot gallery
- Browser — Browser automation
- Settings — Configuration

## Task States

The dashboard shows the current state with visual indicators:

| State          | Indicator      | Description                |
|----------------|----------------|----------------------------|
| `none`         | Gray           | No active task             |
| `loaded`       | Blue           | Task loaded, ready to plan |
| `planning`     | Yellow spinner | Planning in progress       |
| `planned`      | Green          | Ready to implement         |
| `implementing` | Yellow spinner | Implementation in progress |
| `implemented`  | Green          | Ready to review            |
| `reviewing`    | Yellow         | Review in progress         |
| `submitted`    | Green check    | Task complete              |
| `failed`       | Red            | Error occurred             |

## Browser Automation

The **Browser** panel provides Playwright-powered browser automation directly from the dashboard. Use it to interact with web applications during implementation or testing.

- **Navigate** — Enter a URL to open in the browser
- **Click / Type / Fill** — Interact with page elements by CSS selector
- **Screenshot** — Capture the current page state
- **Evaluate** — Run JavaScript in the browser context
- **Console / Network** — Inspect console messages and network requests

The browser panel connects to a headless Chromium instance managed by kvelmo. Install it first with `kvelmo browser install` or via the panel's install button.

## Screenshots

The **Screenshots** panel manages captured screenshots. Screenshots can be:

- Captured from the browser automation panel
- Taken via `kvelmo screenshots capture`
- Attached to chat messages for visual context

The panel shows thumbnails with timestamps. Click to view full-size, or delete screenshots you no longer need.

## Refreshing

The dashboard updates automatically via WebSocket. No manual refresh needed.

Prefer the command line? See [CLI Reference](/cli/index.md).
