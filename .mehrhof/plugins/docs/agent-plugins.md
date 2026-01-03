# Agent Plugin Development

Agent plugins add custom AI backends to Mehrhof for planning and implementation phases. Use them to integrate local LLMs, custom models, or alternative AI providers.

## Manifest Structure

```yaml
version: "1"
name: "ollama"
type: "agent"
description: "Local LLM via Ollama"
protocol: "1"

executable:
  command: ["python3", "./ollama-agent.py"]

agent:
  name: "ollama"
  streaming: true
  capabilities:
    - streaming
    - tool_use

env:
  OLLAMA_MODEL:
    description: "Model to use (e.g., codellama:13b)"
    required: false
    default: "codellama:13b"
```

## Required Methods

### agent.init

Initialize the agent with configuration.

**Request:**

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "agent.init",
  "params": {
    "config": {
      "model": "codellama:13b"
    }
  }
}
```

**Response:**

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "name": "ollama",
    "model": "codellama:13b",
    "capabilities": ["streaming", "tool_use"]
  }
}
```

### agent.available

Check if the agent is ready to accept requests.

**Request:**

```json
{ "jsonrpc": "2.0", "id": 2, "method": "agent.available", "params": {} }
```

**Response:**

```json
{ "jsonrpc": "2.0", "id": 2, "result": { "available": true } }
```

**Not Available:**

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "available": false,
    "reason": "Ollama server not running"
  }
}
```

### agent.run

Execute a prompt. This is the main method that streams responses.

**Request:**

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "agent.run",
  "params": {
    "prompt": "Create a function that validates email addresses",
    "context": {
      "task_id": "abc123",
      "phase": "implementing",
      "files": ["src/validators.py"]
    },
    "tools": [
      {
        "name": "read_file",
        "description": "Read a file's contents",
        "input_schema": {
          "type": "object",
          "properties": {
            "path": { "type": "string" }
          },
          "required": ["path"]
        }
      }
    ]
  }
}
```

## Streaming Responses

Agent plugins stream responses via JSON-RPC notifications (no `id` field):

````json
{"jsonrpc":"2.0","method":"stream","params":{"type":"text","data":"Here's the implementation"}}
{"jsonrpc":"2.0","method":"stream","params":{"type":"text","data":":\n\n```python\n"}}
{"jsonrpc":"2.0","method":"stream","params":{"type":"file","data":{"path":"src/validate.py","content":"..."}}}
{"jsonrpc":"2.0","method":"stream","params":{"type":"usage","data":{"input_tokens":150,"output_tokens":200}}}
{"jsonrpc":"2.0","method":"stream","params":{"type":"complete","data":null}}
````

After streaming completes, send the final response:

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": { "status": "complete", "tokens_used": 350 }
}
```

## Stream Event Types

| Type          | Description     | Data Format                                       |
| ------------- | --------------- | ------------------------------------------------- |
| `text`        | Text content    | `string`                                          |
| `tool_use`    | Tool invocation | `{"id":"...","name":"read_file","input":{...}}`   |
| `tool_result` | Tool result     | `{"id":"...","result":"..."}`                     |
| `file`        | File operation  | `{"path":"...","content":"...","action":"write"}` |
| `usage`       | Token usage     | `{"input_tokens":N,"output_tokens":N}`            |
| `thinking`    | Model reasoning | `string` (for models that support it)             |
| `complete`    | Stream finished | `null`                                            |
| `error`       | Error occurred  | `{"code":N,"message":"..."}`                      |

## Tool Use Flow

When the model wants to use a tool:

1. Agent sends `tool_use` event:

   ```json
   {
     "method": "stream",
     "params": {
       "type": "tool_use",
       "data": {
         "id": "tool_1",
         "name": "read_file",
         "input": { "path": "src/main.py" }
       }
     }
   }
   ```

2. Mehr executes the tool and sends result back via `agent.tool_result`:

   ```json
   {
     "jsonrpc": "2.0",
     "id": 4,
     "method": "agent.tool_result",
     "params": { "tool_id": "tool_1", "result": "file contents..." }
   }
   ```

3. Agent continues generation with the tool result.

## Capabilities

| Capability       | Description                    |
| ---------------- | ------------------------------ |
| `streaming`      | Supports streaming responses   |
| `tool_use`       | Supports tool/function calling |
| `code_execution` | Can execute code safely        |
| `vision`         | Supports image inputs          |
| `long_context`   | Supports >32k context          |

## Complete Example: Ollama Agent

```python
#!/usr/bin/env python3
"""Ollama agent plugin for Mehrhof."""

