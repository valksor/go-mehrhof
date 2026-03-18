# Agent Strategies

Strategies define how agents reason about tasks. They control prompt construction and output evaluation, allowing different reasoning approaches per phase or task type.

## Built-in Strategies

| Strategy     | Description                                          |
|-------------|------------------------------------------------------|
| `direct`    | Default. Passes prompts through with minimal wrapping |
| `iterative` | Adds self-review loop. Checks output for TODO/FIXME markers |

### Direct Strategy

The default strategy. Prepends context (if any) and appends constraints. Output is always marked as complete.

This matches kvelmo's existing behavior before strategies were introduced.

### Iterative Strategy

Instructs the agent to implement, self-review, and fix issues in a single pass. After execution, the output is scanned for unresolved markers (`TODO`, `FIXME`, `HACK`, `XXX`, `NEEDS_REVIEW`). If found, the conductor can re-run the phase with the prior output as context.

## Configuring Strategies

Set the default strategy in `kvelmo.yaml` under `agent`:

```yaml
agent:
  strategy: direct
```

Or override per phase using the phase names `plan`, `implement`, `simplify`, `optimize`:

```yaml
agent:
  strategy: direct
  phase_strategy:
    plan: direct
    implement: iterative
```

## Discovering Strategies

List registered strategies via the global socket:

```bash
kvelmo rpc strategy.list
```

## Strategy Interface

Strategies implement three methods:

```go
type Strategy interface {
    Name() string
    BuildPrompt(input Input) string
    EvaluateOutput(output string) Output
}
```

- **BuildPrompt** receives the task description, accumulated context, variable pool values, and constraints. Returns the final prompt sent to the agent.
- **EvaluateOutput** examines the agent's output and returns a status: `complete`, `needs_iteration`, or `blocked`. The conductor calls this after each phase completes — if the status is `needs_iteration`, the phase is re-submitted (up to 3 times by default).

## Creating a Custom Strategy

1. Create a Go file in `pkg/agent/strategy/`:

```go
package strategy

type MyStrategy struct{}

func (m *MyStrategy) Name() string { return "my-strategy" }

func (m *MyStrategy) BuildPrompt(input Input) string {
    // Build your custom prompt
    return input.Task
}

func (m *MyStrategy) EvaluateOutput(output string) Output {
    return Output{Content: output, Status: "complete"}
}
```

2. Register it in the strategy package's `init()` function or at startup.

3. Reference it by name in configuration: `agent: { strategy: my-strategy }`
