# State Machine

kvelmo uses a state machine to manage the task lifecycle. Understanding states helps you know what actions are available and what to do when things go wrong.

## Core States

The main workflow progresses through these states:

| State          | Description                                      |
| -------------- | ------------------------------------------------ |
| `none`         | No active task                                   |
| `loaded`       | Task fetched from provider, branch created       |
| `planning`     | Agent generating specification (in progress)     |
| `planned`      | Specification complete, ready for implementation |
| `implementing` | Agent executing specification (in progress)      |
| `implemented`  | Implementation complete, ready for review        |
| `reviewing`    | Human review in progress                         |
| `submitted`    | Task submitted to provider (PR created)          |

## Optional States

These states are available from `implemented`:

| State         | Description                        |
| ------------- | ---------------------------------- |
| `simplifying` | Agent simplifying code for clarity |
| `optimizing`  | Agent improving code quality       |

## Auxiliary States

These states handle special situations:

| State     | Description                                    |
| --------- | ---------------------------------------------- |
| `failed`  | Error state (recoverable via `reset`)          |
| `waiting` | Waiting for user input                         |
| `paused`  | Execution paused (budget limits, manual pause) |

## State Transitions

### Normal Flow

```
none → loaded → planning → planned → implementing → implemented → reviewing → submitted
```

### Optional Phases

From `implemented`, you can optionally run:

- `simplify` → enters `simplifying` → returns to `implemented`
- `optimize` → enters `optimizing` → returns to `implemented`

### Recovery Transitions

| From      | Event    | To             | Description             |
| --------- | -------- | -------------- | ----------------------- |
| `failed`  | `reset`  | `loaded`       | Recover from failure    |
| `waiting` | `answer` | Previous state | Resume after user input |
| `paused`  | `resume` | Previous state | Resume after pause      |

### Abort Transitions

Any active state can transition to `failed` via `abort`.

## Checking State

```bash
# Show current state
kvelmo status

# Show state with details
kvelmo status --json
```

## Guards

Some transitions have guards that must pass:

| Transition                 | Guard                        |
| -------------------------- | ---------------------------- |
| `none` → `loaded`          | Must have a source reference |
| `loaded` → `planning`      | Must have a description      |
| `planned` → `implementing` | Must have specifications     |
| `reviewing` → `submitted`  | Must have a provider         |

## Events

Events trigger state transitions:

### Phase Events

- `start` — Begin working on task
- `plan` — Enter planning phase
- `implement` — Enter implementation phase
- `simplify` — Enter simplification phase
- `optimize` — Enter optimization phase
- `review` — Enter review phase
- `submit` — Submit to provider

### Completion Events

- `plan_done` — Planning completed
- `implement_done` — Implementation completed
- `simplify_done` — Simplification completed
- `optimize_done` — Optimization completed
- `review_done` — Review completed

### Navigation Events

- `undo` — Revert to previous checkpoint
- `redo` — Restore next checkpoint

### Control Events

- `error` — Error occurred
- `abort` — Abort task
- `reset` — Recover from failed state
- `wait` — Agent asked a question
- `answer` — User answered question
- `pause` — Pause execution
- `resume` — Resume after pause

## Diagram

```
                            ┌─────────────┐
                            │    none     │
                            └──────┬──────┘
                                   │ start
                            ┌──────▼──────┐
                            │   loaded    │◄────────┐
                            └──────┬──────┘         │
                                   │ plan           │ reset
                            ┌──────▼──────┐         │
                            │  planning   │    ┌────┴────┐
                            └──────┬──────┘    │  failed │
                                   │ plan_done └─────────┘
                            ┌──────▼──────┐         ▲
                            │   planned   │         │
                            └──────┬──────┘         │ abort
                                   │ implement      │ (from any)
                            ┌──────▼──────┐         │
                            │implementing │─────────┤
                            └──────┬──────┘         │
                                   │ implement_done │
                            ┌──────▼──────┐         │
              simplify ────►│ implemented │◄────────┤
              optimize ────►└──────┬──────┘         │
                                   │ review         │
                            ┌──────▼──────┐         │
                            │  reviewing  │─────────┘
                            └──────┬──────┘
                                   │ submit
                            ┌──────▼──────┐
                            │  submitted  │ (terminal)
                            └─────────────┘
```
