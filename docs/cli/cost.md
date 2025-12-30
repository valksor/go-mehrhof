# mehr cost

Show token usage and costs for the active task.

## Usage

```bash
mehr cost                    # Show cost for active task
mehr cost --by-step          # Break down by workflow step
mehr cost --all              # Show costs for all tasks
mehr cost --summary          # Show aggregate summary across all tasks
```

## Description

The `cost` command displays token usage and associated costs for AI agent operations. Costs are tracked automatically during workflow phases (planning, implementing, reviewing).

Cost tracking works by:
1. Each agent call returns usage stats after completion (via Claude CLI's `message_stop` event)
2. Usage is aggregated in `work.yaml` under the `costs` section
3. Costs persist across sessions and accumulate for the task lifetime

## Flags

| Flag            | Description                              | Default |
| --------------- | ---------------------------------------- | ------- |
| `--by-step`     | Break down costs by workflow step        | `false` |
| `--all`         | Show costs for all tasks in workspace    | `false` |
| `--summary`     | Show aggregate summary across all tasks  | `false` |
| `--json`        | Output as JSON for programmatic use      | `false` |

## Output

### Active Task Cost

```bash
$ mehr cost

Task: Add user authentication
Key: AUTH-001

Input Tokens:     125,000
Output Tokens:     45,000
Cached Tokens:     80,000
Total Cost:        $1.2350
```

### By Step Breakdown

```bash
$ mehr cost --by-step

Task: Add user authentication
Key: AUTH-001

Input Tokens:     125,000
Output Tokens:     45,000
Cached Tokens:     80,000
Total Cost:        $1.1000

By Step:
  planning:      $0.4500 (45,000 input + 15,000 output)
  implementing:  $0.6500 (60,000 input + 25,000 output)
```

### All Tasks

```bash
$ mehr cost --all

TASK ID     TITLE                    INPUT      OUTPUT     COST
abc123      Add authentication       125,000    45,000     $1.2350
def456      Fix database query        45,000     8,000     $0.2790
ghi789      Update docs                8,000     3,000     $0.0630

Total: 178,000 input tokens, 56,000 output tokens, $1.5770 total
```

### Summary

```bash
$ mehr cost --summary

All Tasks Summary:
  Tasks: 3
  Input Tokens: 178,000
  Output Tokens: 56,000
  Cached Tokens: 80,000
  Total Cost: $1.5770
```

### JSON Output

For programmatic access, use `--json`:

```bash
mehr cost --json
```

Output:

```json
{
  "task_id": "abc12345",
  "title": "Add user authentication",
  "total_tokens": 170000,
  "input_tokens": 125000,
  "output_tokens": 45000,
  "cached_tokens": 80000,
  "cached_percent": 47.1,
  "total_cost_usd": 1.235,
  "by_step": {
    "planning": {
      "input_tokens": 45000,
      "output_tokens": 15000,
      "cached_tokens": 30000,
      "total_tokens": 60000,
      "cost_usd": 0.45,
      "calls": 2
    },
    "implementing": {
      "input_tokens": 60000,
      "output_tokens": 25000,
      "cached_tokens": 50000,
      "total_tokens": 85000,
      "cost_usd": 0.65,
      "calls": 5
    }
  }
}
```

## Cost Data Location

Cost data is stored in `.mehrhof/work/<task-id>/work.yaml`:

```yaml
costs:
  total_input_tokens: 125000
  total_output_tokens: 45000
  total_cached_tokens: 80000
  total_cost_usd: 1.235
  by_step:
    planning:
      input_tokens: 45000
      output_tokens: 15000
      cached_tokens: 30000
      cost_usd: 0.45
      calls: 2
    implementing:
      input_tokens: 60000
      output_tokens: 25000
      cached_tokens: 50000
      cost_usd: 0.65
      calls: 5
```

## Pricing

Costs are calculated based on the agent's model pricing. For Claude models via Claude CLI, pricing follows Anthropic's published rates.

Note: Cached tokens are billed at 90% discount compared to standard input tokens.
