# Custom

Create your own agent implementation for kvelmo.

## Agent Interface

Custom agents must implement the agent interface:

```go
type Agent interface {
    // Start begins agent execution
    Start(ctx context.Context, prompt string) error

    // Events returns the event channel
    Events() <-chan Event

    // Stop terminates the agent
    Stop() error

    // Name returns the agent identifier
    Name() string
}
```

## Event Types

Agents emit events during execution:

```go
type Event struct {
    Type    EventType
    Content string
    Data    interface{}
}

const (
    EventToken      EventType = "token"
    EventToolCall   EventType = "tool_call"
    EventToolResult EventType = "tool_result"
    EventPermission EventType = "permission"
    EventCompletion EventType = "completion"
    EventError      EventType = "error"
)
```

## Registering a Custom Agent

Register your agent in the registry:

```go
import "github.com/valksor/kvelmo/agent"

func init() {
    agent.Register("myagent", NewMyAgent)
}

func NewMyAgent(config agent.Config) (agent.Agent, error) {
    return &MyAgent{config: config}, nil
}
```

## Configuration

Configure custom agents in settings:

```json
{
  "agents": {
    "myagent": {
      "type": "custom",
      "binary": "/path/to/myagent",
      "args": ["--mode", "streaming"]
    }
  }
}
```

## Example: Wrapper Agent

Wrap an existing CLI:

```go
type WrapperAgent struct {
    cmd    *exec.Cmd
    events chan Event
}

func (a *WrapperAgent) Start(ctx context.Context, prompt string) error {
    a.cmd = exec.CommandContext(ctx, "my-cli", "--prompt", prompt)
    stdout, _ := a.cmd.StdoutPipe()
    a.cmd.Start()

    go func() {
        scanner := bufio.NewScanner(stdout)
        for scanner.Scan() {
            a.events <- Event{
                Type:    EventToken,
                Content: scanner.Text(),
            }
        }
        a.events <- Event{Type: EventCompletion}
    }()

    return nil
}
```

## Tool Handling

Implement tool handling for agent capabilities:

```go
func (a *MyAgent) handleToolCall(call ToolCall) ToolResult {
    switch call.Name {
    case "read_file":
        content, err := os.ReadFile(call.Args["path"].(string))
        return ToolResult{Content: string(content), Error: err}
    // ... other tools
    }
}
```

## Permission Handling

Request permissions for sensitive operations:

```go
// Request permission
a.events <- Event{
    Type: EventPermission,
    Data: PermissionRequest{
        Tool:        "write_file",
        Description: "Write to /path/to/file",
    },
}

// Wait for approval
response := <-a.permissions
if !response.Approved {
    return errors.New("permission denied")
}
```

## Testing

Test your agent:

```bash
# Register and use
kvelmo config set default_agent myagent
kvelmo plan
```

## Related

- [Agents Overview](/agents/index.md)
- [Claude Agent](/agents/claude.md)
- [Codex Agent](/agents/codex.md)
