# Agents

Agents are the execution layer underneath kvelmo's workflow.

kvelmo does not replace them. It coordinates them through task state, permissions, checkpoints, workflow transitions, and interface synchronization.

## Supported Agent Paths

| Agent | Description |
|-------|-------------|
| [Claude](/agents/claude.md) | Claude-based local agent path |
| [Codex](/agents/codex.md) | Codex-based local agent path |
| [Custom](/agents/custom.md) | Custom integration path |
| [Ollama](/agents/ollama.md) | Local model integration |
| [OpenAI](/agents/openai.md) | API-backed OpenAI integration |

## How Agents Fit into kvelmo

Conceptually:

```text
kvelmo -> task state + workflow + permissions + checkpoints -> agent -> result
```

That means:

- the agent performs reasoning and execution
- kvelmo controls when phases run
- kvelmo records state and recovery points
- outputs are surfaced through Web UI, CLI, desktop app, and TUI

## Agent Selection

Agent choice can come from:

1. command-level overrides
2. task-level configuration
3. project settings
4. global settings
5. local detection and defaults

## Per-Phase Selection

kvelmo supports phase-aware selection, so planning and implementation do not have to use the same agent path.

That is useful when:

- one agent is better for planning
- another is better for code generation
- a project needs local or API-backed fallback behavior

## Permissions and Controls

Agents do not operate without surrounding controls.

kvelmo can mediate:

- permission prompts
- write approvals
- workflow gates
- checkpoint creation
- output recording and replay

## Events and Streaming

During execution, agent activity can be surfaced as streamed output and structured events, which then appear across the available interfaces.

## Related

- [Claude](/agents/claude.md)
- [Codex](/agents/codex.md)
- [Custom](/agents/custom.md)
- [Strategies](/agents/strategies.md)
- [Permissions](/agents/permissions.md)
