# JSON-RPC Protocol Reference

Mehrhof plugins communicate via JSON-RPC 2.0 over stdin/stdout. This document covers the protocol specification and all available methods.

## Protocol Overview

```
mehr ──stdin──> Plugin Process ──stdout──> mehr
       JSON-RPC                    JSON-RPC
```

- Mehr spawns the plugin as a subprocess
- Requests are sent as single-line JSON to stdin
- Responses are read as single-line JSON from stdout
- stderr is captured for logging (not shown by default)

## Message Format

### Request

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "provider.fetch",
  "params": { "id": "PROJ-123" }
}
```

| Field     | Type          | Required | Description        |
| --------- | ------------- | -------- | ------------------ |
| `jsonrpc` | string        | Yes      | Always `"2.0"`     |
| `id`      | number/string | Yes      | Request identifier |
| `method`  | string        | Yes      | Method name        |
| `params`  | object        | No       | Method parameters  |

### Response

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": { "id": "PROJ-123", "title": "..." }
}
```

| Field     | Type          | Required    | Description                  |
| --------- | ------------- | ----------- | ---------------------------- |
| `jsonrpc` | string        | Yes         | Always `"2.0"`               |
| `id`      | number/string | Yes         | Matching request ID          |
| `result`  | any           | Conditional | Success result (if no error) |
| `error`   | object        | Conditional | Error object (if failed)     |

### Error Response

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32000,
    "message": "Task not found",
    "data": { "task_id": "PROJ-999" }
  }
}
```

### Notification (Streaming)

Notifications have no `id` field and don't expect a response:

```json
{
  "jsonrpc": "2.0",
  "method": "stream",
  "params": { "type": "text", "data": "Hello" }
}
```

## Error Codes

### Standard JSON-RPC Errors

| Code   | Message          | Description                |
| ------ | ---------------- | -------------------------- |
| -32700 | Parse error      | Invalid JSON received      |
| -32600 | Invalid request  | Not a valid Request object |
| -32601 | Method not found | Method doesn't exist       |
| -32602 | Invalid params   | Invalid method parameters  |
| -32603 | Internal error   | Internal JSON-RPC error    |

### Plugin-Specific Errors

| Code   | Message              | Description                 |
| ------ | -------------------- | --------------------------- |
| -32000 | Plugin error         | General plugin error        |
| -32001 | Not implemented      | Method not implemented      |
| -32002 | Capability error     | Missing required capability |
| -32003 | Authentication error | Invalid credentials         |
| -32004 | Rate limit           | API rate limit exceeded     |
| -32005 | Not found            | Resource not found          |
| -32006 | Validation error     | Input validation failed     |

## Provider Methods

### provider.init

Initialize the provider with configuration.

```json
// Request
{"jsonrpc":"2.0","id":1,"method":"provider.init","params":{"config":{"url":"..."}}}

// Response
{"jsonrpc":"2.0","id":1,"result":{"capabilities":["read","snapshot","list"]}}
```

### provider.match

Check if input matches this provider's scheme.

```json
// Request
{"jsonrpc":"2.0","id":2,"method":"provider.match","params":{"input":"jira:PROJ-123"}}

// Response
{"jsonrpc":"2.0","id":2,"result":{"matches":true}}
```

### provider.parse

Parse input string to extract task ID.

```json
// Request
{"jsonrpc":"2.0","id":3,"method":"provider.parse","params":{"input":"jira:PROJ-123"}}

// Response (success)
{"jsonrpc":"2.0","id":3,"result":{"id":"PROJ-123"}}

// Response (error)
{"jsonrpc":"2.0","id":3,"result":{"error":"Invalid format"}}
```

### provider.fetch

Fetch task details. Returns a WorkUnit.

```json
// Request
{"jsonrpc":"2.0","id":4,"method":"provider.fetch","params":{"id":"PROJ-123"}}

// Response
{
  "jsonrpc":"2.0","id":4,
  "result":{
    "id":"abc123",
    "externalId":"PROJ-123",
    "externalKey":"PROJ-123",
    "taskType":"story",
    "provider":"jira",
    "title":"Implement feature",
    "description":"Full description...",
    "status":"open",
    "priority":2,
    "labels":["frontend"],
    "source":{"reference":"https://jira.example.com/PROJ-123"}
  }
}
```

### provider.snapshot

Capture task content for storage.

```json
// Request
{"jsonrpc":"2.0","id":5,"method":"provider.snapshot","params":{"id":"PROJ-123"}}

// Response
{"jsonrpc":"2.0","id":5,"result":{"content":"# Task Content\n\n..."}}
```

### provider.list

List tasks (requires `list` capability).

```json
// Request
{
  "jsonrpc":"2.0","id":6,"method":"provider.list",
  "params":{"filter":{"status":"open","labels":["bug"]},"limit":20,"offset":0}
}

// Response
{
  "jsonrpc":"2.0","id":6,
  "result":{
    "items":[{"id":"PROJ-1","title":"Bug 1"},{"id":"PROJ-2","title":"Bug 2"}],
    "total":42,
    "hasMore":true
  }
}
```

## Agent Methods

### agent.init

Initialize agent with configuration.

```json
// Request
{"jsonrpc":"2.0","id":1,"method":"agent.init","params":{"config":{"model":"gpt-4"}}}