import json
import os
import sys
import urllib.request

OLLAMA_URL = os.environ.get("OLLAMA_URL", "http://localhost:11434")
OLLAMA_MODEL = os.environ.get("OLLAMA_MODEL", "codellama:13b")

def stream_event(event_type, data):
    """Send a streaming event."""
    notification = {
        "jsonrpc": "2.0",
        "method": "stream",
        "params": {"type": event_type, "data": data}
    }
    print(json.dumps(notification), flush=True)

def handle_init(params):
    return {
        "name": "ollama",
        "model": OLLAMA_MODEL,
        "capabilities": ["streaming"]
    }

def handle_available(params):
    try:
        req = urllib.request.Request(f"{OLLAMA_URL}/api/tags")
        with urllib.request.urlopen(req, timeout=5) as resp:
            return {"available": True}
    except Exception as e:
        return {"available": False, "reason": str(e)}

def handle_run(params):
    prompt = params.get("prompt", "")
    context = params.get("context", {})

    # Build system prompt
    system = f"""You are an AI assistant helping with software development.
Current phase: {context.get('phase', 'unknown')}
Task: {context.get('task_id', 'unknown')}

Respond with clear, well-structured code and explanations."""

    # Call Ollama API with streaming
    request_data = json.dumps({
        "model": OLLAMA_MODEL,
        "prompt": prompt,
        "system": system,
        "stream": True
    }).encode()

    req = urllib.request.Request(
        f"{OLLAMA_URL}/api/generate",
        data=request_data,
        headers={"Content-Type": "application/json"}
    )

    total_tokens = 0
    try:
        with urllib.request.urlopen(req) as resp:
            for line in resp:
                if not line.strip():
                    continue

                chunk = json.loads(line)

                # Stream text content
                if chunk.get("response"):
                    stream_event("text", chunk["response"])

                # Track completion
                if chunk.get("done"):
                    total_tokens = chunk.get("eval_count", 0) + chunk.get("prompt_eval_count", 0)
                    stream_event("usage", {
                        "input_tokens": chunk.get("prompt_eval_count", 0),
                        "output_tokens": chunk.get("eval_count", 0)
                    })

        stream_event("complete", None)
        return {"status": "complete", "tokens_used": total_tokens}

    except Exception as e:
        stream_event("error", {"code": -32000, "message": str(e)})
        return {"status": "error", "error": str(e)}

def handle_request(request):
    method = request.get("method", "")
    params = request.get("params", {})

    handlers = {
        "agent.init": handle_init,
        "agent.available": handle_available,
        "agent.run": handle_run,
    }

    if method in handlers:
        return handlers[method](params)

    return {"error": {"code": -32601, "message": f"Method not found: {method}"}}

def main():
    for line in sys.stdin:
        if not line.strip():
            continue

        try:
            request = json.loads(line)
            result = handle_request(request)

            response = {"jsonrpc": "2.0", "id": request.get("id")}
            if isinstance(result, dict) and "error" in result and "code" in result.get("error", {}):
                response["error"] = result["error"]
            else:
                response["result"] = result

            print(json.dumps(response), flush=True)

        except json.JSONDecodeError:
            print(json.dumps({
                "jsonrpc": "2.0",
                "id": None,
                "error": {"code": -32700, "message": "Parse error"}
            }), flush=True)

if __name__ == "__main__":
    main()
```

## Using Agent Aliases

Instead of creating a full plugin, you can create agent aliases in config:

```yaml
# .mehrhof/config.yaml
agents:
  fast-local:
    extends: ollama
    description: "Fast local model"
    env:
      OLLAMA_MODEL: "codellama:7b"

  accurate-local:
    extends: ollama
    description: "More accurate local model"
    env:
      OLLAMA_MODEL: "codellama:34b"
```

Use with:

```bash
mehr plan --agent fast-local
mehr implement --agent accurate-local
```

## Testing Your Agent

```bash
python3 ./my-agent.py
{"jsonrpc":"2.0","id":1,"method":"agent.available","params":{}}

{"jsonrpc":"2.0","id":2,"method":"agent.run","params":{"prompt":"Hello, world!"}}

mehr plugins validate my-agent

mehr plan --agent my-agent
```

## Tips

1. **Flush immediately** - Always use `flush=True` when printing to stdout
2. **Stream early** - Start streaming as soon as content is available
3. **Handle cancellation** - Watch for stdin EOF to stop generation
4. **Report usage** - Send token usage for cost tracking
5. **Test tool use** - If supporting tools, test the full request/response cycle
