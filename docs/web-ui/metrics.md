# Metrics

View real-time system metrics for the kvelmo server.

## Opening

Open the Metrics panel from the **Tools** dropdown menu in the project toolbar. It loads as a modal overlay.

## Available Metrics

The panel calls `metrics` and displays counters and latency data:

| Metric           | Description                             |
| ---------------- | --------------------------------------- |
| Jobs Submitted   | Total jobs submitted to the worker pool |
| Jobs Completed   | Successfully completed jobs             |
| Jobs Failed      | Jobs that encountered errors            |
| Jobs In Progress | Currently running jobs                  |
| RPC Requests     | Total JSON-RPC requests processed       |
| RPC Errors       | Requests that returned errors           |
| Avg Latency      | Average RPC request latency (ms)        |
| P99 Latency      | 99th percentile latency (ms)            |
| Agent Connects   | Agent WebSocket connections             |
| Tokens Consumed  | Total tokens used across all agents     |

When per-agent breakdowns are available, each agent's tokens, requests, errors, and latency are shown in a separate table.

## Metrics History

Toggle **History** to view a time-series chart of key metrics over time, powered by the `metrics.history` RPC method.

## Refreshing

Click **Refresh** to re-fetch metrics. The panel also auto-refreshes on a periodic interval while open.

## Related

- [kvelmo stats](/cli/stats.md) — CLI equivalent
- `/stats` — Chat command equivalent
