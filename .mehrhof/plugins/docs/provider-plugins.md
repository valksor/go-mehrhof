# Provider Plugin Development

Provider plugins add custom task sources to Mehrhof. They allow loading tasks from external systems like Jira, YouTrack, Linear, or any custom API.

## Manifest Structure

```yaml
version: "1"
name: "jira"
type: "provider"
description: "Jira integration for task tracking"
protocol: "1"

executable:
  command: ["python3", "./jira-provider.py"]

provider:
  name: "jira" # Provider identifier (required)
  schemes: ["jira", "JIRA"] # URL schemes to match (required)
  priority: 50 # Higher = checked first (default: 50)
  capabilities: # What the provider can do
    - read
    - snapshot
    - list
    - comment
    - update_status

env:
  JIRA_URL:
    description: "Jira server URL"
    required: true
  JIRA_TOKEN:
    description: "API token"
    required: true
```

## Required Methods

### provider.init

Called once when the plugin starts.

**Request:**

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "provider.init",
  "params": { "config": {} }
}
```

**Response:**

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": { "capabilities": ["read", "snapshot"] }
}
```

### provider.match

Check if input matches this provider's scheme.

**Request:**

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "provider.match",
  "params": { "input": "jira:PROJ-123" }
}
```

**Response:**

```json
{ "jsonrpc": "2.0", "id": 2, "result": { "matches": true } }
```

### provider.parse

Parse input string to extract the task ID.

**Request:**

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "provider.parse",
  "params": { "input": "jira:PROJ-123" }
}
```

**Response:**

```json
{ "jsonrpc": "2.0", "id": 3, "result": { "id": "PROJ-123" } }
```

**Error Response (invalid input):**

```json
{ "jsonrpc": "2.0", "id": 3, "result": { "error": "Invalid Jira key format" } }
```

### provider.fetch

Fetch task details by ID. This is the main method that returns the WorkUnit.

**Request:**

```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "provider.fetch",
  "params": { "id": "PROJ-123" }
}
```

**Response (WorkUnit):**

```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "result": {
    "id": "abc12345",
    "externalId": "PROJ-123",
    "externalKey": "PROJ-123",
    "taskType": "story",
    "provider": "jira",
    "title": "Implement login page",
    "description": "Full task description...",
    "status": "open",
    "priority": 2,
    "labels": ["frontend", "auth"],
    "source": {
      "reference": "https://company.atlassian.net/browse/PROJ-123"
    }
  }
}
```

## WorkUnit Fields

| Field              | Type     | Required | Description                             |
| ------------------ | -------- | -------- | --------------------------------------- |
| `id`               | string   | Yes      | Internal unique ID (can be generated)   |
| `externalId`       | string   | No       | Original external reference             |
| `externalKey`      | string   | No       | User-facing key for branch naming       |
| `taskType`         | string   | No       | Task type (story, bug, issue, pr, task) |
| `provider`         | string   | Yes      | Provider name                           |
| `title`            | string   | Yes      | Task title                              |
| `description`      | string   | Yes      | Full task description/content           |
| `status`           | string   | No       | Status (open, in_progress, closed)      |
| `priority`         | int      | No       | Priority 1-5 (1=highest)                |
| `labels`           | []string | No       | Labels/tags                             |
| `subtasks`         | []string | No       | Subtask IDs                             |
| `slug`             | string   | No       | URL-safe title for branch naming        |
| `source.reference` | string   | No       | Source URL or reference                 |
| `metadata`         | object   | No       | Arbitrary additional data               |

## Optional Methods

### provider.snapshot

Return raw content for storage. Called after fetch to capture the original content.

**Request:**

```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "method": "provider.snapshot",
  "params": { "id": "PROJ-123" }
}
```

**Response:**

```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "result": { "content": "# PROJ-123\n\nFull markdown content..." }
}
```

### provider.list

List available tasks (requires `list` capability).

**Request:**

```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "method": "provider.list",
  "params": { "filter": { "status": "open" }, "limit": 10 }
}
```

**Response:**

```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "result": {
    "items": [
      { "id": "PROJ-123", "title": "Task 1", "status": "open" },
      { "id": "PROJ-124", "title": "Task 2", "status": "open" }
    ],
    "total": 42
  }
}
```

### provider.addComment

Add a comment to a task (requires `comment` capability).

**Request:**

```json
{
  "jsonrpc": "2.0",
  "id": 7,
  "method": "provider.addComment",
  "params": { "id": "PROJ-123", "comment": "Implementation started" }
}
```

### provider.updateStatus

Update task status (requires `update_status` capability).

**Request:**

