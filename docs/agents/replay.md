# Replay Agent

The replay agent replays recorded agent sessions deterministically, enabling reproducible testing and debugging of task workflows.

## Purpose

When kvelmo records an agent session (via the recorder subsystem), the replay agent can replay that recording step-by-step. This is useful for:

- **Deterministic testing** — verify that a workflow produces expected results without calling a live AI agent
- **Debugging** — replay a failed session to inspect what happened at each step
- **CI pipelines** — run integration tests against recorded sessions for fast, predictable builds

## How It Works

1. An agent session is recorded to a JSONL file via `agent/recorder`
2. The replay agent reads the recording file
3. When asked to run a task, it replays the recorded responses in order instead of calling a live model
4. Timing is preserved from the original recording for realistic playback

## Configuration

### Using for a Specific Task

```bash
kvelmo start --from file:task.md --agent replay --agent-opts recording=path/to/session.jsonl
```

### In Tests

```go
agent, err := replay.New("testdata/session.jsonl")
```

The replay agent implements the standard `agent.Agent` interface, so it can be used anywhere a regular agent is expected.

## Limitations

- Replay output is fixed — the agent cannot adapt to different input than what was recorded
- If the task state diverges from the recording, responses may not make sense
- Best used for regression testing and debugging, not production workflows

## Creating Recordings

Recordings are captured automatically during agent sessions. Use the `recordings` CLI command to manage them:

```bash
kvelmo recordings list          # List available recordings
kvelmo recordings view <id>     # View a recording
kvelmo recordings replay <id>   # Replay a recording
```

## Related

- [Recordings](/cli/recordings.md) — View and manage recorded sessions
- [Claude](/agents/claude.md) — Primary live agent
- [Custom](/agents/custom.md) — Custom agent interface
