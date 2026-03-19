# Workers

Monitor and manage the AI agent worker pool.

## Overview

The workers panel appears in two forms:

- **Agents panel** (sidebar) — Manage individual workers with add/remove controls
- **Workers widget** (dashboard) — Read-only overview of pool status and job stats

Both auto-refresh while connected (every 3-5 seconds).

## Stats

The stats bar shows at-a-glance pool health:

| Metric | Description |
|--------|-------------|
| Workers | Total count and how many are available |
| Available | Idle workers ready to accept jobs |
| Working | Workers currently executing a job |
| Queued | Jobs waiting for a free worker |
| Completed | Total jobs finished successfully |
| Failed | Total jobs that errored |

## Worker List

Each worker card shows:

- **Agent name** (claude, codex, custom) and worker ID
- **Status badge** — green for available, yellow for working, red for disconnected
- **Current job** — Displayed when the worker is active, with a progress indicator
- **Default badge** — Marks the default worker (cannot be removed)

## Adding Workers

1. Click **Add Worker**
2. Select an agent type from the dropdown (Claude, Codex, Custom)
3. Click **Add**

## Removing Workers

Hover over a non-default worker to reveal the remove button. Workers currently executing a job cannot be removed.