```json
{
  "jsonrpc": "2.0",
  "id": 8,
  "method": "provider.updateStatus",
  "params": { "id": "PROJ-123", "status": "in_progress" }
}
```

## Capabilities

| Capability       | Methods                 | Description            |
| ---------------- | ----------------------- | ---------------------- |
| `read`           | fetch                   | Basic read-only access |
| `snapshot`       | snapshot                | Capture task content   |
| `list`           | list                    | List/search tasks      |
| `comment`        | addComment              | Add comments           |
| `update_status`  | updateStatus            | Change task status     |
| `create_pr`      | createPR                | Create pull requests   |
| `link_branch`    | linkBranch              | Link branch to task    |
| `manage_labels`  | addLabels, removeLabels | Manage labels          |
| `fetch_comments` | fetchComments           | Get task comments      |

## Complete Example: YouTrack Provider

```python
#!/usr/bin/env python3
"""YouTrack provider plugin for Mehrhof."""

import json
import os
import re
import sys
import urllib.request
import hashlib

YOUTRACK_URL = os.environ.get("YOUTRACK_URL", "")
YOUTRACK_TOKEN = os.environ.get("YOUTRACK_TOKEN", "")

def generate_id(external_id):
    """Generate deterministic internal ID."""
    return hashlib.sha256(external_id.encode()).hexdigest()[:8]

def youtrack_api(endpoint):
    """Make YouTrack API request."""
    url = f"{YOUTRACK_URL}/api/{endpoint}"
    req = urllib.request.Request(url)
    req.add_header("Authorization", f"Bearer {YOUTRACK_TOKEN}")
    req.add_header("Accept", "application/json")

    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read())

def handle_init(params):
    if not YOUTRACK_URL or not YOUTRACK_TOKEN:
        return {"error": {"code": -32000, "message": "Missing YOUTRACK_URL or YOUTRACK_TOKEN"}}
    return {"capabilities": ["read", "snapshot", "comment"]}

def handle_match(params):
    inp = params.get("input", "")
    # Match "yt:PROJ-123" or "youtrack:PROJ-123"
    return {"matches": bool(re.match(r"^(yt|youtrack):[A-Z]+-\d+$", inp, re.I))}

def handle_parse(params):
    inp = params.get("input", "")
    match = re.match(r"^(yt|youtrack):([A-Z]+-\d+)$", inp, re.I)
    if not match:
        return {"error": f"Invalid YouTrack issue format: {inp}"}
    return {"id": match.group(2).upper()}

def handle_fetch(params):
    issue_id = params.get("id", "")

    try:
        data = youtrack_api(f"issues/{issue_id}?fields=idReadable,summary,description,resolved")
    except Exception as e:
        return {"error": {"code": -32000, "message": str(e)}}

    return {
        "id": generate_id(issue_id),
        "externalId": issue_id,
        "externalKey": issue_id,
        "taskType": "issue",
        "provider": "youtrack",
        "title": data.get("summary", ""),
        "description": data.get("description", "") or "",
        "status": "closed" if data.get("resolved") else "open",
        "source": {"reference": f"{YOUTRACK_URL}/issue/{issue_id}"}
    }

def handle_snapshot(params):
    issue_id = params.get("id", "")
    data = youtrack_api(f"issues/{issue_id}?fields=summary,description")

    content = f"# {data.get('summary', '')}\n\n{data.get('description', '')}"
    return {"content": content}

def handle_request(request):
    method = request.get("method", "")
    params = request.get("params", {})

    handlers = {
        "provider.init": handle_init,
        "provider.match": handle_match,
        "provider.parse": handle_parse,
        "provider.fetch": handle_fetch,
        "provider.snapshot": handle_snapshot,
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
            if isinstance(result, dict) and "error" in result and "code" in result["error"]:
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

## Testing Your Provider

```bash
# 1. Start plugin manually
python3 ./my-provider.py

# 2. Send test requests
{"jsonrpc":"2.0","id":1,"method":"provider.init","params":{}}
{"jsonrpc":"2.0","id":2,"method":"provider.match","params":{"input":"my:TEST-1"}}
{"jsonrpc":"2.0","id":3,"method":"provider.fetch","params":{"id":"TEST-1"}}

# 3. Validate with mehr
mehr plugins validate my-provider

# 4. Test end-to-end
mehr start my:TEST-1
```

## Tips

1. **Generate stable IDs** - Use a hash of the external ID for deterministic internal IDs
2. **Handle rate limits** - Implement exponential backoff for API calls
3. **Cache responses** - Cache API responses to reduce latency
4. **Validate credentials early** - Check env vars in `init` and return clear errors
5. **Return externalKey** - This is used for branch naming; make it user-friendly
