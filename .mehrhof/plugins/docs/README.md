# Plugin Development Guide

Create custom plugins to extend Mehrhof with new task sources, AI backends, or workflow phases.

## Plugin Types

| Type         | Purpose                  | Use Cases                                   |
| ------------ | ------------------------ | ------------------------------------------- |
| **Provider** | Custom task sources      | Jira, YouTrack, Linear, Notion, custom APIs |
| **Agent**    | AI backends              | Local LLMs, Codex, custom models            |
| **Workflow** | State machine extensions | Approval steps, notifications, CI triggers  |

## Quick Start

### 1. Create Plugin Directory

```bash
# Project-local plugin
mkdir -p .mehrhof/plugins/my-plugin

# OR global plugin (available to all projects)
mkdir -p ~/.mehrhof/plugins/my-plugin
```

### 2. Create Manifest

Every plugin needs a `plugin.yaml`:

```yaml
version: "1"
name: "my-plugin"
type: "provider" # provider | agent | workflow
description: "My custom plugin"
protocol: "1"

executable:
  command: ["python3", "./plugin.py"]

provider:
  name: "my-plugin"
  schemes: ["my"]
  priority: 50
  capabilities:
    - read
```

### 3. Implement Handler

```python
#!/usr/bin/env python3
import json
import sys

def handle_request(request):
    method = request.get("method")
    params = request.get("params", {})

    if method == "provider.init":
        return {"capabilities": ["read"]}
    elif method == "provider.match":
        inp = params.get("input", "")
        return {"matches": inp.startswith("my:")}
    elif method == "provider.parse":
        return {"id": params["input"].replace("my:", "")}
    elif method == "provider.fetch":
        return {
            "id": params["id"],
            "title": f"Task {params['id']}",
            "description": "Task content here",
            "status": "open",
            "provider": "my-plugin"
        }

    return {"error": {"code": -32601, "message": "Method not found"}}

def main():
    for line in sys.stdin:
        request = json.loads(line)
        result = handle_request(request)
        response = {
            "jsonrpc": "2.0",
            "id": request.get("id"),
            "result": result
        }
        print(json.dumps(response), flush=True)

if __name__ == "__main__":
    main()
```

### 4. Enable Plugin

Add to `.mehrhof/config.yaml`:

```yaml
plugins:
  enabled:
    - my-plugin
```

### 5. Test

```bash
mehr plugins list              # Should show your plugin
mehr plugins validate my-plugin  # Validate configuration
mehr start my:test-task        # Use it!
```

## Documentation

| Guide                                   | Description                |
| --------------------------------------- | -------------------------- |
| [Provider Plugins](provider-plugins.md) | Create custom task sources |
| [Agent Plugins](agent-plugins.md)       | Create custom AI backends  |
| [Workflow Plugins](workflow-plugins.md) | Extend the state machine   |
| [Protocol Reference](protocol.md)       | JSON-RPC 2.0 specification |

## Plugin Locations

Plugins are discovered from two locations:

```
~/.mehrhof/plugins/     # Global (all projects)
.mehrhof/plugins/       # Project-local (overrides global)
```

Project plugins take precedence over global plugins with the same name.

## Best Practices

1. **Fast startup** - Plugins are spawned on demand; keep initialization quick
2. **Graceful shutdown** - Handle stdin EOF and `shutdown` method
3. **Validate early** - Check required env vars in `init`
4. **Clear errors** - Return meaningful error messages with appropriate codes
5. **Flush output** - Always `flush=True` when writing to stdout
6. **Declare capabilities** - Only declare what you actually implement

## Language Support

Plugins can be written in any language. Common choices:

| Language | Pros                              | Cons                   |
| -------- | --------------------------------- | ---------------------- |
| Python   | Easy, no compilation, good stdlib | Slower startup         |
| Go       | Fast, single binary               | Requires compilation   |
| Node.js  | Good for API integrations         | Requires node runtime  |
| Bash     | Simple cases                      | Limited error handling |

## Debugging

### Manual Testing

```bash
# Run plugin directly
python3 .mehrhof/plugins/my-plugin/plugin.py

# Send test requests (type and press Enter)
{"jsonrpc":"2.0","id":1,"method":"provider.init","params":{}}
{"jsonrpc":"2.0","id":2,"method":"provider.match","params":{"input":"my:test"}}
```

### Verbose Mode

```bash
mehr --verbose start my:test
```

### Check Plugin Status

```bash
mehr plugins list
mehr plugins validate my-plugin
```