// Response
{"jsonrpc":"2.0","id":1,"result":{"name":"openai","model":"gpt-4","capabilities":["streaming","tool_use"]}}
```

### agent.available

Check if agent is ready.

```json
// Request
{"jsonrpc":"2.0","id":2,"method":"agent.available","params":{}}

// Response (available)
{"jsonrpc":"2.0","id":2,"result":{"available":true}}

// Response (not available)
{"jsonrpc":"2.0","id":2,"result":{"available":false,"reason":"API key not set"}}
```

### agent.run

Execute a prompt with streaming.

```json
// Request
{
  "jsonrpc":"2.0","id":3,"method":"agent.run",
  "params":{
    "prompt":"Implement login validation",
    "context":{"task_id":"abc123","phase":"implementing"},
    "tools":[{"name":"read_file","input_schema":{...}}]
  }
}

// Stream events (notifications, no id)
{"jsonrpc":"2.0","method":"stream","params":{"type":"text","data":"Here's the code:"}}
{"jsonrpc":"2.0","method":"stream","params":{"type":"file","data":{"path":"src/auth.py","content":"..."}}}
{"jsonrpc":"2.0","method":"stream","params":{"type":"complete","data":null}}

// Final response
{"jsonrpc":"2.0","id":3,"result":{"status":"complete","tokens_used":500}}
```

### agent.tool_result

Send tool execution result back to agent.

```json
// Request
{"jsonrpc":"2.0","id":4,"method":"agent.tool_result","params":{"tool_id":"call_123","result":"file contents..."}}

// Response
{"jsonrpc":"2.0","id":4,"result":{"acknowledged":true}}
```

## Workflow Methods

### workflow.init

Initialize and declare extensions.

```json
// Request
{"jsonrpc":"2.0","id":1,"method":"workflow.init","params":{"config":{}}}

// Response
{
  "jsonrpc":"2.0","id":1,
  "result":{
    "phases":[{"name":"approval","description":"Manager approval","after":"reviewing"}],
    "guards":[{"name":"requiresApproval","description":"Check if needs approval"}],
    "effects":[{"name":"notifySlack","description":"Send Slack message"}]
  }
}
```

### workflow.evaluateGuard

Evaluate a guard condition.

```json
// Request
{
  "jsonrpc":"2.0","id":2,"method":"workflow.evaluateGuard",
  "params":{"guard":"requiresApproval","context":{"labels":["critical"]}}
}

// Response (allowed)
{"jsonrpc":"2.0","id":2,"result":{"allowed":true}}

// Response (blocked)
{"jsonrpc":"2.0","id":2,"result":{"allowed":false,"reason":"Critical tasks need approval"}}
```

### workflow.executeEffect

Execute a side effect.

```json
// Request
{
  "jsonrpc":"2.0","id":3,"method":"workflow.executeEffect",
  "params":{"effect":"notifySlack","context":{"task_id":"abc123","event":"phase_entered"}}
}

// Response
{"jsonrpc":"2.0","id":3,"result":{"success":true,"message":"Notification sent"}}
```

## Common Methods

### shutdown

Graceful shutdown request. Plugin should exit after responding.

```json
// Request
{"jsonrpc":"2.0","id":99,"method":"shutdown","params":{}}

// Response
{"jsonrpc":"2.0","id":99,"result":{"status":"shutting_down"}}
```

### ping

Health check.

```json
// Request
{"jsonrpc":"2.0","id":100,"method":"ping","params":{}}

// Response
{"jsonrpc":"2.0","id":100,"result":{"pong":true}}
```

## Implementation Template

```python
#!/usr/bin/env python3
import json
import sys

def handle_request(request):
    method = request.get("method", "")
    params = request.get("params", {})

    # Route to handler
    if method == "provider.init":
        return {"capabilities": ["read"]}
    elif method == "shutdown":
        return {"status": "shutting_down"}
    elif method == "ping":
        return {"pong": True}
    else:
        return {"error": {"code": -32601, "message": f"Method not found: {method}"}}

def main():
    for line in sys.stdin:
        if not line.strip():
            continue

        try:
            request = json.loads(line)
            result = handle_request(request)

            response = {"jsonrpc": "2.0", "id": request.get("id")}

            # Check if result is an error
            if isinstance(result, dict) and "error" in result and isinstance(result["error"], dict):
                response["error"] = result["error"]
            else:
                response["result"] = result

            print(json.dumps(response), flush=True)

            # Exit on shutdown
            if request.get("method") == "shutdown":
                break

        except json.JSONDecodeError as e:
            print(json.dumps({
                "jsonrpc": "2.0",
                "id": None,
                "error": {"code": -32700, "message": f"Parse error: {e}"}
            }), flush=True)
        except Exception as e:
            print(json.dumps({
                "jsonrpc": "2.0",
                "id": request.get("id") if 'request' in dir() else None,
                "error": {"code": -32603, "message": f"Internal error: {e}"}
            }), flush=True)

if __name__ == "__main__":
    main()
```

## Best Practices

1. **Always flush stdout** - Use `flush=True` to ensure output is sent immediately
2. **Handle EOF gracefully** - Exit cleanly when stdin closes
3. **Log to stderr** - Debug output should go to stderr, not stdout
4. **Validate early** - Check parameters before processing
5. **Return clear errors** - Use appropriate error codes and messages
6. **Support shutdown** - Handle the shutdown method for graceful termination
